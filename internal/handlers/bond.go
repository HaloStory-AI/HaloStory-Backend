package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BondHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewBondHandler(db *gorm.DB, cfg *config.Config) *BondHandler {
	return &BondHandler{db: db, cfg: cfg}
}

func (h *BondHandler) Recruit(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req struct {
		BotID string `json:"bot_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.BotID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var existing models.UserBotBond
	result := h.db.Where("user_id = ? AND bot_id = ?", userID, req.BotID).First(&existing)
	if result.RowsAffected > 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Already recruited"})
	}

	status := models.BondNormal
	bond := models.UserBotBond{
		UserID:     userID,
		BotID:      req.BotID,
		Xp:         0,
		BondStatus: &status,
	}

	if err := h.db.Create(&bond).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to recruit"})
	}

	return c.Status(201).JSON(fiber.Map{"bond": bond})
}

func (h *BondHandler) Show(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	botID := c.Params("bot_id")

	var bond models.UserBotBond
	if err := h.db.Where("user_id = ? AND bot_id = ?", userID, botID).Preload("Bot").First(&bond).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Bond not found"})
	}

	return c.JSON(fiber.Map{
		"bond":              bond,
		"level":             bond.Level(),
		"required_xp":       bond.RequiredXPForLevel(bond.Level() + 1),
		"bond_status":       bond.GetStatus(),
		"relationship":      bond.RelationshipLabel(),
		"emotion":           bond.BondEmotion(),
		"has_pending_choice": bond.HasPendingChoice(),
	})
}

func (h *BondHandler) HandleChoice(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	botID := c.Params("bot_id")

	var req struct {
		Choice string `json:"choice"`
	}
	if err := c.BodyParser(&req); err != nil || req.Choice == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var bond models.UserBotBond
	if err := h.db.Where("user_id = ? AND bot_id = ?", userID, botID).First(&bond).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Bond not found"})
	}

	if !bond.HasPendingChoice() {
		return c.Status(400).JSON(fiber.Map{"error": "No pending choice"})
	}

	bond.ApplyChoice(req.Choice)
	h.db.Save(&bond)

	return c.JSON(fiber.Map{"bond": bond, "status": bond.GetStatus()})
}

func (h *BondHandler) DebugUpdate(c *fiber.Ctx) error {
	if !h.cfg.AppDebug {
		return c.Status(403).JSON(fiber.Map{"error": "Forbidden"})
	}

	userID := middleware.GetUserID(c)
	botID := c.Params("bot_id")

	var req struct {
		Xp      *int64  `json:"xp"`
		Status  *string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var bond models.UserBotBond
	if err := h.db.Where("user_id = ? AND bot_id = ?", userID, botID).First(&bond).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Bond not found"})
	}

	updates := map[string]interface{}{}
	if req.Xp != nil {
		updates["xp"] = *req.Xp
	}
	if req.Status != nil {
		updates["bond_status"] = *req.Status
	}
	h.db.Model(&bond).Updates(updates)

	return c.JSON(fiber.Map{"bond": bond})
}
