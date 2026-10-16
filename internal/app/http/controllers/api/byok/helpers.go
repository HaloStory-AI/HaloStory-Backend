package byok

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/services"
	byokSvc "halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services/llm"
	"halostory_go_backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OpenAIProviderRequest struct {
	Messages          []byokSvc.Message        `json:"messages" binding:"required"`
	APIKey            string                   `json:"api_key" binding:"required"`
	Model             string                   `json:"model"`
	Temperature       *float64                 `json:"temperature,omitempty"`
	MaxTokens         *int                     `json:"max_tokens,omitempty"`
	TopP              *float64                 `json:"top_p,omitempty"`
	Stop              []string                 `json:"stop,omitempty"`
	FrequencyPenalty  *float64                 `json:"frequency_penalty,omitempty"`
	PresencePenalty   *float64                 `json:"presence_penalty,omitempty"`
	TopK              *int                     `json:"top_k,omitempty"`
	MinP              *float64                 `json:"min_p,omitempty"`
	RepetitionPenalty *float64                 `json:"repetition_penalty,omitempty"`
	IncludeReasoning  *bool                    `json:"include_reasoning,omitempty"`
	Reasoning         *byokSvc.ReasoningConfig `json:"reasoning,omitempty"`
	ResponseLength    *int                     `json:"response_length,omitempty"`
	Summary           *string                  `json:"summary,omitempty"`
	IsSummarize       *bool                    `json:"is_summarize,omitempty"`
	ExistingMemories  []string                 `json:"existing_memories,omitempty"`
	Language          *string                  `json:"language,omitempty"`
	HistoryDepth      *int                     `json:"history_depth,omitempty"`
	IsGroup           *bool                    `json:"is_group,omitempty"`
	UserPersona       *string                  `json:"user_persona,omitempty"`
	UserName          *string                  `json:"user_name,omitempty"`
	BotIDs            []string                 `json:"bot_ids,omitempty"`
	BotID             *string                  `json:"bot_id,omitempty"`
	ContextLimit      *int                     `json:"context_limit,omitempty"`
	Stream            *bool                    `json:"stream,omitempty"`
}

func enqueueProviderChat(c *gin.Context, req OpenAIProviderRequest, baseURL, provider, defaultModel string) {
	if req.APIKey == "" || len(req.Messages) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": "api_key and messages are required",
		})
		return
	}

	model := req.Model
	if model == "" {
		model = defaultModel
	}

	_ = llm.Default.Validate(provider, model, nil)

	// No VIP/Sensei block — public BYOK relay
	if err := byokSvc.SanitizeByokInput(&byokSvc.BYOKRequest{
		Messages:          req.Messages,
		APIKey:            req.APIKey,
		Model:             model,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		TopK:              req.TopK,
		MinP:              req.MinP,
		RepetitionPenalty: req.RepetitionPenalty,
		MaxTokens:         req.MaxTokens,
		Stop:              req.Stop,
		FrequencyPenalty:  req.FrequencyPenalty,
		PresencePenalty:   req.PresencePenalty,
		Reasoning:         req.Reasoning,
		ResponseLength:    req.ResponseLength,
		Summary:           req.Summary,
		IsSummarize:       req.IsSummarize,
		Language:          req.Language,
		HistoryDepth:      req.HistoryDepth,
		IsGroup:           req.IsGroup,
		UserPersona:       req.UserPersona,
		BotIDs:            req.BotIDs,
		ExistingMemories:  req.ExistingMemories,
		BotID:             req.BotID,
		ContextLimit:      req.ContextLimit,
		SourceIP:          c.ClientIP(),
	}); err != nil {
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

	userName := ""
	if req.UserName != nil {
		userName = strings.TrimSpace(*req.UserName)
	}
	if userName == "" {
		userName = "Sensei"
	}
	userPersona := ""
	if req.UserPersona != nil {
		userPersona = *req.UserPersona
	}

	payload := map[string]interface{}{
		"uuid":         uid,
		"status":       "queued",
		"api_key":      encryptedKey,
		"messages":     messagesB64,
		"base_url":     baseURL,
		"model":        model,
		"provider":     provider,
		"trace_id":     traceID,
		"user_name":    userName,
		"user_persona": userPersona,
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
	if req.ResponseLength != nil {
		payload["response_length"] = *req.ResponseLength
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
	if req.TopK != nil {
		payload["top_k"] = *req.TopK
	}
	if req.MinP != nil {
		payload["min_p"] = *req.MinP
	}
	if req.RepetitionPenalty != nil {
		payload["repetition_penalty"] = *req.RepetitionPenalty
	}
	if req.IncludeReasoning != nil {
		payload["include_reasoning"] = *req.IncludeReasoning
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
