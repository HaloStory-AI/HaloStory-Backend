package middlewares

import (
	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

// ActivityLogger mencatat api_hit untuk request terautentikasi (setelah Protected).
func ActivityLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		// Hanya catat untuk user login dan response sukses (2xx)
		if c.Writer.Status() < 200 || c.Writer.Status() >= 400 {
			return
		}
		val, ok := c.Get("user")
		if !ok {
			return
		}
		user, ok := val.(models.User)
		if !ok || user.ID == 0 {
			return
		}
		botID := c.Param("bot_id")
		if botID == "" {
			botID = c.Param("slug")
			if botID == "" {
				botID = c.Query("bot_id")
			}
		}
		services.LogActivity(user.ID, "api_hit", botID, "", c.ClientIP(), c.GetHeader("User-Agent"), nil)
	}
}
