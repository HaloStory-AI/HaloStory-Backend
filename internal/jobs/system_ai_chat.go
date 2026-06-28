package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services"
	"rafiq8k-moga/halostory-go-backend/internal/services/ai"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	TypeSystemAIChat = "ai:chat:system"
)

type SystemAIChatPayload struct {
	UUID   string `json:"uuid"`
	UserID uint   `json:"user_id"`
	BotID  string `json:"bot_id"`
	IsGroup bool  `json:"is_group"`
}

func NewSystemAIChatTask(uuid string, userID uint, botID string, isGroup bool) (*asynq.Task, error) {
	payload, err := json.Marshal(SystemAIChatPayload{
		UUID:    uuid,
		UserID:  userID,
		BotID:   botID,
		IsGroup: isGroup,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSystemAIChat, payload, asynq.MaxRetry(3), asynq.Timeout(240)), nil
}

type SystemAIChatProcessor struct {
	db     *gorm.DB
	cfg    *config.Config
	rdb    *redis.Client
	fallback *ai.FallbackChainService
	cache  *services.CacheService
}

func NewSystemAIChatProcessor(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *SystemAIChatProcessor {
	cache := services.NewCacheService(rdb)
	return &SystemAIChatProcessor{
		db:       db,
		cfg:      cfg,
		rdb:      rdb,
		fallback: ai.NewFallbackChainService(cfg, cache),
		cache:    cache,
	}
}

func (p *SystemAIChatProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload SystemAIChatPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Read job data from Redis
	chatKey := fmt.Sprintf("halostory:chat:%s", payload.UUID)
	jobData, err := p.rdb.Get(ctx, chatKey).Result()
	if err != nil {
		return fmt.Errorf("job data not found: %w", err)
	}

	var jobDataMap map[string]interface{}
	if err := json.Unmarshal([]byte(jobData), &jobDataMap); err != nil {
		return fmt.Errorf("failed to parse job data: %w", err)
	}

	jobDataMap["status"] = "processing"
	p.rdb.Set(ctx, chatKey, toJSON(jobDataMap), 0)
	p.rdb.Expire(ctx, chatKey, 600)

	// Create UserActivity
	p.db.Create(&models.UserActivity{UserID: payload.UserID})

	isSummarize, _ := jobDataMap["is_summarize"].(bool)
	isPriority, _ := jobDataMap["is_priority"].(bool)

	if isSummarize {
		return p.handleSummarization(ctx, payload, jobDataMap, chatKey, isPriority)
	}

	return p.handleChat(ctx, payload, jobDataMap, chatKey, isPriority)
}

func (p *SystemAIChatProcessor) handleChat(ctx context.Context, payload SystemAIChatPayload, jobData map[string]interface{}, chatKey string, isPriority bool) error {
	messagesEncoded, _ := jobData["messages"].(string)

	var messages []ai.Message
	if err := json.Unmarshal([]byte(messagesEncoded), &messages); err != nil {
		return p.markError(ctx, chatKey, "invalid messages")
	}

	// Decode base64 messages
	if decoded, err := decodeBase64JSON(messagesEncoded); err == nil {
		messages = decoded
	}

	// Load bot
	var bot models.Bot
	if err := p.db.Where("bot_id = ?", payload.BotID).First(&bot).Error; err != nil {
		return p.markError(ctx, chatKey, "bot not found")
	}

	// Load bond
	var bond models.UserBotBond
	bondResult := p.db.Where("user_id = ? AND bot_id = ?", payload.UserID, payload.BotID).First(&bond)
	hasBond := bondResult.Error == nil

	// Check disable bond
	isDisableBond, _ := jobData["isdisablebond"].(bool)
	language, _ := jobData["language"].(string)
	summary, _ := jobData["summary"].(string)
	userPersona, _ := jobData["user_persona"].(string)

	systemPrompt := ai.FormatGlobalPrompt(
		messages,
		&bot,
		mapBond(&bond, hasBond, isDisableBond),
		language,
		true,
		false,
		summary,
		payload.UserID,
		userPersona,
	)

	// Handle group context
	isGroup, _ := jobData["is_group"].(bool)
	if isGroup {
		botIDs, _ := jobData["bot_ids"].([]string)
		systemPrompt += "\n" + ai.BuildGroupContextString(botIDs, payload.BotID, language, nil)
	}

	// Build prompt from messages
	userPrompt := ""
	for _, msg := range messages {
		if msg.Role == "user" {
			userPrompt = msg.Content
		}
	}

	// Get fallback chain
	var chain []ai.FallbackEntry
	if isPriority {
		chain = p.fallback.GetPriorityFallbackChain()
	} else {
		chain = p.fallback.GetFallbackChain()
	}

	opts := map[string]interface{}{}
	if mt, ok := jobData["max_tokens"]; ok {
		opts["max_tokens"] = mt
	}
	if temp, ok := jobData["temperature"]; ok {
		opts["temperature"] = temp
	}
	if topP, ok := jobData["top_p"]; ok {
		opts["top_p"] = topP
	}

	result, model, err := p.fallback.SendChat(userPrompt, systemPrompt, chain, opts, isPriority)
	if err != nil {
		return p.markError(ctx, chatKey, fmt.Sprintf("all providers failed: %v", err))
	}

	content, _ := result["content"].(string)
	content = ai.ProcessAIResponse(content)

	// Parse bond tags
	parsed := ai.ParseBondTags(content)
	content = parsed.CleanText

	// Handle child safety
	if bot.Child && (contains(parsed.Tags, "failed_nsfw") || contains(parsed.Tags, "nsfw_bypassed")) {
		log.Printf("CHILD SAFETY: bot=%s user=%d tags=%v", payload.BotID, payload.UserID, parsed.Tags)
	}

	// Add bond XP
	var bondEvent interface{}
	var bondStatus interface{}
	if hasBond && !isDisableBond {
		bondEventResult := ai.AddBondXP(&bot, parsed, &bond)
		if bondEventResult != nil {
			bondEvent = bondEventResult.BondEvent
			bondStatus = bondEventResult.BondStatus
			p.db.Save(&bond)
		}
	}

	response := map[string]interface{}{
		"status":       "completed",
		"content":      content,
		"model":        model,
		"bond_event":   bondEvent,
		"bond_status":  bondStatus,
		"http_status":  200,
	}

	p.rdb.Set(ctx, chatKey, toJSON(response), 0)
	p.rdb.Expire(ctx, chatKey, 600)

	return nil
}

func (p *SystemAIChatProcessor) handleSummarization(ctx context.Context, payload SystemAIChatPayload, jobData map[string]interface{}, chatKey string, isPriority bool) error {
	summarizationPrompt, _ := jobData["summarization_prompt"].(string)
	if summarizationPrompt == "" {
		return p.markError(ctx, chatKey, "no summarization prompt")
	}

	summary, err := p.fallback.BatchSummarize(summarizationPrompt, isPriority)
	if err != nil {
		return p.markError(ctx, chatKey, fmt.Sprintf("summarization failed: %v", err))
	}

	response := map[string]interface{}{
		"status":      "completed",
		"summary":     summary,
		"http_status": 200,
	}

	p.rdb.Set(ctx, chatKey, toJSON(response), 0)
	p.rdb.Expire(ctx, chatKey, 600)

	return nil
}

func (p *SystemAIChatProcessor) markError(ctx context.Context, chatKey string, errMsg string) error {
	response := map[string]interface{}{
		"status":      "error",
		"error":       errMsg,
		"http_status": 500,
	}
	p.rdb.Set(ctx, chatKey, toJSON(response), 0)
	p.rdb.Expire(ctx, chatKey, 600)
	return fmt.Errorf("%s", errMsg)
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func decodeBase64JSON(encoded string) ([]ai.Message, error) {
	// Messages stored as JSON string, then base64-encoded in Laravel
	// In Go, try direct decode first, then fallback to base64
	var messages []ai.Message
	if err := json.Unmarshal([]byte(encoded), &messages); err == nil {
		return messages, nil
	}
	return nil, fmt.Errorf("cannot decode messages")
}

func mapBond(bond *models.UserBotBond, hasBond bool, isDisableBond bool) *models.UserBotBond {
	if !hasBond || isDisableBond {
		return nil
	}
	return bond
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
