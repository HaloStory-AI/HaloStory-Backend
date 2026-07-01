package middlewares

import (
	"time"

	"halostory_go_backend/internal/app/models"

	"github.com/gofiber/fiber/v2"
)

// CheckBanned adalah middleware untuk memastikan user belum di-banned.
// Middleware ini HARUS diletakkan setelah middleware Protected() (JWT).
func CheckBanned() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Ambil user dari context (diset oleh Protected() JWT Middleware)
		userVal := c.Locals("user")
		if userVal == nil {
			// Jika tidak ada user (mungkin lupa pasang JWT), langsung tolak
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "User belum terautentikasi.",
			})
		}

		user := userVal.(models.User)

		// Cek apakah user sedang di-banned
		if user.HasActiveBan() {
			var bannedUntilStr *string
			if user.BannedUntil != nil {
				str := user.BannedUntil.Format(time.RFC3339) // Format ISO 8601
				bannedUntilStr = &str
			}

			// Return JSON 403 persis seperti format Laravel
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"message":      "Akun Anda telah dibanned.",
				"reason":       user.BanReason,
				"banned_until": bannedUntilStr,
			})
		}

		// Lanjut ke controller berikutnya
		return c.Next()
	}
}
