package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services/observability"
	"halostory_go_backend/internal/services/systemai"
	"halostory_go_backend/internal/utils"
)

type ChatJobPayload struct {
	UUID    string `json:"uuid"`
	UserID  uint   `json:"user_id"`
	BotID   string `json:"bot_id"`
	IsGroup bool   `json:"is_group"`
}

type chatData struct {
	Status           string           `json:"status"`
	APIKey           string           `json:"api_key"`
	Messages         string           `json:"messages"`
	Model            string           `json:"model"`
	Provider         string           `json:"provider"`
	BaseURL          string           `json:"base_url,omitempty"`
	UserID           float64          `json:"user_id"`
	BotID            string           `json:"bot_id"`
	Language         string           `json:"language,omitempty"`
	Temperature      float64          `json:"temperature,omitempty"`
	MaxTokens        float64          `json:"max_tokens,omitempty"`
	TopP             float64          `json:"top_p,omitempty"`
	TopK             float64          `json:"top_k,omitempty"`
	PresencePenalty  float64          `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64          `json:"frequency_penalty,omitempty"`
	Summary          string           `json:"summary,omitempty"`
	IsSummarize      bool             `json:"is_summarize,omitempty"`
	HistoryDepth     float64          `json:"history_depth,omitempty"`
	IsGroup          bool             `json:"is_group,omitempty"`
	IsDisableBond    bool             `json:"isdisablebond,omitempty"`
	UserPersona      string           `json:"user_persona,omitempty"`
	BotIDs           []string         `json:"bot_ids,omitempty"`
	ExistingMemories []string         `json:"existing_memories,omitempty"`
	Reasoning        *reasoningConfig `json:"reasoning,omitempty"`
	Stop             []string         `json:"stop,omitempty"`
	ResponseLength   float64          `json:"response_length,omitempty"`
	TraceID          string           `json:"trace_id,omitempty"`
}

type reasoningConfig struct {
	Enabled   bool    `json:"enabled"`
	Effort    string  `json:"effort,omitempty"`
	MaxTokens float64 `json:"max_tokens,omitempty"`
	Exclude   bool    `json:"exclude,omitempty"`
}

var httpClient = utils.NewAIHttpClient()

func isSystemAIJob(data []byte) bool {
	var p struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return false
	}
	return p.Provider == "system-ai"
}

func jobIsPriority(data []byte) bool {
	var p struct {
		IsPriority bool `json:"is_priority"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return false
	}
	return p.IsPriority
}

func dispatchJob(data []byte, sem chan struct{}, goroutineName string) {
	sem <- struct{}{}
	go func(d []byte) {
		var jobUUID string
		var jobTraceID string
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Worker: RECOVERED panic in %s goroutine: %v", goroutineName, r)
				observability.Error("worker.panic", "goroutine", goroutineName, "error", fmt.Sprintf("%v", r), "uuid", jobUUID, "trace_id", jobTraceID)
				if jobUUID != "" {
					markError(jobUUID, "chat:"+jobUUID, "Internal worker panic: "+fmt.Sprintf("%v", r), 500)
				}
			}
			<-sem
		}()
		// Pre-extract uuid/trace_id for panic recovery
		var rawPayload map[string]interface{}
		if json.Unmarshal(d, &rawPayload) == nil {
			if u, ok := rawPayload["uuid"].(string); ok {
				jobUUID = u
			}
			if tid, ok := rawPayload["trace_id"].(string); ok {
				jobTraceID = tid
			}
		}
		processJob(d)
	}(data)
}

