package middleware

import (
	"strings"

	"rafiq8k-moga/halostory-go-backend/internal/config"

	"github.com/gofiber/fiber/v2"
)

func StaticTokenAuth(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Get("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")

		if cfg.StaticToken == "" || token != cfg.StaticToken {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Unauthorized entry. Invalid static token.",
			})
		}

		return c.Next()
	}
}
