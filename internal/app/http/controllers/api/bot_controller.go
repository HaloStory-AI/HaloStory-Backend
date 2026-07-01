package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"github.com/gofiber/fiber/v2"
)

// Index menampilkan seluruh bot
func BotIndex(c *fiber.Ctx) error {
	var bots []models.Bot
	database.DB.Select("bot_id, name, description, avatar, jailbreak, personality, chat_examples, birthday, sbr, color, child").Find(&bots)

	var bonds []models.UserBotBond
	// Ambil dari context jika ada
	userVal := c.Locals("user")
	if userVal != nil {
		user := userVal.(models.User)
		database.DB.Where("user_id = ?", user.ID).Find(&bonds)
	}

	bondMap := make(map[string]models.UserBotBond)
	for _, b := range bonds {
		bondMap[b.BotID] = b
	}

	appUrl := config.Env("APP_URL", "http://127.0.0.1")

	var response []fiber.Map
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

		response = append(response, fiber.Map{
			"id":            bot.BotID,
			"name":          bot.Name,
			"description":   bot.Description,
			"avatar":        bot.Avatar,
			"image_url":     fmt.Sprintf("%s/api/image?path=Bot/%s/%s", appUrl, bot.BotID, avatar),
			"background":    bot.Jailbreak,
			"personality":   bot.Personality,
			"chat_examples": bot.ChatExamples,
			"birthday":      birthdayStr,
			"bond_xp":       bondXp,
			"bond_hearts":   bondHearts,
			"sbr":           sbr,
			"color":         color,
			"child":         bot.Child,
		})
	}

	// Tangani kasus slice nil menjadi empty array []
	if response == nil {
		response = make([]fiber.Map, 0)
	}

	return c.JSON(response)
}

// ShowBio mengekstrak profil dari personality string via RegEx
func ShowBio(c *fiber.Ctx) error {
	botID := c.Params("bot_id")
	var bot models.Bot

	if err := database.DB.Where("bot_id = ?", botID).First(&bot).Error; err != nil {
		return c.JSON(fiber.Map{
			"description": "",
			"personality": nil,
		})
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

	return c.JSON(fiber.Map{
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
			if strings.HasSuffix(val, ",") {
				val = strings.TrimSuffix(val, ",")
			}
			val = strings.TrimSpace(val)
			if val != "" {
				result[key] = val
			}
		}
	}

	return result
}