func main() {
	config.LoadConfig()
	observability.InitLogger()
	database.Connect()
	services.InitRedis()

	// maxWorkersBYOK := 30
	maxWorkersBYOK := 100
	maxWorkersSystem := 30
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxWorkersBYOK = n
		}
	}
	mainSem := make(chan struct{}, maxWorkersBYOK)
	sysSem := make(chan struct{}, maxWorkersSystem)
	log.Printf("Worker service starting — BYOK concurrency: %d, System AI concurrency: %d (free), VIP routed to BYOK pool", maxWorkersBYOK, maxWorkersSystem)

	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		})
		log.Printf("Worker health check listening on :3001")
		if err := http.ListenAndServe(":3001", nil); err != nil {
			log.Printf("Worker health check server error: %v", err)
		}
	}()

	go func() {
		backoff := 100 * time.Millisecond
		for {
			_, data, err := services.Dequeue("ai-chat", 5*time.Second)
			if err != nil {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > 10*time.Second {
					backoff = 10 * time.Second
				}
				continue
			}
			backoff = 100 * time.Millisecond

			if isSystemAIJob(data) {
				if jobIsPriority(data) {
					dispatchJob(data, mainSem, "main")
				} else {
					dispatchJob(data, sysSem, "sys")
				}
			} else {
				dispatchJob(data, mainSem, "main")
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Worker service shutting down...")
}

func processJob(rawData []byte) {
	start := time.Now()
	var payload map[string]interface{}
	if err := json.Unmarshal(rawData, &payload); err != nil {
		log.Printf("Worker: Failed to parse job payload: %v", err)
		return
	}

	uuid, _ := payload["uuid"].(string)
	traceID, _ := payload["trace_id"].(string)
	if uuid == "" {
		log.Printf("Worker: Missing uuid in payload")
		return
	}
	if traceID != "" && observability.ShouldSample() {
		observability.Info("worker.job.start", "trace_id", traceID, "uuid", uuid, "provider", payload["provider"])
	}

	storeKey := "chat:" + uuid

	rawJSON, _ := json.Marshal(payload)
	if err := services.StoreChatResult(uuid, rawJSON, 10*time.Minute); err != nil {
		log.Printf("Worker: Failed to store initial payload for %s: %v", uuid, err)
	}

	var data chatData
	jsonBytes, _ := json.Marshal(payload)
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		log.Printf("Worker: Failed to parse chat data for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to parse chat data", 500)
		return
	}

	data.Status = "processing"
	updateJSON, _ := json.Marshal(data)
	services.StoreChatResult(uuid, updateJSON, 10*time.Minute)

	if data.UserID > 0 {
		userID := uint(data.UserID)
		database.DB.Create(&models.UserActivity{UserID: userID})
	}

	switch data.Provider {
	case "mock":
		processMockChat(uuid, storeKey, data)
	case "openai-compatible", "mistral", "deepseek", "openrouter", "zai":
		processOpenAIChat(uuid, storeKey, data)
	case "claude":
		processClaudeChat(uuid, storeKey, data)
	case "gemini":
		processGeminiChat(uuid, storeKey, data)
	case "system-ai":
		processSystemAIChat(uuid, storeKey, data)
	default:
		log.Printf("Worker: Unsupported provider %s for UUID %s", data.Provider, uuid)
		markError(uuid, storeKey, "Unsupported provider: "+data.Provider, 400)
	}

	latency := time.Since(start)
	observability.ObserveLatency(data.Provider, latency)
	observability.IncRequest(data.Provider, data.Model, "completed")
	if traceID != "" {
		observability.Info("worker.job.done", "trace_id", traceID, "uuid", uuid, "provider", data.Provider, "model", data.Model, "latency_ms", latency.Milliseconds())
	}
}

func processGeminiChat(uuid, storeKey string, data chatData) {
	decryptedKey, err := utils.DecryptBYOKAPIKey(data.APIKey)
	if err != nil {
		log.Printf("Worker: Failed to decrypt API key for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to decrypt API key", 500)
		return
	}

	messagesJSON, err := base64.StdEncoding.DecodeString(data.Messages)
	if err != nil {
		log.Printf("Worker: Failed to decode messages for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to decode messages", 400)
		return
	}

	var messages []byok.Message
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		log.Printf("Worker: Failed to parse messages for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to parse messages", 400)
		return
	}

	userID := uint(data.UserID)
	botID := data.BotID
	lang := data.Language
	if lang == "" {
		lang = "id"
	}
	historyDepth := int(data.HistoryDepth)
	if historyDepth == 0 {
		historyDepth = 15
	}
	maxTokens := int(data.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 2048
	}
	var contextLimit *int
	if maxTokens > 0 {
		contextLimit = &maxTokens
	}
	var summary *string
	if data.Summary != "" {
		summary = &data.Summary
	}
	var userPersona *string
	if data.UserPersona != "" {
		userPersona = &data.UserPersona
	}

	// WithoutOverlapping lock
	if userID > 0 {
		lockKey := fmt.Sprintf("ai-chat-lock:%d:%s", userID, botID)
		if data.IsGroup {
			lockKey = fmt.Sprintf("ai-chat-lock:%d:group", userID)
		}
		locked, err := services.RedisClient.SetNX(services.RedisCtx, lockKey, "1", 120*time.Second).Result()
		if err != nil {
			log.Printf("Worker: Lock check failed for %s: %v", uuid, err)
		}
		if err == nil && !locked {
			log.Printf("Worker: Job %s skipped — overlapping lock held for %s", uuid, lockKey)
			return
		}
		if err == nil && locked {
			defer services.RedisClient.Del(services.RedisCtx, lockKey)
		}
	}

	if data.IsSummarize {
		handleGeminiSummarization(uuid, storeKey, data, decryptedKey, messagesJSON)
		return
	}

	messages = byok.InjectByokSystemPrompt(
		messages, &botID, &lang, &historyDepth, contextLimit,
		summary, &userID, data.IsDisableBond, userPersona,
	)

	if data.IsGroup && len(data.BotIDs) > 1 {
		primaryBotID := botID
		messages = byok.InjectGroupContext(
			messages, data.BotIDs, &primaryBotID, lang, &userID,
		)
	}

	contents := make([]byok.GeminiContent, 0)
	var systemInstruct *byok.GeminiContent
	systemText := ""
	isGemma := strings.Contains(data.Model, "gemma")
	isModernGemma := false
	if isGemma && len(data.Model) >= 7 {
		v := data.Model[6]
		isModernGemma = v >= '2' && v <= '9'
	}
	supportsSystemInstruction := !isGemma || isModernGemma

	for _, msg := range messages {
		if msg.Role == "system" {
			if !supportsSystemInstruction {
				systemText = msg.Content
			} else {
				systemInstruct = &byok.GeminiContent{
					Parts: []byok.GeminiPart{{Text: msg.Content}},
				}
			}
			continue
		}

		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}

		text := msg.Content
		if !supportsSystemInstruction && len(contents) == 0 && role == "user" && systemText != "" {
			text = systemText + "\n\n" + text
		}

		contents = append(contents, byok.GeminiContent{
			Role:  role,
			Parts: []byok.GeminiPart{{Text: text}},
		})
	}

	if !supportsSystemInstruction && len(contents) == 0 && systemText != "" {
		contents = append(contents, byok.GeminiContent{
			Role:  "user",
			Parts: []byok.GeminiPart{{Text: systemText}},
		})
	}

	temp := 0.7
	if data.Temperature != 0 {
		temp = data.Temperature
	}
	topP := 1.0
	if data.TopP != 0 {
		topP = data.TopP
	}
	topK := 40
	if data.TopK != 0 {
		topK = int(data.TopK)
	}
	maxOut := 1000
	if data.ResponseLength != 0 {
		maxOut = int(data.ResponseLength)
	} else if data.MaxTokens != 0 {
		maxOut = int(data.MaxTokens)
	}
	presPenalty := 0.0
	if data.PresencePenalty != 0 {
		presPenalty = data.PresencePenalty
	}
	freqPenalty := 0.0
	if data.FrequencyPenalty != 0 {
		freqPenalty = data.FrequencyPenalty
	}

	genConfig := byok.GenerationConfig{
		Temperature:      temp,
		TopP:             topP,
		TopK:             topK,
		MaxOutputTokens:  maxOut,
		StopSequences:    data.Stop,
		PresencePenalty:  presPenalty,
		FrequencyPenalty: freqPenalty,
	}

	if data.Reasoning != nil && data.Reasoning.Enabled {
		effort := data.Reasoning.Effort
		if effort == "" {
			effort = "low"
		}
		maxOutTokens := int(data.MaxTokens)
		if maxOutTokens == 0 {
			maxOutTokens = 2048
		}
		var thinkingBudget int
		switch effort {
		case "high":
			thinkingBudget = int(float64(maxOutTokens) * 0.8)
		case "medium":
			thinkingBudget = int(float64(maxOutTokens) * 0.5)
		case "custom":
			thinkingBudget = int(data.Reasoning.MaxTokens)
			if thinkingBudget == 0 {
				thinkingBudget = 1024
			}
		default:
			thinkingBudget = int(float64(maxOutTokens) * 0.2)
		}

		includeThoughts := !data.Reasoning.Exclude
		genConfig.ThinkingConfig = &byok.ThinkingConfig{
			IncludeThoughts: &includeThoughts,
			ThinkingBudget:  &thinkingBudget,
		}
	}

	if isGemma {
		if genConfig.ThinkingConfig == nil {
			genConfig.ThinkingConfig = &byok.ThinkingConfig{}
		}
		genConfig.ThinkingConfig.ThinkingLevel = "MINIMAL"
	}

	req := byok.GeminiRequest{
		Contents:         contents,
		GenerationConfig: genConfig,
		SafetySettings:   byok.DefaultSafetySettings,
	}

	if systemInstruct != nil {
		req.SystemInstruction = systemInstruct
	}

	geminiResp, statusCode, err := byok.CallGemini(decryptedKey, data.Model, req)
	if err != nil {
		log.Printf("Worker: Gemini API error for %s: %v (status %d)", uuid, err, statusCode)
		markError(uuid, storeKey, "[GoogleAI] response failed: "+err.Error(), statusCode)
		return
	}

	if geminiResp.PromptFeedback != nil {
		blocked := geminiResp.PromptFeedback.BlockReasonMessage
		if blocked == "" {
			blocked = geminiResp.PromptFeedback.BlockReason
		}
		if blocked != "" {
			log.Printf("Worker: Prompt blocked for %s: %s", uuid, blocked)
			openaiResp := byok.ConvertToOpenAIFormat(geminiResp, data.Model)
			openaiResp.Choices[0].Message.Content = "I'm sorry, tapi prompt ini diblokir oleh sensor Google... (" + blocked + ")"
			openaiResp.Choices[0].FinishReason = "content_filter"
			// Match Laravel's blocked response structure: error + choices
			respBytes, _ := json.Marshal(openaiResp)
			var respMap map[string]interface{}
			json.Unmarshal(respBytes, &respMap)
			respMap["error"] = "[GoogleAI] Prompt was blocked: " + blocked
			markCompleted(uuid, storeKey, respMap, 400)
			return
		}
	}

	openaiResp := byok.ConvertToOpenAIFormat(geminiResp, data.Model)

	if len(openaiResp.Choices) > 0 && openaiResp.Choices[0].Message.Content != "" {
		content := openaiResp.Choices[0].Message.Content
		content = byok.ProcessAIResponse(content)
		parsed := byok.ParseBondTags(content)
		openaiResp.Choices[0].Message.Content = parsed.CleanText

		var bot models.Bot
		database.DB.Where("bot_id = ?", data.BotID).First(&bot)

		var user models.User
		database.DB.First(&user, uint(data.UserID))

		if bot.BotID != "" && user.ID != 0 {
			messagesRaw, _ := base64.StdEncoding.DecodeString(data.Messages)
			var originalMessages []byok.Message
			_ = json.Unmarshal(messagesRaw, &originalMessages)

			lastUserMessage := ""
			for i := len(originalMessages) - 1; i >= 0; i-- {
				if originalMessages[i].Role == "user" {
					lastUserMessage = originalMessages[i].Content
					break
				}
			}

			byok.HandleChildSafety(&bot, &user, lastUserMessage, content, parsed.Tags)
		}

		// Convert to map for bond field injection
		respBytes, _ := json.Marshal(openaiResp)
		var respMap map[string]interface{}
		json.Unmarshal(respBytes, &respMap)

		if !data.IsGroup && !data.IsDisableBond && data.BotID != "" {
			bondResult := byok.AddBondXp(data.BotID, parsed, uint(data.UserID))
			respMap["bond_event"] = bondResult.BondEvent
			respMap["bond_status"] = bondResult.BondStatus
			respMap["relationship_label"] = bondResult.RelationshipLabel
			respMap["bond_level"] = bondResult.BondLevel
		} else {
			respMap["bond_event"] = nil
			respMap["bond_status"] = nil
			respMap["relationship_label"] = nil
			respMap["bond_level"] = nil
		}

		markCompleted(uuid, storeKey, respMap, 200)
		return
	}

	markCompleted(uuid, storeKey, openaiResp, 200)
}

