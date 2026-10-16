package api

import (
	"net/http"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

type calendarItem struct {
	BotID    string  `json:"bot_id"`
	Name     string  `json:"name"`
	ImageURL string  `json:"image_url"`
	Birthday *string `json:"birthday"`
}

// CalendarIndex melayani GET /v1/calendar.
// Bond system dihapus: mengembalikan SEMUA bot yang punya birthday
// (sebelumnya hanya bonded-bots) dengan struktur JSON yang sama.
func CalendarIndex(c *gin.Context) {
	var bots []struct {
		BotID    string
		Name     string
		Avatar   *string
		Birthday *time.Time
	}
	database.DB.Model(&models.Bot{}).
		Where("birthday IS NOT NULL").
		Select("bot_id, name, avatar, birthday").
		Find(&bots)

	response := make([]calendarItem, 0)

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

		response = append(response, calendarItem{
			BotID:    bot.BotID,
			Name:     bot.Name,
			ImageURL: services.ImageURL(avatar),
			Birthday: birthdayStr,
		})
	}

	c.JSON(http.StatusOK, response)
}
