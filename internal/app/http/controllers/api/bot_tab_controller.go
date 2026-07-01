package api

import (
	"fmt"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"github.com/gofiber/fiber/v2"
)

func BotTabIndex(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	var bonds []models.UserBotBond
	database.DB.Preload("Bot").Where("user_id = ?", user.ID).Find(&bonds)

	appUrl := config.Env("APP_URL", "http://127.0.0.1")
	var response []fiber.Map

	for _, bond := range bonds {
		bot := bond.Bot
		if bot == nil {
			continue
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
			"image_url":   fmt.Sprintf("%s/api/image?path=Bot/%s/%s", appUrl, bot.BotID, avatar),
			"color":       color,
			"birthday":    birthdayStr,
			"bond_hearts": bond.GetLevel(),
		})
	}

	if response == nil {
		response = make([]fiber.Map, 0)
	}

	return c.JSON(response)
}

func RecruitedBotsIndex(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	var bonds []models.UserBotBond
	database.DB.Preload("Bot").Where("user_id = ?", user.ID).Find(&bonds)

	appUrl := config.Env("APP_URL", "http://127.0.0.1")
	var response []fiber.Map

	for _, bond := range bonds {
		bot := bond.Bot
		if bot == nil {
			continue
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
			"image_url":   fmt.Sprintf("%s/api/image?path=Bot/%s/%s", appUrl, bot.BotID, avatar),
			"color":       color,
			"birthday":    birthdayStr,
			"description": bot.Description,
		})
	}

	if response == nil {
		response = make([]fiber.Map, 0)
	}

	return c.JSON(response)
}
