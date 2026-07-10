package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"net/http"

	"github.com/gin-gonic/gin"
)

type botIndexItem struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	Avatar       *string `json:"avatar"`
	ImageURL     string  `json:"image_url"`
	Background   *string `json:"background"`
	Personality  *string `json:"personality"`
	ChatExamples *string `json:"chat_examples"`
	Birthday     *string `json:"birthday"`
	BondXp       int     `json:"bond_xp"`
	BondHearts   int     `json:"bond_hearts"`
	Sbr          string  `json:"sbr"`
	Color        string  `json:"color"`
	Child        bool    `json:"child"`
}

// Index menampilkan seluruh bot
func BotIndex(c *gin.Context) {
	var bots []models.Bot
	database.DB.Select("bot_id, name, description, avatar, jailbreak, personality, chat_examples, birthday, sbr, color, child").Find(&bots)

	var bonds []models.UserBotBond
	// Ambil dari context jika ada
	userVal, exists := c.Get("user")
	if exists && userVal != nil {
		user := userVal.(models.User)
		database.DB.Where("user_id = ?", user.ID).Find(&bonds)
	}

	bondMap := make(map[string]models.UserBotBond)
	for _, b := range bonds {
		bondMap[b.BotID] = b
	}

	appUrl := config.Env("APP_URL", "http://127.0.0.1")

	var response []botIndexItem
	for _, bot := range bots {
		bondXp := 0
		bondHearts := 0

		if bond, exists := bondMap[bot.BotID]; exists {
			bondXp = bond.Xp
			bondHearts = bond.GetLevel()
		}

		sbr := ""
		if bot.Sbr != nil {
			sbr = *bot.Sbr
		}

		// Logika Happy Birthday (jika hari ini ultah)
		if bot.Birthday != nil {
			now := time.Now()
			if bot.Birthday.Month() == now.Month() && bot.Birthday.Day() == now.Day() {
				if sbr == "" {
					sbr = "happy_birthday"
				} else {
					sbr += " happy_birthday"
				}
			}
		}

		color := "0xFF607D8B"
		if bot.Color != nil {
			color = *bot.Color
		}

		avatar := ""
		if bot.Avatar != nil {
			avatar = *bot.Avatar
		}

		var birthdayStr *string
		if bot.Birthday != nil {
			str := bot.Birthday.Format("2006-01-02")
			birthdayStr = &str
		}

		response = append(response, botIndexItem{
			ID:           bot.BotID,
			Name:         bot.Name,
			Description:  bot.Description,
			Avatar:       bot.Avatar,
			ImageURL:     fmt.Sprintf("%s/api/image?path=%s", appUrl, avatar),
			Background:   bot.Jailbreak,
			Personality:  bot.Personality,
			ChatExamples: bot.ChatExamples,
			Birthday:     birthdayStr,
			BondXp:       bondXp,
			BondHearts:   bondHearts,
			Sbr:          sbr,
			Color:        color,
			Child:        bot.Child,
		})
	}

	// Tangani kasus slice nil menjadi empty array []
	if response == nil {
		response = make([]botIndexItem, 0)
	}

	c.JSON(http.StatusOK, response)
}

// ShowBio mengekstrak profil dari personality string via RegEx
func ShowBio(c *gin.Context) {
	botID := c.Param("bot_id")
	var bot models.Bot

	if err := database.DB.Where("bot_id = ?", botID).First(&bot).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"description": "",
			"personality": nil,
		})
		return
	}

	desc := ""
	if bot.Description != nil {
		desc = *bot.Description
	}

	var pers *string
	if bot.Personality != nil {
		pers = bot.Personality
	}

	parsedPers := parsePersonality(pers)

	c.JSON(http.StatusOK, gin.H{
		"description": desc,
		"personality": parsedPers,
	})
}

func parsePersonality(raw *string) map[string]string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}

	result := map[string]string{
		"Nickname":    "*Unknown*",
		"Age":         "*Unknown*",
		"Nationality": "*Unknown*",
		"Height":      "*Unknown*",
		"Weight":      "*Unknown*",
		"Like":        "*Unknown*",
		"Dislike":     "*Unknown*",
	}

	patterns := map[string]*regexp.Regexp{
		"Nickname":    regexp.MustCompile(`(?mi)(?:Nickname|Alias(?:es)?)\s*[:\-]\s*(.+)$`),
		"Age":         regexp.MustCompile(`(?mi)Age\s*[:\-]\s*(.+)$`),
		"Nationality": regexp.MustCompile(`(?mi)(?:Nationality|Country)\s*[:\-]\s*(.+)$`),
		"Height":      regexp.MustCompile(`(?mi)Height\s*[:\-]\s*(.+)$`),
		"Weight":      regexp.MustCompile(`(?mi)Weight\s*[:\-]\s*(.+)$`),
		"Like":        regexp.MustCompile(`(?mi)Like(?:s)?\s*[:\-]\s*(.+)$`),
		"Dislike":     regexp.MustCompile(`(?mi)Dislike(?:s)?\s*[:\-]\s*(.+)$`),
	}

	for key, pattern := range patterns {
		matches := pattern.FindStringSubmatch(*raw)
		if len(matches) > 1 {
			val := strings.TrimSpace(matches[1])
			val = strings.Trim(val, `"'*`)
			val = strings.TrimSuffix(val, ",")
			val = strings.TrimSpace(val)
			if val != "" {
				result[key] = val
			}
		}
	}

	return result
}
