package middlewares

import (
	"strings"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Protected adalah middleware untuk memvalidasi JWT (mirip auth:sanctum di Laravel)
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Ambil header Authorization
		authHeader := c.Get("Authorization")

		// Cek apakah formatnya Bearer token
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Token tidak ditemukan atau format salah",
			})
		}

		// Ekstrak token dengan membuang tulisan "Bearer "
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse dan validasi token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Pastikan algoritma token sesuai
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			secretKey := config.Env("APP_KEY", "halo_story_default_secret_key")
			return []byte(secretKey), nil
		})

		// Jika error atau token tidak valid
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Token tidak valid atau sudah kedaluwarsa",
			})
		}

		// Ambil payload (claims) dari token
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Format payload token tidak valid",
			})
		}

		// Dapatkan userID (sub)
		userIDFloat, ok := claims["sub"].(float64) // JSON numbers di-decode sebagai float64 oleh map[string]interface{}
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "User ID tidak ditemukan dalam token",
			})
		}
		userID := uint(userIDFloat)

		// Ambil data user lengkap dari database
		var user models.User
		if err := database.DB.First(&user, userID).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "User tidak ditemukan di database",
			})
		}

		// Simpan object user ke dalam context request (bisa diakses di controller via c.Locals)
		c.Locals("user", user)

		// Lanjut ke controller/handler berikutnya
		return c.Next()
	}
}
