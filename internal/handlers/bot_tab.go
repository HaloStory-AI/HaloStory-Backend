package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BotTabHandler struct {
	db *gorm.DB
}

func NewBotTabHandler(db *gorm.DB) *BotTabHandler {
	return &BotTabHandler{db: db}
}

func (h *BotTabHandler) Index(c *fiber.Ctx) error {
	var categories []models.Category
	if err := h.db.Where("pinned = ? OR featured = ?", true, true).
		Preload("Bots", func(db *gorm.DB) *gorm.DB {
			return db.Order("name asc")
		}).
		Order("pinned desc, name asc").
		Find(&categories).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch bot tab"})
	}

	return c.JSON(fiber.Map{"categories": categories})
}
