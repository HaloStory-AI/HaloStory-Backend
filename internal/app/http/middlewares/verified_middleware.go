package middlewares

import (
	"net/http"

	"halostory_go_backend/internal/app/models"

	"github.com/gin-gonic/gin"
)

func EnsureEmailIsVerified() gin.HandlerFunc {
	return func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Unauthorized",
			})
			return
		}
		user := userVal.(models.User)
		if user.EmailVerifiedAt == nil {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"message": "Your email address is not verified.",
			})
			return
		}
		c.Next()
	}
}
