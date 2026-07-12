package middlewares

import (
	"net/http"

	"halostory_go_backend/internal/app/models"

	"github.com/gin-gonic/gin"
)

func CheckAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Unauthenticated.",
			})
			return
		}

		user := userVal.(models.User)
		if user.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message": "Unauthorized. Admin access required.",
			})
			return
		}

		c.Next()
	}
}
