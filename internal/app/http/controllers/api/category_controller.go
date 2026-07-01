package api

import (
	"fmt"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"github.com/gofiber/fiber/v2"
)

func CategoryIndex(c *fiber.Ctx) error {
	var categories []models.Category
	database.DB.Order("name asc").Find(&categories)

	appUrl := config.Env("APP_URL", "http://127.0.0.1")

	pinned := make([]fiber.Map, 0)
	featured := make([]fiber.Map, 0)
	defaultCat := make([]fiber.Map, 0)

	for _, cat := range categories {
		logoUrl := ""
		if cat.LogoUrl != nil {
			logoUrl = *cat.LogoUrl
		}

		item := fiber.Map{
			"id":       cat.ID,
			"name":     cat.Name,
			"slug":     cat.Slug,
			"logo_url": fmt.Sprintf("%s/api/image?path=Category/%s", appUrl, logoUrl),
		}

		if cat.Pinned {
			pinned = append(pinned, item)
		} else if cat.Featured {
			featured = append(featured, item)
		} else {
			defaultCat = append(defaultCat, item)
		}
	}

	return c.JSON(fiber.Map{
		"pinned":   pinned,
		"featured": featured,
		"default":  defaultCat,
	})
}

func CategoryBots(c *fiber.Ctx) error {
	slug := c.Params("slug")
	
	// Kita asumsikan ini di bawah JWT protected route, jadi local "user" ada.
	// Jika tidak, kita bisa abaikan logika ini dan beri bond_hearts 0.
	userVal := c.Locals("user")
	var bonds []models.UserBotBond
	if userVal != nil {
		user := userVal.(models.User)
		database.DB.Where("user_id = ?", user.ID).Find(&bonds)
	}

	var bots []models.Bot
	database.DB.Where("category_slug = ?", slug).Order("name asc").Select("bot_id, name, description, avatar, birthday, sbr, color, child").Find(&bots)

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
			"id":          bot.BotID,
			"name":        bot.Name,
			"description": bot.Description,
			"avatar":      bot.Avatar,
			"image_url":   fmt.Sprintf("%s/api/image?path=Bot/%s/%s", appUrl, bot.BotID, avatar),
			"birthday":    birthdayStr,
			"bond_xp":     bondXp,
			"bond_hearts": bondHearts,
			"sbr":         sbr,
			"color":       color,
			"child":       bot.Child,
		})
	}

	if response == nil {
		response = make([]fiber.Map, 0)
	}

	return c.JSON(response)
}
