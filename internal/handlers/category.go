package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CategoryHandler struct {
	db *gorm.DB
}

func NewCategoryHandler(db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{db: db}
}

func (h *CategoryHandler) Index(c *fiber.Ctx) error {
	var categories []models.Category
	if err := h.db.Order("pinned desc, name asc").Find(&categories).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch categories"})
	}
	return c.JSON(fiber.Map{"categories": categories})
}

func (h *CategoryHandler) Bots(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var category models.Category
	if err := h.db.Where("slug = ?", slug).Preload("Bots").First(&category).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Category not found"})
	}

	return c.JSON(fiber.Map{"category": category, "bots": category.Bots})
}
