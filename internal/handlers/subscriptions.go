package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/parser"
	"subdeck/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const maxSubscriptionURLs = 10

func validateRemoteURL(raw string, allowPrivate bool) (bool, string) {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false, "远程订阅链接必须为合法的 http:// 或 https:// 地址"
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false, "远程订阅链接必须为合法的 http:// 或 https:// 地址"
	}
	if !allowPrivate {
		hostname := strings.TrimSpace(u.Hostname())
		if strings.EqualFold(hostname, "localhost") {
			return false, "出于 SSRF 安全限制，默认禁止订阅本机/内网地址（可在系统设置中放行）"
		}
		if ip := net.ParseIP(hostname); ip != nil && services.IsBlockedIP(ip) {
			return false, "出于 SSRF 安全限制，默认禁止订阅私有/保留 IP 地址（可在系统设置中放行）"
		}
	}
	return true, ""
}

// normalizeSubscriptionURLs trims, drops empties and removes exact duplicates
// while preserving order.
func normalizeSubscriptionURLs(raw []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, u := range raw {
		trimmed := strings.TrimSpace(u)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

// validateSubscriptionFetchSettings validates every remote URL plus the optional
// fetch proxy and failure policy.
func validateSubscriptionFetchSettings(s *models.Subscription, allowPrivate bool) (string, bool) {
	urls := normalizeSubscriptionURLs(s.URLs)
	if len(urls) == 0 && strings.TrimSpace(s.URL) != "" {
		urls = []string{strings.TrimSpace(s.URL)}
	}
	if len(urls) == 0 {
		return "远程订阅链接不能为空", false
	}
	if len(urls) > maxSubscriptionURLs {
		return "单个订阅源最多支持 10 条链接", false
	}
	for _, u := range urls {
		if ok, errMsg := validateRemoteURL(u, allowPrivate); !ok {
			return errMsg, false
		}
	}
	s.URLs = urls
	s.URL = urls[0]

	policy := strings.ToLower(strings.TrimSpace(s.FailurePolicy))
	if policy == "" {
		policy = models.FetchFailureStrict
	}
	if policy != models.FetchFailureStrict && policy != models.FetchFailureSkip {
		return "失败策略仅支持 strict（任一失败整体回退）或 skip（跳过失败链接）", false
	}
	s.FailurePolicy = policy

	mode := strings.ToLower(strings.TrimSpace(s.FetchMode))
	if mode == "" {
		mode = models.FetchModeMerge
	}
	if mode != models.FetchModeMerge && mode != models.FetchModeFailover {
		return "拉取模式仅支持 merge（合并全部链接）或 failover（镜像故障转移）", false
	}
	s.FetchMode = mode

	if strings.TrimSpace(s.FetchProxyURL) != "" {
		if _, err := services.ValidateProxyURL(s.FetchProxyURL, allowPrivate); err != nil {
			return "拉取代理配置无效: " + err.Error(), false
		}
	} else {
		s.FetchProxyURL = ""
	}
	return "", true
}

type SubscriptionHandler struct {
	db      *database.DB
	fetcher *services.FetcherService
}

func NewSubscriptionHandler(db *database.DB, fetcher *services.FetcherService) *SubscriptionHandler {
	return &SubscriptionHandler{
		db:      db,
		fetcher: fetcher,
	}
}

func (h *SubscriptionHandler) List(c *gin.Context) {
	subType := c.Query("type")
	subs, err := h.db.ListSubscriptions(subType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取订阅列表失败"})
		return
	}
	// Clear cached nodes in list view to save payload size
	for _, s := range subs {
		s.CachedNodes = nil
	}
	c.JSON(http.StatusOK, subs)
}

func (h *SubscriptionHandler) Create(c *gin.Context) {
	var s models.Subscription
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if strings.TrimSpace(s.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订阅名称不能为空"})
		return
	}

	if s.Type == models.SubscriptionTypeLocal {
		// Local Subscription
		s.RawContent = strings.TrimSpace(s.RawContent)
		if s.RawContent == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "本地订阅的节点内容不能为空"})
			return
		}

		nodes, err := parser.ParseSubscriptionContent([]byte(s.RawContent))
		if err != nil || len(nodes) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未能从输入中解析出任何有效代理节点链接"})
			return
		}

		for _, n := range nodes {
			n.Source = s.Name
		}
		s.CachedNodes = nodes
		s.NodeCount = len(nodes)
		s.Status = "ok"

		if err := h.db.CreateSubscription(&s); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建本地订阅失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, s)
		return
	}

	// Remote Subscription
	s.Type = models.SubscriptionTypeRemote
	if errMsg, ok := validateSubscriptionFetchSettings(&s, h.fetcher.AllowPrivateNetwork || h.db.GetSystemSettings().AllowPrivateFetch); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	if s.IntervalHours <= 0 {
		s.IntervalHours = 12
	}

	if err := h.db.CreateSubscription(&s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建远程订阅失败: " + err.Error()})
		return
	}

	// Trigger async initial fetch
	go func(id string) {
		_, _ = h.fetcher.FetchSubscription(id)
	}(s.ID)

	c.JSON(http.StatusOK, s)
}