func markCompleted(uuid, storeKey string, response interface{}, statusCode int) {
	respJSON, _ := json.Marshal(response)
	services.MarkChatCompleted(uuid, json.RawMessage(respJSON), statusCode)
	log.Printf("Worker: Completed job %s with status %d", uuid, statusCode)
}

func processSystemAIChat(uuid, storeKey string, data chatData) {
	messagesJSON, err := base64.StdEncoding.DecodeString(data.Messages)
	if err != nil {
		log.Printf("Worker: Failed to decode messages for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to decode messages", 400)
		return
	}

	var messages []byok.Message
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		log.Printf("Worker: Failed to parse messages for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to parse messages", 400)
		return
	}

	userID := uint(data.UserID)
	botID := data.BotID

	// WithoutOverlapping lock
	lockKey := fmt.Sprintf("ai-chat-lock:%d:%s", userID, botID)
	if data.IsGroup {
		lockKey = fmt.Sprintf("ai-chat-lock:%d:group", userID)
	}
	locked, err := services.RedisClient.SetNX(services.RedisCtx, lockKey, "1", 120*time.Second).Result()
	if err != nil {
		log.Printf("Worker: Lock check failed for %s: %v", uuid, err)
	}
	if err == nil && !locked {
		log.Printf("Worker: Job %s skipped — overlapping lock held for %s", uuid, lockKey)
		return
	}
	if err == nil && locked {
		defer services.RedisClient.Del(services.RedisCtx, lockKey)
	}

	lang := data.Language
	if lang == "" {
		lang = systemai.DefaultLanguage
	}

	// === Summarization path ===
	if data.IsSummarize {
		handleSystemAISummarization(uuid, storeKey, data, messages, userID)
		return
	}

	// === Chat path ===
	historyDepth := int(data.HistoryDepth)
	if historyDepth == 0 {
		historyDepth = systemai.DefaultHistoryDepth
	}
	maxTokens := int(data.MaxTokens)
	if maxTokens == 0 {
		maxTokens = systemai.DefaultMaxTokens
	}

	isVip := false
	var user models.User
	if userID > 0 {
		database.DB.First(&user, userID)
		isVip = user.Role == "vip"
	}

	params := &systemai.SystemAIChatParams{
		BotID:          botID,
		UserID:         userID,
		Language:       lang,
		Temperature:    data.Temperature,
		MaxTokens:      int(data.MaxTokens),
		TopP:           data.TopP,
		TopK:           int(data.TopK),
		ResponseLength: int(data.ResponseLength),
		Stop:           data.Stop,
		Summary:        data.Summary,
		IsSummarize:    false,
		HistoryDepth:   historyDepth,
		IsGroup:        data.IsGroup,
		IsDisableBond:  data.IsDisableBond,
		UserPersona:    data.UserPersona,
		BotIDs:         data.BotIDs,
	}

	var summary *string
	if data.Summary != "" {
		summary = &data.Summary
	}
	var userPersona *string
	if data.UserPersona != "" {
		userPersona = &data.UserPersona
	}

	messages = byok.InjectByokSystemPrompt(
		messages, &botID, &lang, &historyDepth, &maxTokens,
		summary, &userID, data.IsDisableBond, userPersona,
	)

	if data.IsGroup && len(data.BotIDs) > 1 {
		primaryBotID := botID
		messages = byok.InjectGroupContext(
			messages, data.BotIDs, &primaryBotID, lang, &userID,
		)
	}

	start := time.Now()
	result := systemai.RunFallbackChain(httpClient, messages, params, isVip)
	log.Printf("Worker: System AI chain for %s took %v, success=%v", uuid, time.Since(start), result.Error == "")

	if result.Error != "" {
		pingsJSON, _ := json.Marshal(result.Pings)
		log.Printf("Worker: System AI all failed for %s: %s | pings: %s", uuid, result.Error, string(pingsJSON))
		markError(uuid, storeKey, result.Error, result.StatusCode)
		return
	}

	content := result.Content
	content = byok.ProcessAIResponse(content)
	parsed := byok.ParseBondTags(content)

	updateResponseContent(result.Response, parsed.CleanText)

	var bot models.Bot
	database.DB.Where("bot_id = ?", data.BotID).First(&bot)

	if bot.BotID != "" && user.ID != 0 {
		var originalMessages []byok.Message
		json.Unmarshal(messagesJSON, &originalMessages)

		lastUserMessage := ""
		for i := len(originalMessages) - 1; i >= 0; i-- {
			if originalMessages[i].Role == "user" {
				lastUserMessage = originalMessages[i].Content
				break
			}
		}

		byok.HandleChildSafety(&bot, &user, lastUserMessage, content, parsed.Tags)
	}

	bondEvent := ""
	bondStatus := ""
	relationshipLabel := ""
	bondLevel := 0
	if !data.IsGroup && !data.IsDisableBond && bot.BotID != "" {
		bondResult := byok.AddBondXp(data.BotID, parsed, uint(data.UserID))
		bondEvent = bondResult.BondEvent
		bondStatus = bondResult.BondStatus
		relationshipLabel = bondResult.RelationshipLabel
		bondLevel = bondResult.BondLevel
	}

	markSystemAICompleted(uuid, storeKey, result.Response, 200, bondEvent, bondStatus, relationshipLabel, bondLevel, userID)
}

