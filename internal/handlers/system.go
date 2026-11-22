package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"subdeck/internal/auth"
	"subdeck/internal/database"
	"subdeck/internal/models"
	"subdeck/internal/services"

	"github.com/gin-gonic/gin"
)

const maxBackupUploadBytes = 50 << 20 // 50 MiB

type SystemHandler struct {
	db *database.DB
}

func NewSystemHandler(db *database.DB) *SystemHandler {
	return &SystemHandler{db: db}
}

// DownloadBackup serves the current SQLite database as a downloadable file
func (h *SystemHandler) DownloadBackup(c *gin.Context) {
	data, err := h.db.BackupBytes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成备份失败: " + err.Error()})
		return
	}

	filename := fmt.Sprintf("subdeck-backup-%s.db", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// RestoreBackup handles uploading and applying a database backup file
func (h *SystemHandler) RestoreBackup(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBackupUploadBytes+1024*1024)

	fileHeader, err := c.FormFile("backup_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传的备份文件或文件超出 50MB 限制"})
		return
	}
	if fileHeader.Size > maxBackupUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "备份文件大小不能超过 50MB"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取上传文件: " + err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxBackupUploadBytes+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件内容失败: " + err.Error()})
		return
	}
	if len(data) > maxBackupUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "备份文件大小不能超过 50MB"})
		return
	}

	if err := h.db.RestoreFromBytes(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "恢复失败: " + err.Error()})
		return
	}

	auth.SetTokenVersion(h.db.GetTokenVersion())

	c.JSON(http.StatusOK, gin.H{
		"message": "数据库恢复成功，请重新登录！",
	})
}

func validateOptionalHTTPURL(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return true
	}
	u, err := url.ParseRequestURI(s)
	if err != nil || u.Host == "" {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

// GetSettings returns persisted system settings.
func (h *SystemHandler) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.db.GetSystemSettings())
}

// UpdateSettings validates and saves runtime system settings.
func (h *SystemHandler) UpdateSettings(c *gin.Context) {
	var req models.SystemSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	if !validateOptionalHTTPURL(req.WebhookURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Webhook 地址必须为合法的 http:// 或 https:// 链接"})
		return
	}
	if !validateOptionalHTTPURL(req.RulesetMirror) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "规则集镜像前缀必须为合法的 http:// 或 https:// 链接"})
		return
	}
	if req.QuotaWarnDays < 1 || req.QuotaWarnDays > 90 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "套餐到期预警天数需在 1-90 之间"})
		return
	}
	if req.QuotaWarnPercent < 50 || req.QuotaWarnPercent > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "流量预警百分比需在 50-100 之间"})
		return
	}

	if err := h.db.SaveSystemSettings(&req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存系统配置失败: " + err.Error()})
		return
	}
	services.SetWebhookAllowPrivateNetwork(req.AllowPrivateFetch)
	c.JSON(http.StatusOK, gin.H{
		"message":  "系统配置已保存",
		"settings": h.db.GetSystemSettings(),
	})
}

type webhookTestReq struct {
	WebhookURL string `json:"webhook_url"`
}

// TestWebhook sends a test alert notification to verify webhook reachability.
func (h *SystemHandler) TestWebhook(c *gin.Context) {
	var req webhookTestReq
	_ = c.ShouldBindJSON(&req)

	target := strings.TrimSpace(req.WebhookURL)
	if target == "" {
		target = h.db.GetSystemSettings().WebhookURL
	}
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先填写 Webhook 回调地址"})
		return
	}

	if err := services.SendWebhookNotification(
		target,
		"webhook_test",
		"SubDeck 告警通知测试",
		"这是一条来自 SubDeck 控制面的测试消息，说明 Webhook 通道工作正常。",
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "测试发送失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "测试通知发送成功"})
}
