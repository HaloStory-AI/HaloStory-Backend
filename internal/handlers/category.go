package handlers

import (
	"encoding/json"

	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CategoryHandler struct {
	db    *gorm.DB
	cache *services.CacheService
}

func NewCategoryHandler(db *gorm.DB, cache *services.CacheService) *CategoryHandler {
	return &CategoryHandler{db: db, cache: cache}
}

func (h *CategoryHandler) Index(c *fiber.Ctx) error {
	cacheKey := services.CacheKey("categories")

	// Try cache first
	if h.cache != nil {
		if cached := h.cache.GetCachedResponse(cacheKey); cached != nil {
			c.Set("Content-Type", "application/json")
			return c.Send(cached)
		}
	}

	var categories []models.Category
	if err := h.db.Order("pinned desc, name asc").Find(&categories).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch categories"})
	}

	data := fiber.Map{"categories": categories}

	// Store in cache
	if h.cache != nil {
		if bytes, err := json.Marshal(data); err == nil {
			h.cache.CacheResponse(cacheKey, bytes, services.CacheTTLPublic)
		}
	}

	return c.JSON(data)
}

func (h *CategoryHandler) Bots(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var category models.Category
	if err := h.db.Where("slug = ?", slug).Preload("Bots").First(&category).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Category not found"})
	}

	type categoryBotsResponse struct {
		models.Category
		Bots []models.BotListResponse `json:"bots"`
	}

	return c.JSON(fiber.Map{"category": categoryBotsResponse{
		Category: category,
		Bots:     models.BotsToListResponse(category.Bots),
	}})
}