func handleSystemAISummarization(uuid, storeKey string, data chatData, messages []byok.Message, userID uint) {
	lang := data.Language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	prompt := byok.BuildSummarizationPrompt(messages, lang, data.ExistingMemories)

	promptMessages := []byok.Message{{Role: "user", Content: prompt}}

	isVip := false
	var user models.User
	if userID > 0 {
		database.DB.First(&user, userID)
		isVip = user.Role == "vip"
	}

	params := &systemai.SystemAIChatParams{
		Language:    lang,
		IsSummarize: true,
	}

	start := time.Now()
	result := systemai.RunFallbackChain(httpClient, promptMessages, params, isVip)
	log.Printf("Worker: System AI summarization for %s took %v, success=%v", uuid, time.Since(start), result.Error == "")

	if result.Error != "" {
		log.Printf("Worker: System AI summarization all failed for %s: %s", uuid, result.Error)
		markSummaryCompleted(uuid, storeKey, "[ERROR 503] All system AI models failed", []string{}, 503, true)
		return
	}

	rawContent := strings.TrimSpace(result.Content)

	summary, memories := byok.ParseSummaryResponse(rawContent)

	if summary == "" && rawContent != "" {
		log.Printf("Worker: System AI summary content empty for %s, retrying with lower temperature", uuid)
		params.Temperature = 0.3
		params.IsSummarize = true
		result2 := systemai.RunFallbackChain(httpClient, promptMessages, params, isVip)
		if result2.Error == "" {
			rawContent = strings.TrimSpace(result2.Content)
			summary, memories = byok.ParseSummaryResponse(rawContent)
		}
	}

	if summary == "" {
		log.Printf("Worker: No System AI summary content returned for %s after retry", uuid)
		markSummaryCompleted(uuid, storeKey, "[ERROR 500] No content returned from API", []string{}, 500, true)
		return
	}

	markSummaryCompleted(uuid, storeKey, summary, memories, 200, false)
}

func updateResponseContent(resp map[string]interface{}, cleanText string) {
	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return
	}
	msg, ok := choice["message"].(map[string]interface{})
	if !ok {
		return
	}
	msg["content"] = cleanText
	choice["message"] = msg
	choices[0] = choice
	resp["choices"] = choices
}

