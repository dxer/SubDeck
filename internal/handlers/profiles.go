package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"subdeck/internal/database"
	"subdeck/internal/generator"
	"subdeck/internal/models"
	"subdeck/internal/pipeline"
	"subdeck/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type ProfileHandler struct {
	db      *database.DB
	fetcher *services.FetcherService
}

func NewProfileHandler(db *database.DB, fetcher *services.FetcherService) *ProfileHandler {
	return &ProfileHandler{db: db, fetcher: fetcher}
}

// resolveTemplateContent picks the effective template text: inline content wins,
// otherwise the cached copy of the remote template is used.
func resolveTemplateContent(content, url, cache string) string {
	if strings.TrimSpace(content) != "" {
		return content
	}
	if strings.TrimSpace(url) != "" {
		return cache
	}
	return ""
}

// syncProfileTemplates validates inline templates, fetches remote templates and
// refreshes the persisted cache + status. Fetch failures keep the previous cache
// and are reported through TemplateStatus without failing the whole save.
func (h *ProfileHandler) syncProfileTemplates(p *models.Profile, allowPrivate bool) {
	var problems []string

	// Clash template: inline content wins; otherwise fetch the remote template
	// and cache the last valid copy (public exports never fetch).
	if strings.TrimSpace(p.ClashTemplate) != "" {
		if _, err := generator.ParseClashTemplate(p.ClashTemplate); err != nil {
			problems = append(problems, "clash: "+err.Error())
		}
	} else if strings.TrimSpace(p.ClashTemplateURL) != "" {
		fetched := false
		if h.fetcher != nil {
			if body, err := h.fetcher.FetchTemplate(p.ClashTemplateURL, allowPrivate); err == nil {
				if text := strings.TrimSpace(string(body)); text != "" {
					if _, parseErr := generator.ParseClashTemplate(text); parseErr == nil {
						p.ClashTemplateCache = text
						fetched = true
					} else {
						problems = append(problems, "clash: 远程模板解析失败: "+parseErr.Error())
					}
				}
			}
		}
		if !fetched && strings.TrimSpace(p.ClashTemplateCache) == "" {
			problems = append(problems, "clash: 模板缓存为空，请检查模板地址或刷新模板")
		}
		if !fetched && strings.TrimSpace(p.ClashTemplateCache) != "" {
			problems = append(problems, "clash: 远程模板拉取失败，沿用上次有效缓存")
		}
	}

	if strings.TrimSpace(p.SingboxTemplate) != "" {
		if _, err := generator.ParseSingboxTemplate(p.SingboxTemplate); err != nil {
			problems = append(problems, "singbox: "+err.Error())
		}
	} else if strings.TrimSpace(p.SingboxTemplateURL) != "" {
		fetched := false
		if h.fetcher != nil {
			if body, err := h.fetcher.FetchTemplate(p.SingboxTemplateURL, allowPrivate); err == nil {
				if text := strings.TrimSpace(string(body)); text != "" {
					if _, parseErr := generator.ParseSingboxTemplate(text); parseErr == nil {
						p.SingboxTemplateCache = text
						fetched = true
					} else {
						problems = append(problems, "singbox: 远程模板解析失败: "+parseErr.Error())
					}
				}
			}
		}
		if !fetched && strings.TrimSpace(p.SingboxTemplateCache) == "" {
			problems = append(problems, "singbox: 模板缓存为空，请检查模板地址或刷新模板")
		}
		if !fetched && strings.TrimSpace(p.SingboxTemplateCache) != "" {
			problems = append(problems, "singbox: 远程模板拉取失败，沿用上次有效缓存")
		}
	}

	p.TemplateStatus = strings.Join(problems, "; ")
}

func (h *ProfileHandler) List(c *gin.Context) {
	profiles, err := h.db.ListProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取订阅组合列表失败"})
		return
	}

	allSubs, err := h.db.ListSubscriptions("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取订阅缓存失败"})
		return
	}
	subMap := make(map[string]*models.Subscription, len(allSubs))
	for _, s := range allSubs {
		subMap[s.ID] = s
	}

	for _, p := range profiles {
		nodes, _, _ := services.AggregateProfileNodesFromSubs(allSubs, subMap, p)
		p.NodeCount = len(nodes)
	}

	c.JSON(http.StatusOK, profiles)
}

