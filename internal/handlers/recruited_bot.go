package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type RecruitedBotHandler struct {
	db *gorm.DB
}

func NewRecruitedBotHandler(db *gorm.DB) *RecruitedBotHandler {
	return &RecruitedBotHandler{db: db}
}

func (h *RecruitedBotHandler) Index(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var bonds []models.UserBotBond
	if err := h.db.Where("user_id = ?", userID).
		Preload("Bot").
		Preload("Bot.Category").
		Order("created_at desc").
		Find(&bonds).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch recruited bots"})
	}

	type recruitedBot struct {
		Bot        models.BotListResponse `json:"bot"`
		Level      int                    `json:"level"`
		Xp         int64                  `json:"xp"`
		BondStatus string                 `json:"bond_status"`
		CreatedAt  string                 `json:"created_at"`
	}

	result := make([]recruitedBot, 0, len(bonds))
	for _, b := range bonds {
		result = append(result, recruitedBot{
			Bot:        models.BotToListResponse(b.Bot),
			Level:      b.Level(),
			Xp:         b.Xp,
			BondStatus: b.GetStatus(),
			CreatedAt:  b.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return c.JSON(fiber.Map{"recruited_bots": result})
}
