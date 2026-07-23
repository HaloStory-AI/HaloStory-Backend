package api

import (
	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"

	"net/http"

	"github.com/gin-gonic/gin"
)

type botTabItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ImageURL   string  `json:"image_url"`
	Color      *string `json:"color"`
	Birthday   *string `json:"birthday"`
	BondHearts int     `json:"bond_hearts"`
	Child      bool    `json:"child"`
}

func BotTabIndex(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var bonds []models.UserBotBond
	database.DB.Preload("Bot").Where("user_id = ?", user.ID).Find(&bonds)

	var response []botTabItem

	for _, bond := range bonds {
		bot := bond.Bot
		if bot == nil {
			continue
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

		response = append(response, botTabItem{
			ID:         bot.BotID,
			Name:       bot.Name,
			ImageURL:   services.ImageURL(avatar),
			Color:      bot.Color,
			Birthday:   birthdayStr,
			BondHearts: bond.GetLevel(),
			Child:      bot.Child,
		})
	}

	if response == nil {
		response = make([]botTabItem, 0)
	}

	c.JSON(http.StatusOK, response)
}

type recruitedBotItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ImageURL    string  `json:"image_url"`
	Color       string  `json:"color"`
	Birthday    *string `json:"birthday"`
	Description *string `json:"description"`
	Child       bool    `json:"child"`
}

func RecruitedBotsIndex(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var bonds []models.UserBotBond
	database.DB.Preload("Bot").Where("user_id = ?", user.ID).Find(&bonds)

	var response []recruitedBotItem

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

		response = append(response, recruitedBotItem{
			ID:          bot.BotID,
			Name:        bot.Name,
			ImageURL:    services.ImageURL(avatar),
			Color:       color,
			Birthday:    birthdayStr,
			Description: bot.Description,
			Child:       bot.Child,
		})
	}

	if response == nil {
		response = make([]recruitedBotItem, 0)
	}

	c.JSON(http.StatusOK, response)
}
