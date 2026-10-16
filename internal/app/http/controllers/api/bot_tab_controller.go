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

// BotTabIndex melayani GET /v1/bot-tab.
// Bond system dihapus: tidak ada lagi konsep recruit. Mengembalikan
// SEMUA bot (bond_hearts selalu 0, field dipertahankan agar frontend
// tidak crash) sehingga chat tetap bisa diakses tanpa gate recruit.
func BotTabIndex(c *gin.Context) {
	var bots []models.Bot
	database.DB.Order("name asc").Select("bot_id, name, avatar, birthday, color, child").Find(&bots)

	response := make([]botTabItem, 0)

	for _, bot := range bots {
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
			BondHearts: 0,
			Child:      bot.Child,
		})
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

// RecruitedBotsIndex melayani GET /v1/recruited-bots.
// Bond system dihapus: mengembalikan SEMUA bot dengan struktur JSON
// yang sama (list kosong hanya jika tidak ada bot sama sekali).
func RecruitedBotsIndex(c *gin.Context) {
	var bots []models.Bot
	database.DB.Order("name asc").Select("bot_id, name, description, avatar, birthday, color, child").Find(&bots)

	response := make([]recruitedBotItem, 0)

	for _, bot := range bots {
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

	c.JSON(http.StatusOK, response)
}
