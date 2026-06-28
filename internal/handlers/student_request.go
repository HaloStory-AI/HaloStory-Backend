package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type StudentRequestHandler struct {
	db      *gorm.DB
	discord *services.DiscordService
}

func NewStudentRequestHandler(db *gorm.DB, cfg *config.Config) *StudentRequestHandler {
	return &StudentRequestHandler{
		db:      db,
		discord: services.NewDiscordService(cfg),
	}
}

func (h *StudentRequestHandler) Index(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var requests []models.StudentRequest
	if err := h.db.Where("user_id = ?", userID).Order("created_at desc").Find(&requests).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch requests"})
	}

	return c.JSON(fiber.Map{"requests": requests})
}

func (h *StudentRequestHandler) Store(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	userName := ""

	var user models.User
	if err := h.db.First(&user, userID).Error; err == nil {
		userName = user.Name
	}

	var req struct {
		StudentName string `json:"student_name"`
	}
	if err := c.BodyParser(&req); err != nil || req.StudentName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Student name required"})
	}

	request := models.StudentRequest{
		UserID:      userID,
		StudentName: req.StudentName,
	}

	if err := h.db.Create(&request).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create request"})
	}

	go h.discord.SendStudentRequest(userName, req.StudentName)

	return c.Status(201).JSON(fiber.Map{"request": request})
}
