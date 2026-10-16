package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

type backupResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message,omitempty"`
	Data       string `json:"data,omitempty"`
	LastBackup string `json:"last_backup,omitempty"`
}

type storeBackupInput struct {
	Data string `json:"data" form:"data" binding:"required"`
}

const maxBackupBytes = 50 * 1024 * 1024 // 50 MB

func BackupStore(c *gin.Context) {
	var input storeBackupInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The data field is required.",
			"errors": gin.H{
				"data": []string{"The data field is required."},
			},
		})
		return
	}

	if strings.TrimSpace(input.Data) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The data field is required.",
			"errors": gin.H{
				"data": []string{"The data field is required."},
			},
		})
		return
	}
	if len(input.Data) > maxBackupBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"message": "Backup too large (max 50 MB).",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	info, err := services.PutBackupObject(user.ID, []byte(input.Data))
	if err != nil {
		c.JSON(http.StatusInternalServerError, backupResponse{
			Success: false,
			Message: "Failed to save backup: " + err.Error(),
		})
		return
	}

	key := services.BackupObjectKey(user.ID)
	size := info.Size
	etag := info.ETag
	backup := models.UserBackup{
		UserID:    user.ID,
		ObjectKey: &key,
		SizeBytes: &size,
		ETag:      &etag,
	}
	// Upsert metadata (Data tidak lagi dipakai)
	if err := database.DB.Where("user_id = ?", user.ID).Assign(backup).FirstOrCreate(&backup).Error; err != nil {
		log.Println("[WARN] backup metadata upsert failed: " + err.Error())
	}
	// Ambil timestamp terbaru dari DB atau dari R2 info
	if backup.UpdatedAt.IsZero() {
		backup.UpdatedAt = time.Now()
	}

	c.JSON(http.StatusOK, backupResponse{
		Success:    true,
		Message:    "Backup saved successfully.",
		LastBackup: backup.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

func BackupRestore(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	// Pastikan metadata ada (opsional) lalu ambil dari R2
	var backup models.UserBackup
	_ = database.DB.Where("user_id = ?", user.ID).First(&backup).Error

	data, err := services.GetBackupObject(user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, backupResponse{
			Success: false,
			Message: "No backup found for this user.",
		})
		return
	}

	lastBackup := time.Now().Format("2006-01-02 15:04:05")
	if !backup.UpdatedAt.IsZero() {
		lastBackup = backup.UpdatedAt.Format("2006-01-02 15:04:05")
	} else if info, err := services.StatBackupObject(user.ID); err == nil {
		lastBackup = info.LastModified.Format("2006-01-02 15:04:05")
	}

	c.JSON(http.StatusOK, backupResponse{
		Success:    true,
		Data:       string(data),
		LastBackup: lastBackup,
	})
}