func markSystemAICompleted(uuid, storeKey string, response map[string]interface{}, statusCode int, bondEvent, bondStatus, relationshipLabel string, bondLevel int, userID uint) {
	if bondEvent != "" || bondStatus != "" || bondLevel > 0 {
		response["bond_event"] = bondEvent
		response["bond_status"] = bondStatus
		response["relationship_label"] = relationshipLabel
		response["bond_level"] = bondLevel
	}

	payload := map[string]interface{}{
		"status":      "completed",
		"response":    response,
		"status_code": statusCode,
	}
	// Preserve trace_id from original payload if exists
	if existing, err := services.GetChatResult(uuid); err == nil {
		var existingMap map[string]interface{}
		if json.Unmarshal(existing, &existingMap) == nil {
			if tid, ok := existingMap["trace_id"].(string); ok && tid != "" {
				payload["trace_id"] = tid
			}
		}
	}
	jsonBytes, _ := json.Marshal(payload)
	services.StoreChatResult(uuid, jsonBytes, 10*time.Minute)
	log.Printf("Worker: Completed system AI job %s with status %d", uuid, statusCode)

	// Drain pending queue
	nextUUID, err := services.PopPending(userID)
	if err != nil || nextUUID == "" {
		return
	}

	nextData, err := services.GetChatResult(nextUUID)
	if err != nil {
		log.Printf("Worker: Pending data for %s not found, skipping", nextUUID)
		return
	}

	var nextPayload map[string]interface{}
	if err := json.Unmarshal(nextData, &nextPayload); err != nil {
		log.Printf("Worker: Failed to parse pending payload for %s: %v", nextUUID, err)
		return
	}

	nextPayload["uuid"] = nextUUID
	enqBytes, _ := json.Marshal(nextPayload)
	services.Enqueue("ai-chat", enqBytes)
	services.SetCooldown(userID, systemai.CooldownDuration)
	log.Printf("Worker: Dispatched pending job %s from queue", nextUUID)
}

func handleGeminiSummarization(uuid, storeKey string, data chatData, decryptedKey string, messagesJSON []byte) {
	var messages []byok.Message
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		markError(uuid, storeKey, "Failed to parse messages", 400)
		return
	}

	lang := data.Language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	prompt := byok.BuildSummarizationPrompt(messages, lang, data.ExistingMemories)

	contents := []byok.GeminiContent{
		{
			Role:  "user",
			Parts: []byok.GeminiPart{{Text: prompt}},
		},
	}

	genConfig := byok.GenerationConfig{
		Temperature:     0.9,
		MaxOutputTokens: 4096,
	}

	isGemma := strings.Contains(data.Model, "gemma")
	if isGemma {
		genConfig.MaxOutputTokens = 32000
		if genConfig.ThinkingConfig == nil {
			genConfig.ThinkingConfig = &byok.ThinkingConfig{}
		}
		genConfig.ThinkingConfig.ThinkingLevel = "MINIMAL"
	}

	req := byok.GeminiRequest{
		Contents:         contents,
		GenerationConfig: genConfig,
		SafetySettings:   byok.DefaultSafetySettings,
	}

	geminiResp, statusCode, err := byok.CallGemini(decryptedKey, data.Model, req)
	if err != nil {
		log.Printf("Worker: Gemini summarization API error for %s: %v (status %d)", uuid, err, statusCode)
		errMsg := err.Error()
		if statusCode > 0 {
			errMsg = fmt.Sprintf("[ERROR %d] %s", statusCode, errMsg)
		}
		markSummaryCompleted(uuid, storeKey, errMsg, []string{}, statusCode, true)
		return
	}

	openaiResp := byok.ConvertToOpenAIFormat(geminiResp, data.Model)
	rawContent := ""
	if len(openaiResp.Choices) > 0 {
		rawContent = strings.TrimSpace(openaiResp.Choices[0].Message.Content)
	}

	summary, memories := byok.ParseSummaryResponse(rawContent)

	// Retry with lower temperature if content is empty
	if summary == "" && rawContent != "" {
		log.Printf("Worker: Summary content empty for %s, retrying with lower temperature", uuid)
		genConfig.Temperature = 0.3
		req.GenerationConfig = genConfig
		geminiResp, statusCode, err = byok.CallGemini(decryptedKey, data.Model, req)
		if err == nil {
			openaiResp = byok.ConvertToOpenAIFormat(geminiResp, data.Model)
			if len(openaiResp.Choices) > 0 {
				rawContent = strings.TrimSpace(openaiResp.Choices[0].Message.Content)
			}
			summary, memories = byok.ParseSummaryResponse(rawContent)
		}
	}

	if summary == "" {
		log.Printf("Worker: No summary content returned for %s after retry", uuid)
		markSummaryCompleted(uuid, storeKey, "[ERROR 500] No content returned from Gemini API", []string{}, 500, true)
		return
	}

	markSummaryCompleted(uuid, storeKey, summary, memories, statusCode, false)
}

func markSummaryCompleted(uuid, storeKey, summary string, memories []string, statusCode int, isError bool) {
	if memories == nil {
		memories = []string{}
	}
	response := map[string]interface{}{
		"summary":              summary,
		"memory_worth_keeping": memories,
	}
	if isError {
		response["error"] = true
		response["error_status_code"] = statusCode
	}
	respJSON, _ := json.Marshal(response)
	services.MarkChatCompleted(uuid, json.RawMessage(respJSON), statusCode)
	log.Printf("Worker: Completed summarization job %s with status %d", uuid, statusCode)
}

