package api

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	byokSvc "halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services/systemai"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SystemChatRequest struct {
	Messages         []byokSvc.Message        `json:"messages" binding:"required"`
	BotID            string                   `json:"bot_id" binding:"required"`
	Language         *string                  `json:"language,omitempty"`
	Temperature      *float64                 `json:"temperature,omitempty"`
	MaxTokens        *int                     `json:"max_tokens,omitempty"`
	TopP             *float64                 `json:"top_p,omitempty"`
	TopK             *int                     `json:"top_k,omitempty"`
	ResponseLength   *int                     `json:"response_length,omitempty"`
	Summary          *string                  `json:"summary,omitempty"`
	HistoryDepth     *int                     `json:"history_depth,omitempty"`
	IsGroup          *bool                    `json:"is_group,omitempty"`
	IsDisableBond    *bool                    `json:"isdisablebond,omitempty"`
	UserPersona      *string                  `json:"user_persona,omitempty"`
	BotIDs           []string                 `json:"bot_ids,omitempty"`
	ExistingMemories []string                 `json:"existing_memories,omitempty"`
	Stop             []string                 `json:"stop,omitempty"`
	ReasoningEnabled *bool                    `json:"reasoning_enabled,omitempty"`
	ContextLimit     *int                     `json:"context_limit,omitempty"`
	Stream           *bool                    `json:"stream,omitempty"`
}

type SummarizeRequest struct {
	Messages         []byokSvc.Message `json:"messages" binding:"required"`
	BotID            string            `json:"bot_id" binding:"required"`
	Language         *string           `json:"language,omitempty"`
	ExistingMemories []string          `json:"existing_memories,omitempty"`
}

func SystemChat(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var req SystemChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	// Global low — system selalu reasoning low (hemat token/latensi).
	// Abaikan preferensi client; reasoning_enabled true/false tetap dipaksa low.
	_ = req.ReasoningEnabled

	if len(req.Messages) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": "messages are required",
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
	// Fase 0: trace_id propagation (X-Request-Id -> payload trace_id, echo headers)
	traceID := c.GetHeader("X-Request-Id")
	if traceID == "" {
		traceID = c.GetHeader("X-Trace-Id")
	}
	if traceID == "" {
		traceID = uid
	}
	c.Header("X-Request-Id", traceID)
	c.Header("X-Trace-Id", traceID)

	payload := map[string]interface{}{
		"uuid":          uid,
		"status":        "queued",
		"messages":      messagesB64,
		"provider":      "system-ai",
		"user_id":       user.ID,
		"bot_id":        req.BotID,
		"is_priority":   user.Role == "vip",
		"trace_id":      traceID,
	}

	if req.Language != nil {
		payload["language"] = *req.Language
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
	if req.Summary != nil {
		payload["summary"] = *req.Summary
	}
	if req.HistoryDepth != nil {
		payload["history_depth"] = *req.HistoryDepth
	}
	if req.ContextLimit != nil {
		payload["context_limit"] = *req.ContextLimit
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
	if len(req.Stop) > 0 {
		payload["stop"] = req.Stop
	}
	// Global low — system selalu reasoning low, abaikan input client.
	payload["reasoning"] = map[string]interface{}{"enabled": true, "effort": "low"}
	// Stream flag: nil/absen = false (pengguna lama tetap polling).
	if req.Stream != nil && *req.Stream {
		payload["is_stream"] = true
	}

	payloadJSON, _ := json.Marshal(payload)

	if viaDB, err := services.StoreChatRequest(uid, payloadJSON, systemai.ChatResultTTL); err != nil {
		log.Printf("Chat %s: store gagal (redis+db): %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Store error",
			"message": "Failed to store chat request",
		})
		return
	} else if viaDB {
		c.Header("X-Degraded", "db-fallback")
	}

	// Cooldown check: if in cooldown, push to pending queue instead
	cooldownActive, err := services.CheckCooldown(user.ID)
	if err == nil && cooldownActive {
		if err := services.PushPending(user.ID, uid); err != nil {
			log.Printf("Chat %s: push-pending gagal: %v — antre langsung", uid, err)
			if err2 := services.EnqueueResilient("ai-chat", uid, payloadJSON); err2 != nil {
				log.Printf("Chat %s: enqueue gagal (redis+db): %v", uid, err2)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Queue error",
					"message": "Failed to queue chat request",
				})
				return
			}
			c.Header("X-Degraded", "db-fallback")
			c.JSON(http.StatusOK, gin.H{
				"status": "queued",
				"uuid":   uid,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "queued",
			"uuid":   uid,
		})
		return
	}

	if err := services.EnqueueResilient("ai-chat", uid, payloadJSON); err != nil {
		log.Printf("Chat %s: enqueue gagal (redis+db): %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Queue error",
			"message": "Failed to enqueue chat job",
		})
		return
	}

	services.SetCooldown(user.ID, systemai.CooldownDuration)

	c.JSON(http.StatusOK, gin.H{
		"status": "queued",
		"uuid":   uid,
	})
}

func Summarize(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var req SummarizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": "messages are required",
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

	payload := map[string]interface{}{
		"uuid":          uid,
		"status":        "queued",
		"messages":      messagesB64,
		"provider":      "system-ai",
		"user_id":       user.ID,
		"bot_id":        req.BotID,
		"is_summarize":  true,
		"is_priority":   user.Role == "vip",
		"trace_id":      traceID,
	}

	language := "id"
	if req.Language != nil {
		language = *req.Language
	}
	payload["language"] = language

	if len(req.ExistingMemories) > 0 {
		payload["existing_memories"] = req.ExistingMemories
	}

	payloadJSON, _ := json.Marshal(payload)

	if viaDB, err := services.StoreChatRequest(uid, payloadJSON, systemai.ChatResultTTL); err != nil {
		log.Printf("Chat %s: store gagal (redis+db): %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Store error",
			"message": "Failed to store chat request",
		})
		return
	} else if viaDB {
		c.Header("X-Degraded", "db-fallback")
	}

	cooldownActive, err := services.CheckCooldown(user.ID)
	if err == nil && cooldownActive {
		if err := services.PushPending(user.ID, uid); err != nil {
			log.Printf("Chat %s: push-pending gagal: %v — antre langsung", uid, err)
			if err2 := services.EnqueueResilient("ai-chat", uid, payloadJSON); err2 != nil {
				log.Printf("Chat %s: enqueue gagal (redis+db): %v", uid, err2)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Queue error",
					"message": "Failed to queue chat request",
				})
				return
			}
			c.Header("X-Degraded", "db-fallback")
			c.JSON(http.StatusOK, gin.H{
				"status": "queued",
				"uuid":   uid,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "queued",
			"uuid":   uid,
		})
		return
	}

	if err := services.EnqueueResilient("ai-chat", uid, payloadJSON); err != nil {
		log.Printf("Chat %s: enqueue gagal (redis+db): %v", uid, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Queue error",
			"message": "Failed to enqueue chat job",
		})
		return
	}

	services.SetCooldown(user.ID, systemai.CooldownDuration)

	c.JSON(http.StatusOK, gin.H{
		"status": "queued",
		"uuid":   uid,
	})
}

func GetSystemPrompt(c *gin.Context) {
	botID := c.Param("bot_id")

	var bot models.Bot
	if err := database.DB.Where("bot_id = ?", botID).First(&bot).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not found",
			"message": "Bot tidak ditemukan",
		})
		return
	}

	lang := "id"
	langPtr := &lang
	prompt := byokSvc.FormatGlobalPrompt(&botID, langPtr, nil, nil, nil, nil, true, true)

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"system_prompt": prompt,
	})
}
