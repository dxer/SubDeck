package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"subdeck/internal/database"

	"github.com/gin-gonic/gin"
)

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
	fileHeader, err := c.FormFile("backup_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到上传的备份文件"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取上传文件: " + err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件内容失败: " + err.Error()})
		return
	}

	if err := h.db.RestoreFromBytes(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "恢复失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "数据库恢复成功，请重新登录！",
	})
}
