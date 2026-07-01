package utils

import (
	"time"

	"halostory_go_backend/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateJWT membuat JWT token baru berdasarkan user ID
func GenerateJWT(userID uint) (string, error) {
	// Ambil APP_KEY dari .env (atau default fallback jika kosong)
	secretKey := config.Env("APP_KEY", "halo_story_default_secret_key")

	// Buat claims (payload token)
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour * 24 * 7).Unix(), // expired dalam 7 hari
		"iat": time.Now().Unix(),
	}

	// Generate token menggunakan algoritma HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token dengan secret key
	return token.SignedString([]byte(secretKey))
}
