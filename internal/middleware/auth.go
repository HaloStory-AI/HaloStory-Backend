package middleware

import (
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	Sub           uint   `json:"sub"`
	Role          string `json:"role"`
	IsVip         bool   `json:"is_vip"`
	EmailVerified bool   `json:"email_verified"`
	jwt.RegisteredClaims
}

func JWTAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := c.Get("Authorization")
		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		} else {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}

		token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		claims, ok := token.Claims.(*UserClaims)
		if !ok || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid token claims",
			})
		}

		c.Locals("userID", claims.Sub)
		c.Locals("userRole", claims.Role)
		c.Locals("isVip", claims.IsVip)
		c.Locals("emailVerified", claims.EmailVerified)
		c.Locals("claims", claims)

		// Sliding expiry: re-issue if within last 24h of TTL
		exp := claims.ExpiresAt.Time
		if !exp.IsZero() && time.Until(exp) < 24*time.Hour {
			newToken, err := GenerateJWT(cfg, claims.Sub, claims.Role, claims.IsVip, claims.EmailVerified)
			if err == nil {
				c.Set("X-Refresh-Token", newToken)
			}
		}

		return c.Next()
	}
}

func GetUserID(c *fiber.Ctx) uint {
	if uid, ok := c.Locals("userID").(uint); ok {
		return uid
	}
	return 0
}

func GetUserClaims(c *fiber.Ctx) *UserClaims {
	if claims, ok := c.Locals("claims").(*UserClaims); ok {
		return claims
	}
	return nil
}

func IsVip(c *fiber.Ctx) bool {
	if v, ok := c.Locals("isVip").(bool); ok {
		return v
	}
	return false
}

func GetUserFromLocals(c *fiber.Ctx) *models.User {
	if user, ok := c.Locals("user").(*models.User); ok {
		return user
	}
	return nil
}

func GenerateJWT(cfg *config.Config, userID uint, role string, isVip bool, emailVerified bool) (string, error) {
	now := time.Now()
	claims := &UserClaims{
		Sub:           userID,
		Role:          role,
		IsVip:         isVip,
		EmailVerified: emailVerified,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.JWTTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}
