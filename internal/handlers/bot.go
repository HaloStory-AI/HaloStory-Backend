package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BotHandler struct {
	db *gorm.DB
}

func NewBotHandler(db *gorm.DB) *BotHandler {
	return &BotHandler{db: db}
}

func (h *BotHandler) Index(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var bonds []models.UserBotBond
	botFields := []string{"bot_id", "name", "category_slug", "description", "avatar", "birthday", "sbr", "color", "child", "created_at", "updated_at"}
	if err := h.db.Where("user_id = ?", userID).
		Preload("Bot", func(db *gorm.DB) *gorm.DB { return db.Select(botFields) }).
		Preload("Bot.Category").
		Find(&bonds).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch bots"})
	}

	type botResponse struct {
		Bot               models.BotListResponse `json:"bot"`
		Level             int                    `json:"level"`
		Xp                int64                  `json:"xp"`
		RequiredXP        int                    `json:"required_xp"`
		BondStatus        string                 `json:"bond_status"`
		RelationshipLabel string                 `json:"relationship_label"`
		Emotion           string                 `json:"emotion"`
		HasPendingChoice  bool                   `json:"has_pending_choice"`
	}

	result := make([]botResponse, 0, len(bonds))
	for _, b := range bonds {
		level := b.Level()
		status := b.GetStatus()
		result = append(result, botResponse{
			Bot:               models.BotToListResponse(b.Bot),
			Level:             level,
			Xp:                b.Xp,
			RequiredXP:        b.RequiredXPForLevel(level + 1),
			BondStatus:        status,
			RelationshipLabel: b.RelationshipLabel(),
			Emotion:           b.BondEmotion(),
			HasPendingChoice:  b.HasPendingChoice(),
		})
	}

	return c.JSON(fiber.Map{"bots": result})
}
