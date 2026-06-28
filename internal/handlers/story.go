package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type StoryHandler struct {
	db *gorm.DB
}

func NewStoryHandler(db *gorm.DB) *StoryHandler {
	return &StoryHandler{db: db}
}

func (h *StoryHandler) GetByBondedBots(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var botIDs []string
	h.db.Model(&models.UserBotBond{}).Where("user_id = ?", userID).Pluck("bot_id", &botIDs)

	if len(botIDs) == 0 {
		return c.JSON(fiber.Map{"stories": []interface{}{}})
	}

	var stories []models.CharacterStory
	if err := h.db.Where("bot_id IN ?", botIDs).Order("created_at desc").Limit(50).Find(&stories).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch stories"})
	}

	return c.JSON(fiber.Map{"stories": stories})
}
