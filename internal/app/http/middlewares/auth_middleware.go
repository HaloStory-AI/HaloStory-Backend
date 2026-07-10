package middlewares

import (
	"strings"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Protected adalah middleware untuk memvalidasi JWT (mirip auth:sanctum di Laravel)
func Protected() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ambil header Authorization
		authHeader := c.GetHeader("Authorization")

		// Cek apakah formatnya Bearer token
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Token tidak ditemukan atau format salah",
			})
			return
		}

		// Ekstrak token dengan membuang tulisan "Bearer "
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Parse dan validasi token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Pastikan algoritma token sesuai
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			secretKey := config.Env("APP_KEY", "halo_story_default_secret_key")
			return []byte(secretKey), nil
		})

		// Jika error atau token tidak valid
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Token tidak valid atau sudah kedaluwarsa",
			})
			return
		}

		// Ambil payload (claims) dari token
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "Format payload token tidak valid",
			})
			return
		}

		// Dapatkan userID (sub)
		userIDFloat, ok := claims["sub"].(float64)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "User ID tidak ditemukan dalam token",
			})
			return
		}
		userID := uint(userIDFloat)

		// Ambil data user lengkap dari database
		var user models.User
		if err := database.DB.First(&user, userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "User tidak ditemukan di database",
			})
			return
		}

		// Simpan object user ke dalam context request
		c.Set("user", user)

		// Lanjut ke controller/handler berikutnya
		c.Next()
	}
}
