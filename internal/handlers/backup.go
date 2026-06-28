package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BackupHandler struct {
	db *gorm.DB
}

func NewBackupHandler(db *gorm.DB) *BackupHandler {
	return &BackupHandler{db: db}
}

func (h *BackupHandler) Store(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req struct {
		Data string `json:"data"`
	}
	if err := c.BodyParser(&req); err != nil || req.Data == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var existing models.UserBackup
	result := h.db.Where("user_id = ?", userID).First(&existing)
	if result.RowsAffected > 0 {
		h.db.Model(&existing).Update("data", req.Data)
	} else {
		h.db.Create(&models.UserBackup{UserID: userID, Data: req.Data})
	}

	return c.JSON(fiber.Map{"message": "Backup saved"})
}

func (h *BackupHandler) Restore(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var backup models.UserBackup
	if err := h.db.Where("user_id = ?", userID).First(&backup).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "No backup found"})
	}

	return c.JSON(fiber.Map{"data": backup.Data})
}
