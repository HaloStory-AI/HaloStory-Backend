package api

import (
	"fmt"
	"net/http"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"github.com/gin-gonic/gin"
)

type calendarItem struct {
	BotID    string  `json:"bot_id"`
	Name     string  `json:"name"`
	ImageURL string  `json:"image_url"`
	Birthday *string `json:"birthday"`
}

func CalendarIndex(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var botIDs []string
	database.DB.Model(&models.UserBotBond{}).
		Where("user_id = ?", user.ID).
		Pluck("bot_id", &botIDs)

	if len(botIDs) == 0 {
		c.JSON(http.StatusOK, []calendarItem{})
		return
	}

	var bots []struct {
		BotID    string
		Name     string
		Avatar   *string
		Birthday *time.Time
	}
	database.DB.Model(&models.Bot{}).
		Where("bot_id IN ? AND birthday IS NOT NULL", botIDs).
		Select("bot_id, name, avatar, birthday").
		Find(&bots)

	appUrl := config.Env("APP_URL", "http://127.0.0.1")
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
			ImageURL: fmt.Sprintf("%s/api/image?path=%s", appUrl, avatar),
			Birthday: birthdayStr,
		})
	}

	c.JSON(http.StatusOK, response)
}
