package handlers

import (
	"net/http"
	"strings"

	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/services"

	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	db *database.DB
}

func NewProfileHandler(db *database.DB) *ProfileHandler {
	return &ProfileHandler{db: db}
}

func (h *ProfileHandler) List(c *gin.Context) {
	profiles, err := h.db.ListProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取订阅组合列表失败"})
		return
	}

	for _, p := range profiles {
		nodes, _ := services.AggregateProfileNodes(h.db, p)
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

	if err := h.db.CreateProfile(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建订阅组合失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *ProfileHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var p models.Profile
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if strings.TrimSpace(p.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "组合名称不能为空"})
		return
	}

	existing, err := h.db.GetProfileByID(id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订阅组合不存在"})
		return
	}

	p.ID = id
	if err := h.db.UpdateProfile(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新组合失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
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
