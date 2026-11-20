package handlers

import (
	"net/http"
	"strings"

	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/parser"
	"subdeck/internal/services"

	"github.com/gin-gonic/gin"
)

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
	if strings.TrimSpace(s.URL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "远程订阅链接不能为空"})
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
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

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
		// Remote
		existing.URL = s.URL
		existing.UserAgent = s.UserAgent
		existing.InsecureSkipVerify = s.InsecureSkipVerify
		existing.AutoUpdate = s.AutoUpdate
		existing.IntervalHours = s.IntervalHours
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
