package handlers

import (
	"encoding/json"
	"fmt"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/jobs"
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services/ai"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AIHandler struct {
	db      *gorm.DB
	cfg     *config.Config
	rdb     *redis.Client
	client  *asynq.Client
}

func NewAIHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *AIHandler {
	var client *asynq.Client
	if rdb != nil {
		client = asynq.NewClient(asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
	}

	return &AIHandler{
		db:     db,
		cfg:    cfg,
		rdb:    rdb,
		client: client,
	}
}

// POST /ai/chat/system
func (h *AIHandler) SystemChat(c *fiber.Ctx) error {
	var req struct {
		Messages        []ai.Message `json:"messages"`
		BotID           string       `json:"bot_id"`
		Language        *string      `json:"language"`
		Temperature     *float64     `json:"temperature"`
		ResponseLength  *int         `json:"response_length"`
		MaxTokens       *int         `json:"max_tokens"`
		TopP            *float64     `json:"top_p"`
		HistoryDepth    *int         `json:"history_depth"`
		ReasoningEnabled *bool       `json:"reasoning_enabled"`
		Summary         *string      `json:"summary"`
		IsGroup         *bool        `json:"is_group"`
		IsDisableBond   *bool        `json:"isdisablebond"`
		BotIDs          []string     `json:"bot_ids"`
		UserPersona     *string      `json:"user_persona"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.ReasoningEnabled != nil && *req.ReasoningEnabled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "System AI Mode can't be used if 'Enable AI Reasoning' option is enabled. Consider to disable it to use System AI Mode or use BYOK AI Mode (Setup required for this mode)",
		})
	}

	uid := uuid.New().String()
	userID := middleware.GetUserID(c)
	isVip := middleware.IsVip(c)

	messagesJSON, _ := json.Marshal(req.Messages)

	payload := map[string]interface{}{
		"messages":    string(messagesJSON),
		"status":      "queued",
		"user_id":     userID,
		"is_priority": isVip,
		"is_summarize": false,
		"bot_id":      req.BotID,
	}

	if req.Language != nil {
		payload["language"] = *req.Language
	}
	if req.Summary != nil {
		payload["summary"] = *req.Summary
	}
	if req.IsDisableBond != nil {
		payload["isdisablebond"] = *req.IsDisableBond
	}
	if req.IsGroup != nil {
		payload["is_group"] = *req.IsGroup
	}
	if len(req.BotIDs) > 0 {
		payload["bot_ids"] = req.BotIDs
	}
	if req.UserPersona != nil {
		payload["user_persona"] = *req.UserPersona
	}
	if req.MaxTokens != nil {
		payload["max_tokens"] = *req.MaxTokens
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		payload["top_p"] = *req.TopP
	}

	payloadBytes, _ := json.Marshal(payload)
	chatKey := fmt.Sprintf("halostory:chat:%s", uid)
	h.rdb.Set(c.Context(), chatKey, string(payloadBytes), 0)
	h.rdb.Expire(c.Context(), chatKey, 600)

	if h.client != nil {
		task, err := jobs.NewSystemAIChatTask(uid, userID, req.BotID, req.IsGroup != nil && *req.IsGroup)
		if err == nil {
			h.client.Enqueue(task, asynq.Queue("ai-chat"))
		}
	}

	return c.JSON(fiber.Map{
		"status": "queued",
		"uuid":   uid,
	})
}

// POST /ai/summarize
func (h *AIHandler) Summarize(c *fiber.Ctx) error {
	var req struct {
		Messages        []ai.Message `json:"messages"`
		BotID           *string      `json:"bot_id"`
		Language        *string      `json:"language"`
		ReasoningEnabled *bool       `json:"reasoning_enabled"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.ReasoningEnabled != nil && *req.ReasoningEnabled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "System AI Mode can't be used if 'Enable AI Reasoning' option is enabled.",
		})
	}

	uid := uuid.New().String()
	userID := middleware.GetUserID(c)
	isVip := middleware.IsVip(c)

	language := "Indonesian"
	if req.Language != nil {
		language = *req.Language
	}

	summarizationPrompt := ai.BuildSummarizationPrompt(req.Messages, language)

	botID := "default"
	if req.BotID != nil {
		botID = *req.BotID
	}

	payload := map[string]interface{}{
		"status":             "queued",
		"user_id":            userID,
		"bot_id":             botID,
		"is_priority":        isVip,
		"is_summarize":       true,
		"summarization_prompt": summarizationPrompt,
		"language":           language,
	}

	payloadBytes, _ := json.Marshal(payload)
	chatKey := fmt.Sprintf("halostory:chat:%s", uid)
	h.rdb.Set(c.Context(), chatKey, string(payloadBytes), 0)
	h.rdb.Expire(c.Context(), chatKey, 600)

	if h.client != nil {
		task, err := jobs.NewSystemAIChatTask(uid, userID, botID, false)
		if err == nil {
			h.client.Enqueue(task, asynq.Queue("ai-chat"))
		}
	}

	return c.JSON(fiber.Map{
		"status": "queued",
		"uuid":   uid,
	})
}

// GET /chat/status/:uuid
func (h *AIHandler) ChatStatus(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	if uuid == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "UUID is required"})
	}

	chatKey := fmt.Sprintf("halostory:chat:%s", uuid)
	data, err := h.rdb.Get(c.Context(), chatKey).Result()
	if err == redis.Nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Chat not found or expired"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get chat status"})
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(data), &result)

	return c.JSON(result)
}

// GET /ai/system-prompt/:bot_id
func (h *AIHandler) GetSystemPrompt(c *fiber.Ctx) error {
	botID := c.Params("bot_id")
	language := c.Query("language", "Indonesian")

	var bot models.Bot
	if err := h.db.Where("bot_id = ?", botID).First(&bot).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Bot not found"})
	}

	prompt := ai.FormatGlobalPrompt(nil, &bot, nil, language, false, false, "", 0, "")

	return c.JSON(fiber.Map{"system_prompt": prompt})
}

// POST /byok/:provider/chat (placeholder for now, full implementation in Phase 3)
func (h *AIHandler) BYOKChat(c *fiber.Ctx) error {
	provider := c.Params("provider")
	_ = provider
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error":   "Not implemented",
		"message": "BYOK chat will be implemented in Phase 3",
	})
}

// POST /byok/models (placeholder for now, full implementation in Phase 3)
func (h *AIHandler) BYOKModels(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error":   "Not implemented",
		"message": "BYOK models will be implemented in Phase 3",
	})
}
