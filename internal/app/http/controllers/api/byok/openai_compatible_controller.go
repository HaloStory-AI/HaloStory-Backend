package byok

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/services"
	byokSvc "halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OpenAICompatibleChatRequest struct {
	Messages         []byokSvc.Message    `json:"messages" binding:"required"`
	APIKey           string               `json:"api_key" binding:"required"`
	BaseURL          string               `json:"base_url" binding:"required"`
	Model            string               `json:"model"`
	Temperature      *float64             `json:"temperature,omitempty"`
	MaxTokens        *int                 `json:"max_tokens,omitempty"`
	TopP             *float64             `json:"top_p,omitempty"`
	FrequencyPenalty *float64             `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64             `json:"presence_penalty,omitempty"`
	Stop             []string             `json:"stop,omitempty"`
	Reasoning        *byokSvc.ReasoningConfig `json:"reasoning,omitempty"`
	Summary          *string              `json:"summary,omitempty"`
	IsSummarize      *bool                `json:"is_summarize,omitempty"`
	ExistingMemories []string             `json:"existing_memories,omitempty"`
	Language         *string              `json:"language,omitempty"`
	HistoryDepth     *int                 `json:"history_depth,omitempty"`
	IsGroup          *bool                `json:"is_group,omitempty"`
	IsDisableBond    *bool                `json:"isdisablebond,omitempty"`
	UserPersona      *string              `json:"user_persona,omitempty"`
	BotIDs           []string             `json:"bot_ids,omitempty"`
	BotID            *string              `json:"bot_id,omitempty"`
}

func OpenAICompatibleChat(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var req OpenAICompatibleChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	if req.APIKey == "" || req.BaseURL == "" || len(req.Messages) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": "api_key, base_url, and messages are required",
		})
		return
	}

	model := req.Model
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	encryptedKey, err := utils.EncryptBYOKAPIKey(req.APIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Encryption failed",
			"message": "Failed to encrypt API key",
		})
		return
	}

	messagesJSON, err := json.Marshal(req.Messages)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Serialization failed",
			"message": "Failed to serialize messages",
		})
		return
	}
	messagesB64 := base64.StdEncoding.EncodeToString(messagesJSON)

	uid := uuid.New().String()

	payload := buildOpenAIPayload(req, encryptedKey, messagesB64, user.ID, uid, model)

	payloadJSON, _ := json.Marshal(payload)

	if err := services.StoreChatResult(uid, payloadJSON, 10*time.Minute); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Redis error",
			"message": "Failed to store chat request",
		})
		return
	}

	if err := services.Enqueue("ai-chat", payloadJSON); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Queue error",
			"message": "Failed to enqueue chat job",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "queued",
		"uuid":   uid,
	})
}

func buildOpenAIPayload(req OpenAICompatibleChatRequest, encryptedKey, messagesB64 string, userID uint, uid, model string) map[string]interface{} {
	payload := map[string]interface{}{
		"uuid":     uid,
		"status":   "queued",
		"api_key":  encryptedKey,
		"messages": messagesB64,
		"base_url": req.BaseURL,
		"model":    model,
		"provider": "openai-compatible",
		"user_id":  userID,
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
	if len(req.Stop) > 0 {
		payload["stop"] = req.Stop
	}
	if req.PresencePenalty != nil {
		payload["presence_penalty"] = *req.PresencePenalty
	}
	if req.FrequencyPenalty != nil {
		payload["frequency_penalty"] = *req.FrequencyPenalty
	}
	if req.Reasoning != nil {
		payload["reasoning"] = req.Reasoning
	}
	if req.Summary != nil {
		payload["summary"] = *req.Summary
	}
	if req.IsSummarize != nil {
		payload["is_summarize"] = *req.IsSummarize
	}
	if req.BotID != nil {
		payload["bot_id"] = *req.BotID
	}
	if req.Language != nil {
		payload["language"] = *req.Language
	}
	if req.HistoryDepth != nil {
		payload["history_depth"] = *req.HistoryDepth
	}
	if req.IsGroup != nil {
		payload["is_group"] = *req.IsGroup
	}
	if req.IsDisableBond != nil {
		payload["isdisablebond"] = *req.IsDisableBond
	}
	if req.UserPersona != nil {
		payload["user_persona"] = *req.UserPersona
	}
	if len(req.BotIDs) > 0 {
		payload["bot_ids"] = req.BotIDs
	}
	if len(req.ExistingMemories) > 0 {
		payload["existing_memories"] = req.ExistingMemories
	}
	return payload
}
