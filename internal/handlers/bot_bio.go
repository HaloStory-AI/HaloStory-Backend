package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BotBioHandler struct {
	db *gorm.DB
}

func NewBotBioHandler(db *gorm.DB) *BotBioHandler {
	return &BotBioHandler{db: db}
}

func (h *BotBioHandler) Show(c *fiber.Ctx) error {
	botID := c.Params("bot_id")

	var bot models.Bot
	if err := h.db.Where("bot_id = ?", botID).Preload("Category").First(&bot).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Bot not found"})
	}

	return c.JSON(fiber.Map{"bot": bot})
}
