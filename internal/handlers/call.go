package handlers

import (
	"math/rand"

	"rafiq8k-moga/halostory-go-backend/internal/config"

	"github.com/gofiber/fiber/v2"
)

func GetCall(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if len(cfg.CallVideos) == 0 {
			return c.JSON(fiber.Map{"url": ""})
		}
		url := cfg.CallVideos[rand.Intn(len(cfg.CallVideos))]
		return c.JSON(fiber.Map{"url": url})
	}
}
