package middlewares

import (
	"time"

	"halostory_go_backend/internal/app/models"

	"net/http"

	"github.com/gin-gonic/gin"
)

// CheckBanned adalah middleware untuk memastikan user belum di-banned.
// Middleware ini HARUS diletakkan setelah middleware Protected() (JWT).
func CheckBanned() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ambil user dari context (diset oleh Protected() JWT Middleware)
		userVal, exists := c.Get("user")
		if !exists || userVal == nil {
			// Jika tidak ada user (mungkin lupa pasang JWT), langsung tolak
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "User belum terautentikasi.",
			})
			return
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message":      "Akun Anda telah dibanned.",
				"reason":       user.BanReason,
				"banned_until": bannedUntilStr,
			})
			return
		}

		// Lanjut ke controller berikutnya
		c.Next()
	}
}
