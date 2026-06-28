package byok

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/jobs"
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/services"
	"rafiq8k-moga/halostory-go-backend/internal/services/ai"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type BYOKHandler struct {
	db     *gorm.DB
	cfg    *config.Config
	rdb    *redis.Client
	client *asynq.Client
	crypto *services.CryptoService
}

func NewBYOKHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *BYOKHandler {
	var client *asynq.Client
	if rdb != nil {
		client = asynq.NewClient(asynq.RedisClientOpt{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
	}

	return &BYOKHandler{
		db:     db,
		cfg:    cfg,
		rdb:    rdb,
		client: client,
		crypto: services.NewCryptoService(cfg.JWTSecret),
	}
}

func (h *BYOKHandler) Chat(c *fiber.Ctx) error {
	provider := c.Params("provider")
	if provider == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Provider is required"})
	}

	if _, exists := ai.BYOKProviders[provider]; !exists {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Unknown provider: %s", provider)})
	}

	req, errResp := ai.SanitizeBYOKInput(c)
	if errResp != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(*errResp)
	}

	uid := uuid.New().String()
	userID := middleware.GetUserID(c)

	// Encrypt API key
	encryptedKey, err := h.crypto.Encrypt(req.APIKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to encrypt API key"})
	}

	// Base64 encode messages
	messagesJSON, _ := json.Marshal(req.Messages)
	messagesB64 := base64.StdEncoding.EncodeToString(messagesJSON)

	// Build payload for Redis
	payload := map[string]interface{}{
		"status":     "queued",
		"user_id":    userID,
		"bot_id":     "",
		"is_summarize": false,
		"is_byok":    true,
		"provider":   provider,
		"api_key":    encryptedKey,
		"api_key_encrypted": true,
		"model":      req.Model,
		"messages":   messagesB64,
	}

	if req.BaseURL != nil {
		payload["base_url"] = *req.BaseURL
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		payload["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		payload["top_p"] = *req.TopP
	}
	if req.BotID != nil {
		payload["bot_id"] = *req.BotID
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
	if req.HistoryDepth != nil {
		payload["history_depth"] = *req.HistoryDepth
	}
	if req.ResponseLength != nil {
		payload["response_length"] = *req.ResponseLength
	}

	payloadBytes, _ := json.Marshal(payload)
	chatKey := fmt.Sprintf("halostory:chat:%s", uid)
	h.rdb.Set(c.Context(), chatKey, string(payloadBytes), 0)
	h.rdb.Expire(c.Context(), chatKey, 600)

	if h.client != nil {
		task, err := jobs.NewBYOKChatTask(uid, userID, provider)
		if err == nil {
			h.client.Enqueue(task, asynq.Queue("ai-chat"))
		}
	}

	return c.JSON(fiber.Map{
		"status": "queued",
		"uuid":   uid,
	})
}

func (h *BYOKHandler) BYOKChatHandler() fiber.Handler {
	return h.Chat
}
