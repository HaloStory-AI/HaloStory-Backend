package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func ChatUpload(c *gin.Context) {
	if !services.S3Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Storage not configured, use base64 fallback", "fallback": "base64"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "file is required"})
		return
	}
	if file.Size > services.ChatImageMaxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": fmt.Sprintf("file too large (max %d bytes)", services.ChatImageMaxBytes)})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif"}
	mime, ok := allowed[ext]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "unsupported image type (jpg/png/webp/gif only)"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to read file"})
		return
	}
	defer f.Close()
	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to read file"})
		return
	}
	// Public: no user binding, use random prefix
	key := fmt.Sprintf("chat-uploads/public/%s%s", uuid.New().String(), ext)
	if err := services.PutChatUploadObject(key, data, mime); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to upload: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"url":  services.ImageURL(key),
		"mime": mime,
		"key":  key,
		"size": file.Size,
	})
}
