package byok

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/services"
	byokSvc "halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GeminiChat(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var req byokSvc.BYOKRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	if req.APIKey == "" || req.Model == "" || len(req.Messages) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": "api_key, model, and messages are required",
		})
		return
	}

	if err := byokSvc.SanitizeByokInput(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Input validation failed",
			"message": err.Message,
		})
		return
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
	traceID := c.GetHeader("X-Request-Id")
	if traceID == "" {
		traceID = c.GetHeader("X-Trace-Id")
	}
	if traceID == "" {
		traceID = uid
	}
	c.Header("X-Request-Id", traceID)
	c.Header("X-Trace-Id", traceID)

	payload := buildPayload(req, encryptedKey, messagesB64, user.ID, uid)
	payload["trace_id"] = traceID

	payloadJSON, _ := json.Marshal(payload)

	if viaDB, err := services.StoreChatRequest(uid, payloadJSON, 10*time.Minute); err != nil {
		log.Printf("Chat %s: store gagal (redis+db): %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Store error",
			"message": "Failed to store chat request",
		})
		return
	} else if viaDB {
		c.Header("X-Degraded", "db-fallback")
	}

	if err := services.EnqueueResilient("ai-chat", uid, payloadJSON); err != nil {
		log.Printf("Chat %s: enqueue gagal (redis+db): %v", uid, err)
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

func buildPayload(req byokSvc.BYOKRequest, encryptedKey, messagesB64 string, userID uint, uid string) map[string]interface{} {
	payload := map[string]interface{}{
		"uuid":     uid,
		"status":   "queued",
		"api_key":  encryptedKey,
		"messages": messagesB64,
		"model":    req.Model,
		"provider": "gemini",
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
	if req.TopK != nil {
		payload["top_k"] = *req.TopK
	}
	if req.ResponseLength != nil {
		payload["response_length"] = *req.ResponseLength
	}
	if req.BaseURL != nil {
		payload["base_url"] = *req.BaseURL
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
	if req.UserPersona != nil {
		payload["user_persona"] = *req.UserPersona
	}
	if len(req.BotIDs) > 0 {
		payload["bot_ids"] = req.BotIDs
	}
	if len(req.ExistingMemories) > 0 {
		payload["existing_memories"] = req.ExistingMemories
	}
	if req.ContextLimit != nil {
		payload["context_limit"] = *req.ContextLimit
	}
	if req.Stream != nil && *req.Stream {
		payload["is_stream"] = true
	}
	return payload
}
