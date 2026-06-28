package jobs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services"
	"rafiq8k-moga/halostory-go-backend/internal/services/ai"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	TypeBYOKChat = "ai:chat:byok"
)

type BYOKChatPayload struct {
	UUID     string `json:"uuid"`
	UserID   uint   `json:"user_id"`
	Provider string `json:"provider"`
}

func NewBYOKChatTask(uuid string, userID uint, provider string) (*asynq.Task, error) {
	payload, err := json.Marshal(BYOKChatPayload{
		UUID:     uuid,
		UserID:   userID,
		Provider: provider,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeBYOKChat, payload, asynq.MaxRetry(3), asynq.Timeout(180)), nil
}

type BYOKChatProcessor struct {
	db     *gorm.DB
	cfg    *config.Config
	rdb    *redis.Client
	crypto *services.CryptoService
}

func NewBYOKChatProcessor(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *BYOKChatProcessor {
	return &BYOKChatProcessor{
		db:     db,
		cfg:    cfg,
		rdb:    rdb,
		crypto: services.NewCryptoService(cfg.JWTSecret),
	}
}

func (p *BYOKChatProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload BYOKChatPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	chatKey := fmt.Sprintf("halostory:chat:%s", payload.UUID)
	jobDataStr, err := p.rdb.Get(ctx, chatKey).Result()
	if err != nil {
		return fmt.Errorf("job data not found: %w", err)
	}

	var jobData map[string]interface{}
	if err := json.Unmarshal([]byte(jobDataStr), &jobData); err != nil {
		return fmt.Errorf("failed to parse job data: %w", err)
	}

	jobData["status"] = "processing"
	p.rdb.Set(ctx, chatKey, toJSON(jobData), 0)
	p.rdb.Expire(ctx, chatKey, 600)

	// Decrypt API key
	encryptedKey, _ := jobData["api_key"].(string)
	apiKey, err := p.crypto.Decrypt(encryptedKey)
	if err != nil {
		return p.markError(ctx, chatKey, "failed to decrypt API key")
	}

	// Decode messages
	messagesB64, _ := jobData["messages"].(string)
	messagesJSON, err := base64.StdEncoding.DecodeString(messagesB64)
	if err != nil {
		return p.markError(ctx, chatKey, "failed to decode messages")
	}

	var messages []ai.Message
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		return p.markError(ctx, chatKey, "invalid messages format")
	}

	provider, _ := jobData["provider"].(string)
	model, _ := jobData["model"].(string)

	// Get provider config
	providerCfg, exists := ai.BYOKProviders[provider]
	if !exists {
		return p.markError(ctx, chatKey, fmt.Sprintf("unknown provider: %s", provider))
	}

	// Build provider-specific payload
	providerReq := ai.BYOKRequest{
		Messages: messages,
		APIKey:   apiKey,
		Model:    model,
	}

	if v, ok := jobData["temperature"].(float64); ok {
		providerReq.Temperature = &v
	}
	if v, ok := jobData["max_tokens"].(float64); ok {
		iv := int(v)
		providerReq.MaxTokens = &iv
	}
	if v, ok := jobData["top_p"].(float64); ok {
		providerReq.TopP = &v
	}
	if v, ok := jobData["base_url"].(string); ok {
		providerReq.BaseURL = &v
	}

	baseURL := providerCfg.BaseURL
	if providerReq.BaseURL != nil && *providerReq.BaseURL != "" {
		if provider == "openai-compatible" {
			baseURL = *providerReq.BaseURL
		}
	}

	providerReqMsg, _ := json.Marshal(providerReq)

	// Inject system prompt if bot_id is present
	botID, _ := jobData["bot_id"].(string)
	if botID != "" {
		var bot models.Bot
		if err := p.db.Where("bot_id = ?", botID).First(&bot).Error; err == nil {
			language, _ := jobData["language"].(string)
			summary, _ := jobData["summary"].(string)
			userPersona, _ := jobData["user_persona"].(string)
			isDisableBond, _ := jobData["isdisablebond"].(bool)
			depth := 0
			if d, ok := jobData["history_depth"].(float64); ok {
				depth = int(d)
			}

			var bond models.UserBotBond
			hasBond := p.db.Where("user_id = ? AND bot_id = ?", payload.UserID, botID).First(&bond).Error == nil
			var bondPtr *models.UserBotBond
			if hasBond && !isDisableBond {
				bondPtr = &bond
			}

			messages = ai.InjectBYOKSystemPrompt(messages, botID, language, depth, 0, summary, payload.UserID, isDisableBond, userPersona, &bot, bondPtr)
		}
	}

	// Build payload and call provider
	buildPayload := providerCfg.BuildPayload
	if buildPayload == nil {
		buildPayload = defaultBuildPayload
	}

	apiPayload := buildPayload(providerReq, string(providerReqMsg))
	headers := map[string]string{
		"Content-Type": "application/json",
		"Authorization": providerCfg.AuthHeader(apiKey),
	}

	// For OpenRouter add headers
	if provider == "openrouter" {
		headers["HTTP-Referer"] = p.cfg.AppURL
		headers["X-Title"] = p.cfg.AppName
	}

	// Call the provider API (non-streaming for BYOK)
	result, err := callProviderAPI(baseURL, headers, apiPayload, model, int(p.cfg.AIRequestTimeout.Seconds()))
	if err != nil {
		return p.markError(ctx, chatKey, fmt.Sprintf("provider call failed: %v", err))
	}

	content, _ := result["content"].(string)
	content = ai.ProcessAIResponse(content)

	// Parse bond tags
	parsed := ai.ParseBondTags(content)
	content = parsed.CleanText

	// Handle child safety if bot is present
	if botID != "" {
		var bot models.Bot
		if err := p.db.Where("bot_id = ?", botID).First(&bot).Error; err == nil && bot.Child {
			if containsTag(parsed.Tags, "failed_nsfw") || containsTag(parsed.Tags, "nsfw_bypassed") {
				// Child safety alert — log for now, full integration in Phase 4
				fmt.Printf("CHILD SAFETY BYOK: bot=%s user=%d tags=%v\n", botID, payload.UserID, parsed.Tags)
			}
		}
	}

	// Add bond XP
	var bondEvent interface{}
	if botID != "" {
		var bond models.UserBotBond
		hasBond := p.db.Where("user_id = ? AND bot_id = ?", payload.UserID, botID).First(&bond).Error == nil
		isDisableBond, _ := jobData["isdisablebond"].(bool)
		if hasBond && !isDisableBond {
			var bot models.Bot
			p.db.Where("bot_id = ?", botID).First(&bot)

			bondResult := ai.AddBondXP(&bot, parsed, &bond)
			if bondResult != nil {
				bondEvent = bondResult.BondEvent
				p.db.Save(&bond)
			}
		}
	}

	response := map[string]interface{}{
		"status":     "completed",
		"content":    content,
		"model":      model,
		"bond_event": bondEvent,
		"http_status": 200,
	}

	p.rdb.Set(ctx, chatKey, toJSON(response), 0)
	p.rdb.Expire(ctx, chatKey, 600)

	return nil
}

func (p *BYOKChatProcessor) markError(ctx context.Context, chatKey string, errMsg string) error {
	response := map[string]interface{}{
		"status":      "error",
		"error":       errMsg,
		"http_status": 500,
	}
	p.rdb.Set(ctx, chatKey, toJSON(response), 0)
	p.rdb.Expire(ctx, chatKey, 600)
	return fmt.Errorf("%s", errMsg)
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func defaultBuildPayload(req ai.BYOKRequest, _ string) map[string]interface{} {
	messages := make([]map[string]interface{}, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}

	payload := map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
		"stream":   false,
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

	return payload
}

func callProviderAPI(url string, headers map[string]string, payload map[string]interface{}, model string, timeout int) (map[string]interface{}, error) {
	// Use net/http for the call
	payloadBytes, _ := json.Marshal(payload)

	client := services.NewHTTPClient(timeout)

	resp, err := client.Post(url, headers, payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("api call failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract content from OpenAI-style response
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					return map[string]interface{}{
						"content": content,
						"model":   model,
					}, nil
				}
			}
		}
	}

	// Anthropic-style response
	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		if block, ok := content[0].(map[string]interface{}); ok {
			if text, ok := block["text"].(string); ok {
				return map[string]interface{}{
					"content": text,
					"model":   model,
				}, nil
			}
		}
	}

	// Fallback: return full response text if available
	if text, ok := result["content"].(string); ok {
		return map[string]interface{}{
			"content": text,
			"model":   model,
		}, nil
	}

	return map[string]interface{}{
		"content": "[No response content]",
		"model":   model,
	}, nil
}
