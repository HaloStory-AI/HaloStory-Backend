package middlewares

import (
	"halostory_go_backend/internal/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func CheckStaticToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		staticToken := config.Env("API_STATIC_TOKEN", "")
		if staticToken == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": "Server configuration error: API_STATIC_TOKEN not set",
			})
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Unauthorized entry. Invalid static token.",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != staticToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Unauthorized entry. Invalid static token.",
			})
			return
		}

		c.Next()
	}
}
