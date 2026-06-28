package middleware

import (
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func BannedCheck(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == 0 {
			return c.Next()
		}

		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			return c.Next()
		}

		if user.IsBanned() {
			var bannedUntil *string
			if user.BannedUntil != nil {
				s := user.BannedUntil.Format("2006-01-02T15:04:05-07:00")
				bannedUntil = &s
			}

			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"message":      "Akun Anda telah dibanned.",
				"reason":       user.BanReason,
				"banned_until": bannedUntil,
			})
		}

		return c.Next()
	}
}
