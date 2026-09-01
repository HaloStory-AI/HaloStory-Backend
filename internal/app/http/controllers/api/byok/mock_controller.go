package byok

import (
	"encoding/json"
	"net/http"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/services"
	byokSvc "halostory_go_backend/internal/services/byok"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MockChatRequest struct {
	Messages     []byokSvc.Message `json:"messages" binding:"required"`
	Model        string         `json:"model"`
	Temperature  *float64       `json:"temperature,omitempty"`
	MaxTokens    *int           `json:"max_tokens,omitempty"`
	TopP         *float64       `json:"top_p,omitempty"`
	TopK         *int           `json:"top_k,omitempty"`
	BotID        *string        `json:"bot_id,omitempty"`
	Language     *string        `json:"language,omitempty"`
	HistoryDepth *int           `json:"history_depth,omitempty"`
	IsSummarize  *bool          `json:"is_summarize,omitempty"`
	IsGroup      *bool          `json:"is_group,omitempty"`
}

func MockChat(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var req MockChatRequest
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
			"message": "messages is required",
		})
		return
	}

	model := req.Model
	if model == "" {
		model = "mock-model"
	}

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

	payload := buildMockPayload(req, user.ID, uid, model)
	payload["trace_id"] = traceID
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

func buildMockPayload(req MockChatRequest, userID uint, uid, model string) map[string]interface{} {
	payload := map[string]interface{}{
		"uuid":     uid,
		"status":   "queued",
		"model":    model,
		"provider": "mock",
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
	if req.BotID != nil {
		payload["bot_id"] = *req.BotID
	}
	if req.Language != nil {
		payload["language"] = *req.Language
	}
	if req.HistoryDepth != nil {
		payload["history_depth"] = *req.HistoryDepth
	}
	if req.IsSummarize != nil {
		payload["is_summarize"] = *req.IsSummarize
	}
	if req.IsGroup != nil {
		payload["is_group"] = *req.IsGroup
	}

	return payload
}
