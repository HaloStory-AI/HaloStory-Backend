package api

import (
	"net/http"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

type storyItem struct {
	ID        uint    `json:"id"`
	BotID     string  `json:"bot_id"`
	BotName   string  `json:"bot_name"`
	ImageURL  string  `json:"image_url"`
	BotAvatar *string `json:"bot_avatar"`
	Rating    string  `json:"rating"`
	CreatedAt string  `json:"created_at"`
}

// GetByBondedBots melayani GET /v1/stories.
// Bond system dihapus: tidak ada lagi filter bonded-bots. Mengembalikan
// story 24 jam terakhir (rating "g") dari SEMUA bot dengan struktur
// JSON yang sama seperti sebelumnya agar frontend tidak crash.
func GetByBondedBots(c *gin.Context) {
	var stories []models.CharacterStory
	database.DB.Where("rating = ? AND created_at >= ?",
		"g", time.Now().Add(-24*time.Hour)).
		Order("bot_id asc, created_at desc").
		Find(&stories)

	if len(stories) == 0 {
		c.JSON(http.StatusOK, []storyItem{})
		return
	}

	botIDs := make([]string, 0, len(stories))
	seen := make(map[string]bool)
	for _, s := range stories {
		if !seen[s.BotID] {
			seen[s.BotID] = true
			botIDs = append(botIDs, s.BotID)
		}
	}

	type botAvatarRow struct {
		BotID  string
		Avatar *string
	}
	var avatars []botAvatarRow
	database.DB.Model(&models.Bot{}).
		Where("bot_id IN ?", botIDs).
		Select("bot_id, avatar").
		Find(&avatars)

	avatarMap := make(map[string]*string)
	for _, a := range avatars {
		avatarMap[a.BotID] = a.Avatar
	}

	response := make([]storyItem, 0)
	for _, s := range stories {
		var botAvatar *string
		if avatar, ok := avatarMap[s.BotID]; ok && avatar != nil {
			url := services.ImageURL(*avatar)
			botAvatar = &url
		}

		response = append(response, storyItem{
			ID:        s.ID,
			BotID:     s.BotID,
			BotName:   s.BotName,
			ImageURL:  s.ImagePath,
			BotAvatar: botAvatar,
			Rating:    s.Rating,
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, response)
}
