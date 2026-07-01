package api

import (
	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type RecruitRequest struct {
	BotID string `json:"bot_id" form:"bot_id"`
}

func BondRecruit(c *fiber.Ctx) error {
	var req RecruitRequest
	if err := c.BodyParser(&req); err != nil || req.BotID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bot_id is required"})
	}

	user := c.Locals("user").(models.User)

	var bond models.UserBotBond
	err := database.DB.Where("user_id = ? AND bot_id = ?", user.ID, req.BotID).First(&bond).Error
	if err != nil {
		// Create if not exists
		bond = models.UserBotBond{
			UserID:     user.ID,
			BotID:      req.BotID,
			Xp:         0,
			BondStatus: "normal",
		}
		database.DB.Create(&bond)
	}

	// Preload bot for RelationshipLabel
	database.DB.Preload("Bot").First(&bond, bond.ID)

	return c.JSON(fiber.Map{
		"success":            true,
		"bot_id":             req.BotID,
		"xp":                 bond.Xp,
		"bond_hearts":        bond.GetLevel(),
		"bond_status":        bond.BondStatus,
		"relationship_label": bond.GetRelationshipLabel(bond.Bot),
	})
}

func BondShow(c *fiber.Ctx) error {
	botID := c.Params("bot_id")
	user := c.Locals("user").(models.User)

	var bond models.UserBotBond
	err := database.DB.Where("user_id = ? AND bot_id = ?", user.ID, botID).First(&bond).Error
	if err != nil {
		bond = models.UserBotBond{
			UserID:     user.ID,
			BotID:      botID,
			Xp:         0,
			BondStatus: "normal",
		}
		database.DB.Create(&bond)
	}
	database.DB.Preload("Bot").First(&bond, bond.ID)

	level := bond.GetLevel()
	xpForCurrentLevel := bond.GetRequiredXpForLevel(level)
	xpForNextLevel := bond.GetRequiredXpForLevel(level + 1)
	xpIntoLevel := bond.Xp - xpForCurrentLevel
	if xpIntoLevel < 0 {
		xpIntoLevel = 0
	}
	xpPerLevel := xpForNextLevel - xpForCurrentLevel
	if xpPerLevel < 1 {
		xpPerLevel = 1
	}

	progressPercent := int((float64(xpIntoLevel) / float64(xpPerLevel)) * 100)
	if progressPercent > 100 {
		progressPercent = 100
	}

	return c.JSON(fiber.Map{
		"success":            true,
		"bot_id":             botID,
		"xp":                 bond.Xp,
		"bond_hearts":        level,
		"bond_status":        bond.BondStatus,
		"relationship_label": bond.GetRelationshipLabel(bond.Bot),
		"pending_choice":     bond.HasPendingChoice(),
		"xp_into_level":      xpIntoLevel,
		"xp_per_level":       xpPerLevel,
		"progress_percent":   progressPercent,
	})
}

type ChoiceRequest struct {
	Choice string `json:"choice" form:"choice"`
}

func BondHandleChoice(c *fiber.Ctx) error {
	botID := c.Params("bot_id")

	var req ChoiceRequest
	if err := c.BodyParser(&req); err != nil || req.Choice == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "choice is required"})
	}

	var bot models.Bot
	database.DB.Where("bot_id = ?", botID).First(&bot)

	if bot.Child && req.Choice != "beloved" && req.Choice != "reject_love" {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Invalid choice for child character.",
		})
	}

	user := c.Locals("user").(models.User)
	var bond models.UserBotBond
	if err := database.DB.Where("user_id = ? AND bot_id = ?", user.ID, botID).First(&bond).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Bond not found"})
	}

	bond.ApplyChoice(req.Choice)
	database.DB.Save(&bond)

	database.DB.Preload("Bot").First(&bond, bond.ID)

	return c.JSON(fiber.Map{
		"success":            true,
		"bond_status":        bond.BondStatus,
		"relationship_label": bond.GetRelationshipLabel(bond.Bot),
	})
}

type DebugRequest struct {
	Xp     *int    `json:"xp" form:"xp"`
	Level  *int    `json:"level" form:"level"`
	Status *string `json:"status" form:"status"`
	Reset  *bool   `json:"reset" form:"reset"`
}

func BondDebugUpdate(c *fiber.Ctx) error {
	appDebug := config.Env("APP_DEBUG", "false")
	if appDebug != "true" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden"})
	}

	botID := c.Params("bot_id")
	user := c.Locals("user").(models.User)

	var bond models.UserBotBond
	err := database.DB.Where("user_id = ? AND bot_id = ?", user.ID, botID).First(&bond).Error
	if err != nil {
		bond = models.UserBotBond{
			UserID:     user.ID,
			BotID:      botID,
			Xp:         0,
			BondStatus: "normal",
		}
		database.DB.Create(&bond)
	}

	var req DebugRequest
	c.BodyParser(&req)

	// Fallback to query params if not JSON body
	if req.Xp == nil && c.Query("xp") != "" {
		xp, _ := strconv.Atoi(c.Query("xp"))
		req.Xp = &xp
	}
	if req.Level == nil && c.Query("level") != "" {
		lvl, _ := strconv.Atoi(c.Query("level"))
		req.Level = &lvl
	}
	if req.Status == nil && c.Query("status") != "" {
		st := c.Query("status")
		req.Status = &st
	}
	if req.Reset == nil && c.Query("reset") == "true" {
		b := true
		req.Reset = &b
	}

	if req.Xp != nil {
		bond.Xp += *req.Xp
	}

	if req.Level != nil {
		bond.Xp = bond.GetRequiredXpForLevel(*req.Level)
	}

	if req.Status != nil {
		bond.BondStatus = *req.Status
	}

	if req.Reset != nil && *req.Reset {
		bond.Xp = 0
		bond.BondStatus = "normal"
		bond.ChoiceAtLevel = nil
	}

	database.DB.Save(&bond)
	database.DB.Preload("Bot").First(&bond, bond.ID)

	return c.JSON(fiber.Map{
		"success":            true,
		"xp":                 bond.Xp,
		"bond_hearts":        bond.GetLevel(),
		"bond_status":        bond.BondStatus,
		"relationship_label": bond.GetRelationshipLabel(bond.Bot),
		"pending_choice":     bond.HasPendingChoice(),
	})
}
