package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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

	ttlHours := config.EnvInt("JWT_TTL_HOURS", 24*7)
	expiresAt := time.Now().Add(time.Duration(ttlHours) * time.Hour)

	// Buat claims (payload token)
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": expiresAt.Unix(),
		"iat": time.Now().Unix(),
	}

	// Generate token menggunakan algoritma HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token dengan secret key
	return token.SignedString([]byte(secretKey))
}

// GenerateRefreshToken membuat opaque refresh token acak (256-bit hex).
// Tidak di-JWT karena disimpan ter-hash di database.
func GenerateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gagal generate refresh token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// HashToken menghitung SHA-256 hex dari sebuah token untuk disimpan di DB.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