func (h *ProfileHandler) Create(c *gin.Context) {
	var p models.Profile
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if strings.TrimSpace(p.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "组合名称不能为空"})
		return
	}
	if err := pipeline.ValidateProfileRegexes(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.SubscriptionSelectionMode != "" && p.SubscriptionSelectionMode != "all" && p.SubscriptionSelectionMode != "explicit" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subscription_selection_mode 仅支持 all 或 explicit"})
		return
	}
	allowPrivate := false
	if h.fetcher != nil {
		allowPrivate = h.fetcher.AllowPrivateNetwork || h.db.GetSystemSettings().AllowPrivateFetch
	}
	h.syncProfileTemplates(&p, allowPrivate)

	if err := h.db.CreateProfile(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建订阅组合失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *ProfileHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var p models.Profile
	if err := c.ShouldBindBodyWith(&p, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var raw map[string]json.RawMessage
	_ = c.ShouldBindBodyWith(&raw, binding.JSON)
	has := func(key string) bool { _, ok := raw[key]; return ok }

	if strings.TrimSpace(p.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "组合名称不能为空"})
		return
	}
	if err := pipeline.ValidateProfileRegexes(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.SubscriptionSelectionMode != "" && p.SubscriptionSelectionMode != "all" && p.SubscriptionSelectionMode != "explicit" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subscription_selection_mode 仅支持 all 或 explicit"})
		return
	}

	existing, err := h.db.GetProfileByID(id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅组合不存在"})
		return
	}

	// Template fields are presence-aware: omitted keys keep the stored values so
	// older API clients cannot wipe templates by sending partial bodies.
	if has("clash_template") {
		existing.ClashTemplate = p.ClashTemplate
	}
	if has("clash_template_url") {
		existing.ClashTemplateURL = strings.TrimSpace(p.ClashTemplateURL)
	}
	if has("singbox_template") {
		existing.SingboxTemplate = p.SingboxTemplate
	}
	if has("singbox_template_url") {
		existing.SingboxTemplateURL = strings.TrimSpace(p.SingboxTemplateURL)
	}

	p.ID = id
	// Carry the (possibly preserved) template fields into the update payload.
	p.ClashTemplate = existing.ClashTemplate
	p.SingboxTemplate = existing.SingboxTemplate
	p.ClashTemplateURL = existing.ClashTemplateURL
	p.SingboxTemplateURL = existing.SingboxTemplateURL
	p.ClashTemplateCache = existing.ClashTemplateCache
	p.SingboxTemplateCache = existing.SingboxTemplateCache

	allowPrivate := false
	if h.fetcher != nil {
		allowPrivate = h.fetcher.AllowPrivateNetwork || h.db.GetSystemSettings().AllowPrivateFetch
	}
	h.syncProfileTemplates(&p, allowPrivate)

	if err := h.db.UpdateProfile(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新组合失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// RefreshTemplates re-fetches remote templates for a profile, keeping the last
// valid cache on failure.
func (h *ProfileHandler) RefreshTemplates(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProfileByID(id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅组合不存在"})
		return
	}
	allowPrivate := false
	if h.fetcher != nil {
		allowPrivate = h.fetcher.AllowPrivateNetwork || h.db.GetSystemSettings().AllowPrivateFetch
	}
	h.syncProfileTemplates(existing, allowPrivate)
	if err := h.db.UpdateProfile(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "刷新模板失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "模板已刷新", "template_status": existing.TemplateStatus})
}

func (h *ProfileHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.DeleteProfile(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除组合失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *ProfileHandler) ResetToken(c *gin.Context) {
	id := c.Param("id")
	newToken, err := h.db.ResetProfileToken(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置令牌失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Token 重置成功",
		"token":   newToken,
	})
}

func (h *ProfileHandler) Preview(c *gin.Context) {
	id := c.Param("id")
	target, err := h.db.GetProfileByID(id)
	if err != nil || target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅组合不存在"})
		return
	}

	nodes, _ := services.AggregateProfileNodes(h.db, target)
	c.JSON(http.StatusOK, nodes)
}

// PreviewDraft computes matching nodes for an unsaved profile draft in real time.
func (h *ProfileHandler) PreviewDraft(c *gin.Context) {
	var draft models.Profile
	if err := c.ShouldBindJSON(&draft); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := pipeline.ValidateProfileRegexes(&draft); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nodes, _ := services.AggregateProfileNodes(h.db, &draft)
	c.JSON(http.StatusOK, gin.H{
		"count": len(nodes),
		"nodes": nodes,
	})
}

// Probe runs lightweight TCP connectivity probes against all source subscriptions of this profile.
func (h *ProfileHandler) Probe(c *gin.Context) {
	id := c.Param("id")
	target, err := h.db.GetProfileByID(id)
	if err != nil || target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅组合不存在"})
		return
	}

	var subIDs []string
	if len(target.SubscriptionIDs) == 0 {
		allSubs, _ := h.db.ListSubscriptions("")
		for _, s := range allSubs {
			subIDs = append(subIDs, s.ID)
		}
	} else {
		seen := make(map[string]bool)
		for _, sid := range target.SubscriptionIDs {
			if !seen[sid] {
				seen[sid] = true
				subIDs = append(subIDs, sid)
			}
		}
	}

	for _, sid := range subIDs {
		_, _, _, _ = services.ProbeSubscription(h.db, sid)
	}

	nodes, _ := services.AggregateProfileNodes(h.db, target)
	healthy, failed := 0, 0
	for _, n := range nodes {
		if n.LatencyMs > 0 {
			healthy++
		} else if n.LatencyMs < 0 {
			failed++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"healthy": healthy,
		"failed":  failed,
		"nodes":   nodes,
	})
}
