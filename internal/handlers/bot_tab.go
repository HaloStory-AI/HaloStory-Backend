package handlers

import (
	"encoding/json"

	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BotTabHandler struct {
	db    *gorm.DB
	cache *services.CacheService
}

func NewBotTabHandler(db *gorm.DB, cache *services.CacheService) *BotTabHandler {
	return &BotTabHandler{db: db, cache: cache}
}

func (h *BotTabHandler) Index(c *fiber.Ctx) error {
	cacheKey := services.CacheKey("bot-tab")

	// Try cache first
	if h.cache != nil {
		if cached := h.cache.GetCachedResponse(cacheKey); cached != nil {
			c.Set("Content-Type", "application/json")
			return c.Send(cached)
		}
	}

	var categories []models.Category
	if err := h.db.Where("pinned = ? OR featured = ?", true, true).
		Preload("Bots", func(db *gorm.DB) *gorm.DB {
			return db.Order("name asc")
		}).
		Order("pinned desc, name asc").
		Find(&categories).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch bot tab"})
	}

	type categoryResponse struct {
		models.Category
		Bots []models.BotListResponse `json:"bots"`
	}
	result := make([]categoryResponse, len(categories))
	for i, cat := range categories {
		result[i] = categoryResponse{
			Category: cat,
			Bots:     models.BotsToListResponse(cat.Bots),
		}
	}

	data := fiber.Map{"categories": result}

	// Store in cache
	if h.cache != nil {
		if bytes, err := json.Marshal(data); err == nil {
			h.cache.CacheResponse(cacheKey, bytes, services.CacheTTLPublic)
		}
	}

	return c.JSON(data)
}
