package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services/systemai"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker/types"
	"halostory_go_backend/internal/services"
)

// HandleSystemAI handles a System AI chat job (non-summarization).
func HandleSystemAI(ctx context.Context, uuid string, data types.ChatData) (map[string]interface{}, int, *types.WorkerError, uint) {
	rawBytes, messages, werr := DecodeMessagesRaw(data.Messages)
	if werr != nil {
		return nil, 0, werr, 0
	}
	_ = rawBytes

	userID := uint(data.UserID)
	botID := data.BotID

	if userID > 0 {
		acquired, err := types.AcquireChatLock(userID, botID, data.IsGroup)
		if err == nil && !acquired {
			log.Printf("Worker: Job %s skipped — overlapping lock held", uuid)
			return nil, 0, nil, userID
		}
		if err == nil && acquired {
			defer types.ReleaseChatLock(userID, botID, data.IsGroup)
		}
	}

	lang := data.Language
	if lang == "" {
		lang = systemai.DefaultLanguage
	}
	historyDepth := HistoryDepth(data.HistoryDepth)
	if data.HistoryDepth == 0 {
		historyDepth = systemai.DefaultHistoryDepth
	}
	maxTokens := int(data.MaxTokens)
	if maxTokens == 0 {
		maxTokens = systemai.DefaultMaxTokens
	}
	var contextLimit *int
	if data.ContextLimit > 0 {
		cl := int(data.ContextLimit)
		contextLimit = &cl
	} else {
		contextLimit = &maxTokens
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

	sysModel := data.Model
	if sysModel == "" {
		if isVip {
			sysModel = "system-vip"
		} else {
			sysModel = "system-free"
		}
	}

	loreInjection := types.LoreInjection(botID, messages)
	var summaryPtr *string
	if data.Summary != "" {
		summaryPtr = &data.Summary
	}
	var userPersonaPtr *string
	if data.UserPersona != "" {
		userPersonaPtr = &data.UserPersona
	}
	messages = byok.InjectByokSystemPromptWithModel(
		messages, &botID, &lang, &historyDepth, contextLimit,
		summaryPtr, &userID, data.IsDisableBond, userPersonaPtr,
		sysModel,
	)
	if loreInjection != "" {
		for i, m := range messages {
			if m.Role == "system" {
				messages[i].Content += loreInjection
				loreInjection = ""
				break
			}
		}
		if loreInjection != "" {
			messages = append([]byok.Message{{Role: "system", Content: loreInjection}}, messages...)
		}
	}
	if data.IsGroup && len(data.BotIDs) > 1 {
		primaryBotID := botID
		messages = byok.InjectGroupContext(messages, data.BotIDs, &primaryBotID, lang, &userID)
	}

	httpClient := utils.NewAIHttpClient()

	var result *systemai.ChainResult
	if types.ShouldStream(data.IsStream) {
		streamCtx, streamCancel := context.WithTimeout(ctx, 200*time.Second)
		result = systemai.RunFallbackChainStream(streamCtx, messages, params, isVip, func(delta string) error {
			_ = services.PublishStream(uuid, delta)
			return nil
		})
		streamCancel()
		if result.Error == "" {
			_ = services.PublishStreamDone(uuid)
		} else {
			log.Printf("Worker: System AI stream gagal untuk %s: %s — fallback non-stream", uuid, result.Error)
			result = systemai.RunFallbackChain(httpClient, messages, params, isVip)
		}
	} else {
		result = systemai.RunFallbackChain(httpClient, messages, params, isVip)
	}

	if result.Error != "" {
		return nil, 0, types.NewWorkerError(result.Error, result.StatusCode), userID
	}

	content := byok.ProcessAIResponse(result.Content)
	parsed := byok.ParseBondTags(content)

	// Update response content with clean text
	updateResponseContent(result.Response, parsed.CleanText)

	var bot models.Bot
	database.DB.Where("bot_id = ?", data.BotID).First(&bot)

	// Decode original for last user message (child safety)
	var originalMessages []byok.Message
	_ = json.Unmarshal(rawBytes, &originalMessages)
	lastUserMessage := ""
	for i := len(originalMessages) - 1; i >= 0; i-- {
		if originalMessages[i].Role == "user" {
			lastUserMessage = originalMessages[i].Content
			break
		}
	}
	if bot.BotID != "" && user.ID != 0 {
		byok.HandleChildSafety(&bot, &user, lastUserMessage, content, parsed.Tags)
	}

	bondEvent := ""
	bondStatus := ""
	relationshipLabel := ""
	bondLevel := 0
	if !data.IsGroup && !data.IsDisableBond && bot.BotID != "" {
		bondResult := byok.AddBondXp(data.BotID, parsed, userID)
		bondEvent = bondResult.BondEvent
		bondStatus = bondResult.BondStatus
		relationshipLabel = bondResult.RelationshipLabel
		bondLevel = bondResult.BondLevel
	}

	// Attach bond fields to response
	if bondEvent != "" || bondStatus != "" || bondLevel > 0 {
		result.Response["bond_event"] = bondEvent
		result.Response["bond_status"] = bondStatus
		result.Response["relationship_label"] = relationshipLabel
		result.Response["bond_level"] = bondLevel
	}

	return result.Response, 200, nil, userID
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
