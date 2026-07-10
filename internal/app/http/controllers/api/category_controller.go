package api

import (
	"fmt"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"net/http"

	"github.com/gin-gonic/gin"
)

type categoryIndexResponse struct {
	Pinned   []gin.H `json:"pinned"`
	Featured []gin.H `json:"featured"`
	Default  []gin.H `json:"default"`
}

type botItemResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Avatar      *string `json:"avatar"`
	ImageURL    string  `json:"image_url"`
	Birthday    *string `json:"birthday"`
	BondXp      int     `json:"bond_xp"`
	BondHearts  int     `json:"bond_hearts"`
	Sbr         string  `json:"sbr"`
	Color       string  `json:"color"`
	Child       bool    `json:"child"`
}

func CategoryIndex(c *gin.Context) {
	var categories []models.Category
	database.DB.Order("name asc").Find(&categories)

	appUrl := config.Env("APP_URL", "http://127.0.0.1")

	pinned := make([]gin.H, 0)
	featured := make([]gin.H, 0)
	defaultCat := make([]gin.H, 0)

	for _, cat := range categories {
		logoUrl := ""
		if cat.LogoUrl != nil {
			logoUrl = *cat.LogoUrl
		}

		item := gin.H{
			"id":       cat.ID,
			"name":     cat.Name,
			"slug":     cat.Slug,
			"logo_url": fmt.Sprintf("%s/api/image?path=%s", appUrl, logoUrl),
		}

		if cat.Pinned {
			pinned = append(pinned, item)
		} else if cat.Featured {
			featured = append(featured, item)
		} else {
			defaultCat = append(defaultCat, item)
		}
	}

	c.JSON(http.StatusOK, categoryIndexResponse{
		Pinned:   pinned,
		Featured: featured,
		Default:  defaultCat,
	})
}

func CategoryBots(c *gin.Context) {
	slug := c.Param("slug")

	userVal, exists := c.Get("user")
	var bonds []models.UserBotBond
	if exists && userVal != nil {
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

	var response []botItemResponse
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
			str := bot.Birthday.Format("2007-01-02")
			birthdayStr = &str
		}

		response = append(response, botItemResponse{
			ID:          bot.BotID,
			Name:        bot.Name,
			Description: bot.Description,
			Avatar:      bot.Avatar,
			ImageURL:    fmt.Sprintf("%s/api/image?path=%s", appUrl, avatar),
			Birthday:    birthdayStr,
			BondXp:      bondXp,
			BondHearts:  bondHearts,
			Sbr:         sbr,
			Color:       color,
			Child:       bot.Child,
		})
	}

	if response == nil {
		response = make([]botItemResponse, 0)
	}

	c.JSON(http.StatusOK, response)
}
