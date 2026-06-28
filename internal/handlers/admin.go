package handlers

import (
	"encoding/csv"
	"strings"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db      *gorm.DB
	cfg     *config.Config
	discord *services.DiscordService
}

func NewAdminHandler(db *gorm.DB, cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		db:      db,
		cfg:     cfg,
		discord: services.NewDiscordService(cfg),
	}
}

// GET /admin/users
func (h *AdminHandler) GetUsers(c *fiber.Ctx) error {
	var users []models.User
	if err := h.db.Order("created_at desc").Find(&users).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch users"})
	}
	return c.JSON(fiber.Map{"users": users})
}

type changeRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=user admin vip"`
}

// PUT /admin/users/:id/role
func (h *AdminHandler) ChangeUserRole(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	var req changeRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if req.Role != "user" && req.Role != "admin" && req.Role != "vip" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid role"})
	}

	result := h.db.Model(&models.User{}).Where("id = ?", uint(id)).Update("role", req.Role)
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(fiber.Map{"message": "Role updated"})
}

// DELETE /admin/users/:id
func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	result := h.db.Delete(&models.User{}, uint(id))
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(fiber.Map{"message": "User deleted"})
}

// GET /admin/student-requests
func (h *AdminHandler) GetStudentRequests(c *fiber.Ctx) error {
	var requests []models.StudentRequest
	if err := h.db.Preload("User").Order("created_at desc").Find(&requests).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch requests"})
	}
	return c.JSON(fiber.Map{"requests": requests})
}

type updateRequestStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=approved rejected"`
}

// PUT /admin/student-requests/:id/status
func (h *AdminHandler) UpdateStudentRequestStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request ID"})
	}

	var req updateRequestStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	var request models.StudentRequest
	if err := h.db.Preload("User").First(&request, uint(id)).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Request not found"})
	}

	h.db.Model(&request).Update("status", req.Status)

	go func() {
		userName := request.User.Name
		if req.Status == "approved" {
			h.discord.SendRequestApproved(userName, request.StudentName)
		} else {
			h.discord.SendRequestRejected(userName, request.StudentName)
		}
	}()

	return c.JSON(fiber.Map{"message": "Request updated"})
}

// GET /admin/bots
func (h *AdminHandler) GetBots(c *fiber.Ctx) error {
	var bots []models.Bot
	if err := h.db.Preload("Category").Order("name asc").Find(&bots).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch bots"})
	}
	return c.JSON(fiber.Map{"bots": bots})
}

// POST /admin/bots
func (h *AdminHandler) StoreBot(c *fiber.Ctx) error {
	bot := new(models.Bot)
	if err := c.BodyParser(bot); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if err := h.db.Create(bot).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create bot: " + err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{"bot": bot})
}

// POST /admin/bots/import
func (h *AdminHandler) ImportBots(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "CSV file required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to open file"})
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid CSV format"})
	}

	if len(records) < 2 {
		return c.Status(400).JSON(fiber.Map{"error": "CSV must have header + at least 1 row"})
	}

	var imported int
	for _, row := range records[1:] {
		if len(row) < 1 {
			continue
		}
		bot := &models.Bot{
			BotID:        row[0],
			Name:         safeCSV(row, 1),
			CategorySlug: nullableString(safeCSV(row, 2)),
			Description:  nullableString(safeCSV(row, 3)),
			Avatar:       nullableString(safeCSV(row, 4)),
			Sbr:          nullableString(safeCSV(row, 5)),
		}
		if err := h.db.Create(bot).Error; err != nil {
			continue
		}
		imported++
	}

	return c.JSON(fiber.Map{"imported": imported})
}

// PUT /admin/bots/:id
func (h *AdminHandler) UpdateBot(c *fiber.Ctx) error {
	botID := c.Params("id")

	var bot models.Bot
	if err := h.db.First(&bot, "bot_id = ?", botID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Bot not found"})
	}

	updates := new(models.Bot)
	if err := c.BodyParser(updates); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	h.db.Model(&bot).Updates(updates)
	return c.JSON(fiber.Map{"bot": bot})
}

// DELETE /admin/bots/:id
func (h *AdminHandler) DeleteBot(c *fiber.Ctx) error {
	botID := c.Params("id")

	result := h.db.Delete(&models.Bot{}, "bot_id = ?", botID)
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "Bot not found"})
	}

	return c.JSON(fiber.Map{"message": "Bot deleted"})
}

func safeCSV(row []string, idx int) string {
	if idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func AdminOnly() fiber.Handler {
	return middleware.AdminOnly()
}
