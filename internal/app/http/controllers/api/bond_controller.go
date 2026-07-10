package api

import (
	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"strconv"

	"net/http"

	"github.com/gin-gonic/gin"
)

type RecruitRequest struct {
	BotID string `json:"bot_id" form:"bot_id"`
}

func BondRecruit(c *gin.Context) {
	var req RecruitRequest
	if err := c.ShouldBind(&req); err != nil || req.BotID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_id is required"})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

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

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"bot_id":             req.BotID,
		"xp":                 bond.Xp,
		"bond_hearts":        bond.GetLevel(),
		"bond_status":        bond.BondStatus,
		"relationship_label": bond.GetRelationshipLabel(bond.Bot),
	})
}

func BondShow(c *gin.Context) {
	botID := c.Param("bot_id")
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

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

	c.JSON(http.StatusOK, gin.H{
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

func BondHandleChoice(c *gin.Context) {
	botID := c.Param("bot_id")

	var req ChoiceRequest
	if err := c.ShouldBind(&req); err != nil || req.Choice == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "choice is required"})
		return
	}

	var bot models.Bot
	database.DB.Where("bot_id = ?", botID).First(&bot)

	if bot.Child && req.Choice != "beloved" && req.Choice != "reject_love" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"success": false,
			"message": "Invalid choice for child character.",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)
	var bond models.UserBotBond
	if err := database.DB.Where("user_id = ? AND bot_id = ?", user.ID, botID).First(&bond).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Bond not found"})
		return
	}

	bond.ApplyChoice(req.Choice)
	database.DB.Save(&bond)

	database.DB.Preload("Bot").First(&bond, bond.ID)

	c.JSON(http.StatusOK, gin.H{
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

func BondDebugUpdate(c *gin.Context) {
	appDebug := config.Env("APP_DEBUG", "false")
	if appDebug != "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	botID := c.Param("bot_id")
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

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
	c.ShouldBind(&req)

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

	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"xp":                 bond.Xp,
		"bond_hearts":        bond.GetLevel(),
		"bond_status":        bond.BondStatus,
		"relationship_label": bond.GetRelationshipLabel(bond.Bot),
		"pending_choice":     bond.HasPendingChoice(),
	})
}
