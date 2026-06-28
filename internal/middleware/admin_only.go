package middleware

import (
	"github.com/gofiber/fiber/v2"
)

func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims := GetUserClaims(c)
		if claims == nil || claims.Role != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Forbidden",
			})
		}
		return c.Next()
	}
}