func processOpenAIChat(uuid, storeKey string, data chatData) {
	decryptedKey, err := utils.DecryptBYOKAPIKey(data.APIKey)
	if err != nil {
		log.Printf("Worker: Failed to decrypt API key for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to decrypt API key", 500)
		return
	}

	messagesJSON, err := base64.StdEncoding.DecodeString(data.Messages)
	if err != nil {
		log.Printf("Worker: Failed to decode messages for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to decode messages", 400)
		return
	}

	var messages []byok.Message
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		log.Printf("Worker: Failed to parse messages for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to parse messages", 400)
		return
	}

	model := data.Model
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	baseURL := data.BaseURL
	if baseURL == "" {
		baseURL = providerDefaultBaseURL(data.Provider)
	}
	if baseURL == "" {
		markError(uuid, storeKey, "Missing base_url", 400)
		return
	}

	// === Summarization ===
	if data.IsSummarize {
		handleOpenAISummarization(uuid, storeKey, data, decryptedKey, messages, model, baseURL)
		return
	}

	// === Chat ===
	userID := uint(data.UserID)
	botID := data.BotID
	lang := data.Language
	if lang == "" {
		lang = "id"
	}
	historyDepth := int(data.HistoryDepth)
	if historyDepth == 0 {
		historyDepth = 15
	}
	maxTokens := int(data.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 2048
	}
	var contextLimit *int
	if maxTokens > 0 {
		contextLimit = &maxTokens
	}
	var summary *string
	if data.Summary != "" {
		summary = &data.Summary
	}
	var userPersona *string
	if data.UserPersona != "" {
		userPersona = &data.UserPersona
	}

	// WithoutOverlapping lock
	if userID > 0 {
		lockKey := fmt.Sprintf("ai-chat-lock:%d:%s", userID, botID)
		if data.IsGroup {
			lockKey = fmt.Sprintf("ai-chat-lock:%d:group", userID)
		}
		locked, err := services.RedisClient.SetNX(services.RedisCtx, lockKey, "1", 120*time.Second).Result()
		if err != nil {
			log.Printf("Worker: Lock check failed for %s: %v", uuid, err)
		}
		if err == nil && !locked {
			log.Printf("Worker: Job %s skipped — overlapping lock held for %s", uuid, lockKey)
			return
		}
		if err == nil && locked {
			defer services.RedisClient.Del(services.RedisCtx, lockKey)
		}
	}

	shouldInject := data.Provider != "openai-compatible" || byok.IsTrustedBaseURL(baseURL)
	if !shouldInject {
		var usr models.User
		if err := database.DB.First(&usr, userID).Error; err == nil && usr.Role == "admin" {
			shouldInject = true
		}
	}

	if shouldInject {
		messages = byok.InjectByokSystemPrompt(
			messages, &botID, &lang, &historyDepth, contextLimit,
			summary, &userID, data.IsDisableBond, userPersona,
		)

		if data.IsGroup && len(data.BotIDs) > 1 {
			primaryBotID := botID
			messages = byok.InjectGroupContext(
				messages, data.BotIDs, &primaryBotID, lang, &userID,
			)
		}
	}

	openaiMessages := make([]byok.OpenAIChatMessage, 0, len(messages))
	for _, msg := range messages {
		role := msg.Role
		if role == "system" {
			role = "system"
		} else if role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		openaiMessages = append(openaiMessages, byok.OpenAIChatMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	temp := 1.0
	if data.Temperature != 0 {
		temp = data.Temperature
	}
	topP := 1.0
	if data.TopP != 0 {
		topP = data.TopP
	}
	maxOut := 1000
	if data.ResponseLength != 0 {
		maxOut = int(data.ResponseLength)
	} else if data.MaxTokens != 0 {
		maxOut = int(data.MaxTokens)
	}
	presPenalty := 0.0
	if data.PresencePenalty != 0 {
		presPenalty = data.PresencePenalty
	}
	freqPenalty := 0.0
	if data.FrequencyPenalty != 0 {
		freqPenalty = data.FrequencyPenalty
	}

	reqPayload := byok.OpenAICompatRequest{
		Model:            model,
		Messages:         openaiMessages,
		Temperature:      &temp,
		MaxTokens:        &maxOut,
		TopP:             &topP,
		FrequencyPenalty: &freqPenalty,
		PresencePenalty:  &presPenalty,
		Stop:             data.Stop,
	}

	// DeepSeek: ensure last message role is 'user'
	if data.Provider == "deepseek" && len(reqPayload.Messages) > 0 {
		lastIdx := len(reqPayload.Messages) - 1
		if reqPayload.Messages[lastIdx].Role == "assistant" {
			reqPayload.Messages[lastIdx].Role = "user"
		}
	}

	var openAIOpts *byok.OpenAIRequestOptions

	// OpenRouter: extra headers + body params
	if data.Provider == "openrouter" {
		openAIOpts = &byok.OpenAIRequestOptions{
			ExtraHeaders: map[string]string{
				"HTTP-Referer": config.Env("APP_URL", "https://hsai-stagging.my-bini.my.id"),
				"X-Title":      "HaloStory",
			},
			ExtraBody: map[string]interface{}{},
		}
		if data.TopK != 0 {
			openAIOpts.ExtraBody["top_k"] = data.TopK
		}
		if data.PresencePenalty != 0 {
			openAIOpts.ExtraBody["repetition_penalty"] = data.PresencePenalty
		}
	}

	openaiResp, statusCode, err := byok.CallOpenAI(httpClient, decryptedKey, baseURL, model, reqPayload, 120*time.Second, openAIOpts)
	if err != nil {
		log.Printf("Worker: OpenAI API error for %s: %v (status %d)", uuid, err, statusCode)
		markError(uuid, storeKey, "OpenAI API error: "+err.Error(), statusCode)
		return
	}

	if len(openaiResp.Choices) > 0 && openaiResp.Choices[0].Message.Content != "" {
		content := openaiResp.Choices[0].Message.Content
		content = byok.ProcessAIResponse(content)
		parsed := byok.ParseBondTags(content)
		openaiResp.Choices[0].Message.Content = parsed.CleanText

		var bot models.Bot
		database.DB.Where("bot_id = ?", data.BotID).First(&bot)

		var user models.User
		database.DB.First(&user, uint(data.UserID))

		if bot.BotID != "" && user.ID != 0 {
			messagesRaw, _ := base64.StdEncoding.DecodeString(data.Messages)
			var originalMessages []byok.Message
			_ = json.Unmarshal(messagesRaw, &originalMessages)

			lastUserMessage := ""
			for i := len(originalMessages) - 1; i >= 0; i-- {
				if originalMessages[i].Role == "user" {
					lastUserMessage = originalMessages[i].Content
					break
				}
			}

			byok.HandleChildSafety(&bot, &user, lastUserMessage, content, parsed.Tags)
		}

		// Convert to map for bond field injection
		respBytes, _ := json.Marshal(openaiResp)
		var respMap map[string]interface{}
		json.Unmarshal(respBytes, &respMap)

		if !data.IsGroup && !data.IsDisableBond && data.BotID != "" {
			bondResult := byok.AddBondXp(data.BotID, parsed, uint(data.UserID))
			respMap["bond_event"] = bondResult.BondEvent
			respMap["bond_status"] = bondResult.BondStatus
			respMap["relationship_label"] = bondResult.RelationshipLabel
			respMap["bond_level"] = bondResult.BondLevel
		} else {
			respMap["bond_event"] = nil
			respMap["bond_status"] = nil
			respMap["relationship_label"] = nil
			respMap["bond_level"] = nil
		}

		markCompleted(uuid, storeKey, respMap, 200)
		return
	}

	markCompleted(uuid, storeKey, openaiResp, 200)
}

func handleOpenAISummarization(uuid, storeKey string, data chatData, decryptedKey string, messages []byok.Message, model, baseURL string) {
	lang := data.Language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	prompt := byok.BuildSummarizationPrompt(messages, lang, data.ExistingMemories)

	temp := 0.9
	maxTokens := 16000
	reqPayload := byok.OpenAICompatRequest{
		Model: model,
		Messages: []byok.OpenAIChatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	var sumOpts *byok.OpenAIRequestOptions
	if data.Provider == "openrouter" {
		sumOpts = &byok.OpenAIRequestOptions{
			ExtraHeaders: map[string]string{
				"HTTP-Referer": config.Env("APP_URL", "https://hsai-stagging.my-bini.my.id"),
				"X-Title":      "HaloStory (Summarizer)",
			},
		}
	}

	openaiResp, statusCode, err := byok.CallOpenAI(httpClient, decryptedKey, baseURL, model, reqPayload, 60*time.Second, sumOpts)
	if err != nil {
		log.Printf("Worker: OpenAI summarization API error for %s: %v (status %d)", uuid, err, statusCode)
		errMsg := err.Error()
		if statusCode > 0 {
			errMsg = fmt.Sprintf("[ERROR %d] %s", statusCode, errMsg)
		}
		markSummaryCompleted(uuid, storeKey, errMsg, []string{}, statusCode, true)
		return
	}

	rawContent := ""
	if len(openaiResp.Choices) > 0 {
		rawContent = strings.TrimSpace(openaiResp.Choices[0].Message.Content)
	}

	summary, memories := byok.ParseSummaryResponse(rawContent)

	if summary == "" && rawContent != "" {
		log.Printf("Worker: OpenAI summary content empty for %s, retrying with lower temperature", uuid)
		temp = 0.3
		reqPayload.Temperature = &temp
		openaiResp, statusCode, err = byok.CallOpenAI(httpClient, decryptedKey, baseURL, model, reqPayload, 60*time.Second, sumOpts)
		if err == nil && len(openaiResp.Choices) > 0 {
			rawContent = strings.TrimSpace(openaiResp.Choices[0].Message.Content)
			summary, memories = byok.ParseSummaryResponse(rawContent)
		}
	}

	if summary == "" {
		log.Printf("Worker: No OpenAI summary content returned for %s after retry", uuid)
		markSummaryCompleted(uuid, storeKey, "[ERROR 500] No content returned from API", []string{}, 500, true)
		return
	}

	markSummaryCompleted(uuid, storeKey, summary, memories, statusCode, false)
}

func processClaudeChat(uuid, storeKey string, data chatData) {
	decryptedKey, err := utils.DecryptBYOKAPIKey(data.APIKey)
	if err != nil {
		log.Printf("Worker: Failed to decrypt API key for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to decrypt API key", 500)
		return
	}

	messagesJSON, err := base64.StdEncoding.DecodeString(data.Messages)
	if err != nil {
		log.Printf("Worker: Failed to decode messages for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to decode messages", 400)
		return
	}

	var messages []byok.Message
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		log.Printf("Worker: Failed to parse messages for %s: %v", uuid, err)
		markError(uuid, storeKey, "Failed to parse messages", 400)
		return
	}

	userID := uint(data.UserID)
	botID := data.BotID
	lang := data.Language
	if lang == "" {
		lang = "id"
	}
	historyDepth := int(data.HistoryDepth)
	if historyDepth == 0 {
		historyDepth = 15
	}
	maxTokens := int(data.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 2048
	}
	var contextLimit *int
	if maxTokens > 0 {
		contextLimit = &maxTokens
	}
	var summary *string
	if data.Summary != "" {
		summary = &data.Summary
	}
	var userPersona *string
	if data.UserPersona != "" {
		userPersona = &data.UserPersona
	}

	// WithoutOverlapping lock
	if userID > 0 {
		lockKey := fmt.Sprintf("ai-chat-lock:%d:%s", userID, botID)
		if data.IsGroup {
			lockKey = fmt.Sprintf("ai-chat-lock:%d:group", userID)
		}
		locked, err := services.RedisClient.SetNX(services.RedisCtx, lockKey, "1", 120*time.Second).Result()
		if err != nil {
			log.Printf("Worker: Lock check failed for %s: %v", uuid, err)
		}
		if err == nil && !locked {
			log.Printf("Worker: Job %s skipped — overlapping lock held for %s", uuid, lockKey)
			return
		}
		if err == nil && locked {
			defer services.RedisClient.Del(services.RedisCtx, lockKey)
		}
	}

	if data.IsSummarize {
		handleClaudeSummarization(uuid, storeKey, data, decryptedKey, messages, messagesJSON)
		return
	}

	messages = byok.InjectByokSystemPrompt(
		messages, &botID, &lang, &historyDepth, contextLimit,
		summary, &userID, data.IsDisableBond, userPersona,
	)

	if data.IsGroup && len(data.BotIDs) > 1 {
		primaryBotID := botID
		messages = byok.InjectGroupContext(
			messages, data.BotIDs, &primaryBotID, lang, &userID,
		)
	}

	payloadMap := map[string]interface{}{
		"model":             data.Model,
		"temperature":       data.Temperature,
		"top_p":             data.TopP,
		"top_k":             data.TopK,
		"response_length":   data.ResponseLength,
		"max_tokens":        data.MaxTokens,
		"stop":              data.Stop,
		"reasoning":         data.Reasoning,
		"bot_id":            data.BotID,
		"user_id":           data.UserID,
		"language":          data.Language,
		"history_depth":     data.HistoryDepth,
		"is_group":          data.IsGroup,
		"isdisablebond":     data.IsDisableBond,
		"user_persona":      data.UserPersona,
		"bot_ids":           data.BotIDs,
		"existing_memories": data.ExistingMemories,
	}

	claudeReq := byok.BuildClaudePayload(messages, data.Model, payloadMap)

	claudeResp, statusCode, err := byok.CallClaude(decryptedKey, data.Model, claudeReq, 200*time.Second)
	if err != nil {
		log.Printf("Worker: Claude API error for %s: %v (status %d)", uuid, err, statusCode)
		markError(uuid, storeKey, "Claude API error: "+err.Error(), statusCode)
		return
	}

	openaiFormat := byok.ConvertClaudeToOpenAIFormat(claudeResp, data.Model)

	content := ""
	var msg map[string]string
	var msgOK bool
	choices, choicesOK := openaiFormat["choices"].([]map[string]interface{})
	if choicesOK && len(choices) > 0 {
		msg, msgOK = choices[0]["message"].(map[string]string)
		if msgOK {
			content = msg["content"]
		}
	}

	if content != "" {
		content = byok.ProcessAIResponse(content)
		parsed := byok.ParseBondTags(content)
		if msgOK {
			msg["content"] = parsed.CleanText
			choices[0]["message"] = msg
			openaiFormat["choices"] = choices
		}

		var bot models.Bot
		database.DB.Where("bot_id = ?", data.BotID).First(&bot)

		var user models.User
		database.DB.First(&user, uint(data.UserID))

		if bot.BotID != "" && user.ID != 0 {
			messagesRaw, _ := base64.StdEncoding.DecodeString(data.Messages)
			var originalMessages []byok.Message
			_ = json.Unmarshal(messagesRaw, &originalMessages)

			lastUserMessage := ""
			for i := len(originalMessages) - 1; i >= 0; i-- {
				if originalMessages[i].Role == "user" {
					lastUserMessage = originalMessages[i].Content
					break
				}
			}

			byok.HandleChildSafety(&bot, &user, lastUserMessage, content, parsed.Tags)
		}

		if !data.IsGroup && !data.IsDisableBond {
			bondResult := byok.AddBondXp(data.BotID, parsed, uint(data.UserID))

			openaiFormat["bond_event"] = bondResult.BondEvent
			openaiFormat["bond_status"] = bondResult.BondStatus
			openaiFormat["relationship_label"] = bondResult.RelationshipLabel
			openaiFormat["bond_level"] = bondResult.BondLevel
		} else {
			openaiFormat["bond_event"] = nil
			openaiFormat["bond_status"] = nil
			openaiFormat["relationship_label"] = nil
			openaiFormat["bond_level"] = nil
		}
	}

	markCompleted(uuid, storeKey, openaiFormat, 200)
}

func handleClaudeSummarization(uuid, storeKey string, data chatData, decryptedKey string, messages []byok.Message, messagesJSON []byte) {
	lang := data.Language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	var parsedMessages []byok.Message
	if err := json.Unmarshal(messagesJSON, &parsedMessages); err != nil {
		markError(uuid, storeKey, "Failed to parse messages", 400)
		return
	}

	prompt := byok.BuildSummarizationPrompt(parsedMessages, lang, data.ExistingMemories)

	payload := byok.ClaudeRequest{
		Model:       data.Model,
		MaxTokens:   16000,
		Temperature: func() *float64 { t := 0.9; return &t }(),
		Messages: []byok.ClaudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	maxRetries := 10
	var claudeResp *byok.ClaudeResponse
	var statusCode int
	var callErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		claudeResp, statusCode, callErr = byok.CallClaude(decryptedKey, data.Model, payload, 60*time.Second)
		if callErr == nil {
			break
		}
		if statusCode == 429 {
			break
		}
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}

	if callErr != nil {
		errMsg := callErr.Error()
		if statusCode > 0 {
			errMsg = fmt.Sprintf("[ERROR %d] %s", statusCode, errMsg)
		}
		markSummaryCompleted(uuid, storeKey, errMsg, []string{}, statusCode, true)
		return
	}

	rawContent := ""
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			rawContent += block.Text
		}
	}
	rawContent = strings.TrimSpace(rawContent)

	summary, memories := byok.ParseSummaryResponse(rawContent)

	if summary == "" && rawContent != "" {
		log.Printf("Worker: Claude summary empty for %s, retrying with lower temperature", uuid)
		t := 0.3
		payload.Temperature = &t
		claudeResp, statusCode, callErr = byok.CallClaude(decryptedKey, data.Model, payload, 60*time.Second)
		if callErr == nil {
			retryContent := ""
			for _, block := range claudeResp.Content {
				if block.Type == "text" {
					retryContent += block.Text
				}
			}
			retryContent = strings.TrimSpace(retryContent)
			if retryContent != "" {
				summary, memories = byok.ParseSummaryResponse(retryContent)
			}
		}
	}

	if summary == "" {
		log.Printf("Worker: No Claude summary content for %s after retry", uuid)
		markSummaryCompleted(uuid, storeKey, "[ERROR 500] No content returned from Claude API", []string{}, 500, true)
		return
	}

	markSummaryCompleted(uuid, storeKey, summary, memories, statusCode, false)
}

func providerDefaultBaseURL(provider string) string {
	switch provider {
	case "mistral":
		return "https://api.mistral.ai/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "zai":
		return "https://api.z.ai/api/paas/v4"
	default:
		return ""
	}
}

func processMockChat(uuid, storeKey string, data chatData) {
	data.Status = "processing"
	updateJSON, _ := json.Marshal(data)
	services.StoreChatResult(uuid, updateJSON, 10*time.Minute)

	time.Sleep(200 * time.Millisecond)

	response := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Mock response for testing.",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     10,
			"completion_tokens": 5,
			"total_tokens":      15,
		},
	}

	respJSON, _ := json.Marshal(response)
	services.MarkChatCompleted(uuid, json.RawMessage(respJSON), 200)
	log.Printf("Worker: Completed mock job %s", uuid)
}

func markError(uuid, storeKey, message string, statusCode int) {
	respJSON, _ := json.Marshal(map[string]interface{}{
		"error":             message,
		"error_status_code": statusCode,
	})
	services.MarkChatCompleted(uuid, json.RawMessage(respJSON), statusCode)
	log.Printf("Worker: Error job %s: %s (status %d)", uuid, message, statusCode)
}
