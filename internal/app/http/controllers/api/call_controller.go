package api

import (
	"math/rand"
	"net/http"
	"strings"

	"halostory_go_backend/internal/config"

	"github.com/gin-gonic/gin"
)

func GetCall(c *gin.Context) {
	videosStr := config.Env("CALL_VIDEOS", "https://archive.org/download/RickAstleyNeverGonnaGiveYouUp/RickAstleyNeverGonnaGiveYouUp.mp4")
	videos := strings.Split(videosStr, ",")

	url := strings.TrimSpace(videos[rand.Intn(len(videos))])

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"url":     url,
	})
}
