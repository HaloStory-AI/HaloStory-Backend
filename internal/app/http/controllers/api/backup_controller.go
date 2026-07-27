package api

import (
	"net/http"
	"strings"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"

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

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var existing models.UserBackup
	result := database.DB.Where("user_id = ?", user.ID).First(&existing)

	if result.Error == nil {
		existing.Data = models.JSON(input.Data)
		if err := database.DB.Save(&existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, backupResponse{
				Success: false,
				Message: "Failed to save backup: " + err.Error(),
			})
			return
		}
	} else {
		backup := models.UserBackup{
			UserID: user.ID,
			Data:   models.JSON(input.Data),
		}
		if err := database.DB.Create(&backup).Error; err != nil {
			c.JSON(http.StatusInternalServerError, backupResponse{
				Success: false,
				Message: "Failed to create backup: " + err.Error(),
			})
			return
		}
		existing = backup
	}

	c.JSON(http.StatusOK, backupResponse{
		Success:    true,
		Message:    "Backup saved successfully.",
		LastBackup: existing.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

func BackupRestore(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var backup models.UserBackup
	result := database.DB.Where("user_id = ?", user.ID).First(&backup)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, backupResponse{
			Success: false,
			Message: "No backup found for this user.",
		})
		return
	}

	c.JSON(http.StatusOK, backupResponse{
		Success:    true,
		Data:       string(backup.Data),
		LastBackup: backup.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}
