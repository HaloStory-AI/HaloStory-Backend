package utils

import (
	"errors"
	"time"

	"halostory_go_backend/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateJWT membuat JWT token baru berdasarkan user ID
func GenerateJWT(userID uint) (string, error) {
	secretKey := config.Env("APP_KEY", "")
	if secretKey == "" {
		return "", errors.New("APP_KEY is not set in environment")
	}

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
