package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"
)

type ChatJobPayload struct {
	UUID     string `json:"uuid"`
	UserID   uint   `json:"user_id"`
	BotID    string `json:"bot_id"`
	IsGroup  bool   `json:"is_group"`
}

type chatData struct {
	Status            string                   `json:"status"`
	APIKey            string                   `json:"api_key"`
	Messages          string                   `json:"messages"`
	Model             string                   `json:"model"`
	Provider          string                   `json:"provider"`
	UserID            float64                  `json:"user_id"`
	BotID             string                   `json:"bot_id"`
	Language          string                   `json:"language,omitempty"`
	Temperature       float64                  `json:"temperature,omitempty"`
	MaxTokens         float64                  `json:"max_tokens,omitempty"`
	TopP              float64                  `json:"top_p,omitempty"`
	TopK              float64                  `json:"top_k,omitempty"`
	PresencePenalty   float64                  `json:"presence_penalty,omitempty"`
	FrequencyPenalty  float64                  `json:"frequency_penalty,omitempty"`
	Summary           string                   `json:"summary,omitempty"`
	IsSummarize       bool                     `json:"is_summarize,omitempty"`
	HistoryDepth      float64                  `json:"history_depth,omitempty"`
	IsGroup           bool                     `json:"is_group,omitempty"`
	IsDisableBond     bool                     `json:"isdisablebond,omitempty"`
	UserPersona       string                   `json:"user_persona,omitempty"`
	BotIDs            []string                 `json:"bot_ids,omitempty"`
	ExistingMemories  []string                 `json:"existing_memories,omitempty"`
	Reasoning         *reasoningConfig         `json:"reasoning,omitempty"`
	Stop              []string                 `json:"stop,omitempty"`
	ResponseLength    float64                  `json:"response_length,omitempty"`
}

type reasoningConfig struct {
	Enabled   bool    `json:"enabled"`
	Effort    string  `json:"effort,omitempty"`
	MaxTokens float64 `json:"max_tokens,omitempty"`
	Exclude   bool    `json:"exclude,omitempty"`
}

func main() {
	config.LoadConfig()
	database.Connect()
	services.InitRedis()

	log.Println("Worker service starting... Waiting for jobs on queue:ai-chat")

	go func() {
		for {
			_, data, err := services.Dequeue("ai-chat", 5*time.Second)
			if err != nil {
				continue
			}

			go processJob(data)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Worker service shutting down...")
}

func processJob(rawData []byte) {
	var payload map[string]interface{}
	if err := json.Unmarshal(rawData, &payload); err != nil {
		log.Printf("Worker: Failed to parse job payload: %v", err)
		return
	}

	uuid, _ := payload["uuid"].(string)
	if uuid == "" {
		log.Printf("Worker: Missing uuid in payload")
		return
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

	if data.Provider != "" && data.Provider != "gemini" {
		log.Printf("Worker: Unsupported provider %s for UUID %s", data.Provider, uuid)
		markError(uuid, storeKey, "Unsupported provider: "+data.Provider, 400)
		return
	}

	processGeminiChat(uuid, storeKey, data)
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
	if len(data.Model) >= 10 {
		lastChar := data.Model[len(data.Model)-1]
		isModernGemma = isGemma && lastChar >= '2' && lastChar <= '9'
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
		markError(uuid, storeKey, "Gemini API error: "+err.Error(), statusCode)
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
			markCompleted(uuid, storeKey, openaiResp, 400)
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

		if !data.IsGroup && !data.IsDisableBond {
			bondResult := byok.AddBondXp(data.BotID, parsed, uint(data.UserID))

			openaiRespRaw := make(map[string]interface{})
			jsonBytes, _ := json.Marshal(openaiResp)
			json.Unmarshal(jsonBytes, &openaiRespRaw)
			openaiRespRaw["bond_event"] = bondResult.BondEvent
			openaiRespRaw["bond_status"] = bondResult.BondStatus
			openaiRespRaw["relationship_label"] = bondResult.RelationshipLabel
			openaiRespRaw["bond_level"] = bondResult.BondLevel

			resultJSON, _ := json.Marshal(openaiRespRaw)
			services.StoreChatResult(uuid, resultJSON, 10*time.Minute)

			finalPayload := map[string]interface{}{
				"status":      "completed",
				"response":    openaiRespRaw,
				"status_code": 200,
			}
			finalJSON, _ := json.Marshal(finalPayload)
			services.StoreChatResult(uuid, finalJSON, 10*time.Minute)
			return
		}
	}

	markCompleted(uuid, storeKey, openaiResp, 200)
}

func markCompleted(uuid, storeKey string, response interface{}, statusCode int) {
	payload := map[string]interface{}{
		"status":      "completed",
		"response":    response,
		"status_code": statusCode,
	}
	jsonBytes, _ := json.Marshal(payload)
	services.StoreChatResult(uuid, jsonBytes, 10*time.Minute)
	log.Printf("Worker: Completed job %s with status %d", uuid, statusCode)
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
	response := map[string]interface{}{
		"summary":              summary,
		"memory_worth_keeping": memories,
	}
	payload := map[string]interface{}{
		"status":      "completed",
		"response":    response,
		"status_code": statusCode,
	}
	if isError {
		payload["error"] = true
		payload["error_status_code"] = statusCode
	}
	jsonBytes, _ := json.Marshal(payload)
	services.StoreChatResult(uuid, jsonBytes, 10*time.Minute)
	log.Printf("Worker: Completed summarization job %s with status %d", uuid, statusCode)
}

func markError(uuid, storeKey, message string, statusCode int) {
	payload := map[string]interface{}{
		"status":      "completed",
		"status_code": statusCode,
		"response": map[string]interface{}{
			"error":   message,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]string{
						"role":    "assistant",
						"content": "An error occurred while processing your request.",
					},
					"finish_reason": "error",
				},
			},
		},
	}
	jsonBytes, _ := json.Marshal(payload)
	services.StoreChatResult(uuid, jsonBytes, 10*time.Minute)
	log.Printf("Worker: Error job %s: %s (status %d)", uuid, message, statusCode)
}