func (h *SubscriptionHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var s models.Subscription
	// Bind twice from the cached body: struct for typed fields, map for key
	// presence so omitted legacy fields never erase newer settings.
	if err := c.ShouldBindBodyWith(&s, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var raw map[string]json.RawMessage
	_ = c.ShouldBindBodyWith(&raw, binding.JSON)

	has := func(key string) bool { _, ok := raw[key]; return ok }

	existing, err := h.db.GetSubscription(id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅不存在"})
		return
	}

	existing.Name = s.Name
	if existing.Type == models.SubscriptionTypeLocal {
		// Re-parse raw content if modified
		if s.RawContent != "" {
			existing.RawContent = s.RawContent
			nodes, err := parser.ParseSubscriptionContent([]byte(s.RawContent))
			if err == nil && len(nodes) > 0 {
				for _, n := range nodes {
					n.Source = existing.Name
				}
				existing.CachedNodes = nodes
				existing.NodeCount = len(nodes)
				existing.Status = "ok"
				existing.ErrorMsg = ""
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "未能解析出有效节点"})
				return
			}
		} else {
			for _, n := range existing.CachedNodes {
				n.Source = existing.Name
			}
		}
	} else {
		// Remote: merge fetch settings presence-aware
		allowPrivate := h.fetcher.AllowPrivateNetwork || h.db.GetSystemSettings().AllowPrivateFetch

		legacyURLChanged := has("url") && strings.TrimSpace(s.URL) != "" && strings.TrimSpace(s.URL) != strings.TrimSpace(existing.URL)
		if has("urls") {
			existing.URLs = s.URLs
		} else if legacyURLChanged {
			// Legacy single-URL client edited the first URL: keep extra URLs.
			if len(existing.URLs) > 1 {
				existing.URLs = append([]string{strings.TrimSpace(s.URL)}, existing.URLs[1:]...)
			} else {
				existing.URLs = []string{strings.TrimSpace(s.URL)}
			}
		}
		if has("fetch_proxy_url") {
			existing.FetchProxyURL = strings.TrimSpace(s.FetchProxyURL)
		}
		if has("failure_policy") {
			existing.FailurePolicy = s.FailurePolicy
		}
		if has("fetch_mode") {
			existing.FetchMode = s.FetchMode
		}

		if errMsg, ok := validateSubscriptionFetchSettings(existing, allowPrivate); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}

		if has("user_agent") {
			existing.UserAgent = s.UserAgent
		}
		if has("insecure_skip_verify") {
			existing.InsecureSkipVerify = s.InsecureSkipVerify
		}
		if has("auto_update") {
			existing.AutoUpdate = s.AutoUpdate
		}
		if has("interval_hours") {
			existing.IntervalHours = s.IntervalHours
		}
		for _, n := range existing.CachedNodes {
			n.Source = existing.Name
		}
	}

	if err := h.db.UpdateSubscription(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新订阅失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (h *SubscriptionHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.DeleteSubscription(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *SubscriptionHandler) Refresh(c *gin.Context) {
	id := c.Param("id")
	nodes, err := h.fetcher.FetchSubscription(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "拉取订阅失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":    "刷新成功",
		"node_count": len(nodes),
	})
}

func (h *SubscriptionHandler) GetNodes(c *gin.Context) {
	id := c.Param("id")
	sub, err := h.db.GetSubscription(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅不存在"})
		return
	}
	c.JSON(http.StatusOK, sub.CachedNodes)
}

// Probe tests TCP connectivity for all cached nodes of a subscription.
func (h *SubscriptionHandler) Probe(c *gin.Context) {
	id := c.Param("id")
	nodes, healthy, failed, err := services.ProbeSubscription(h.db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅不存在或探活失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"healthy": healthy,
		"failed":  failed,
		"nodes":   nodes,
	})
}
