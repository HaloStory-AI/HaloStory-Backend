package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker/types"
)

// HandleOpenAI handles BYOK OpenAI-compatible chat (openai-compatible, mistral, deepseek, openrouter, zai).
func HandleOpenAI(ctx context.Context, uuid string, data types.ChatData) (map[string]interface{}, int, *types.WorkerError) {
	decryptedKey, werr := DecryptKey(data.APIKey)
	if werr != nil {
		return nil, 0, werr
	}

	rawJSON, messages, werr := DecodeMessagesRaw(data.Messages)
	if werr != nil {
		return nil, 0, werr
	}
	_ = rawJSON

	model := data.Model
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	// Summarization is handled by summarizer package; this handler only does chat
	userID := uint(data.UserID)
	botID := data.BotID
	lang := data.Language
	if lang == "" {
		lang = "id"
	}
	historyDepth := HistoryDepth(data.HistoryDepth)
	contextLimit := ContextLimitPtr(data.ContextLimit, data.MaxTokens)
	summary := SummaryPtr(data.Summary)
	userPersona := UserPersonaPtr(data.UserPersona)

	if userID > 0 {
		acquired, err := types.AcquireChatLock(userID, botID, data.IsGroup)
		if err == nil && !acquired {
			log.Printf("Worker: Job %s skipped — overlapping lock held", uuid)
			return nil, 0, nil
		}
		if err == nil && acquired {
			defer types.ReleaseChatLock(userID, botID, data.IsGroup)
		}
	}

	// Trusted base URL gate
	isTrusted := data.Provider != "openai-compatible" || byok.IsTrustedBaseURL(data.BaseURL)
	if !isTrusted && userID > 0 {
		var usr models.User
		if err := database.DB.First(&usr, userID).Error; err == nil && usr.Role == "admin" {
			isTrusted = true
		}
	}

	loreInjection := types.LoreInjection(botID, messages)
	messages = PreparePromptMessages(messages, botID, lang, historyDepth, contextLimit, summary, userID, data.IsDisableBond, userPersona, model, isTrusted, loreInjection, nil, false)
	// Group context only if trusted
	if isTrusted && data.IsGroup && len(data.BotIDs) > 1 {
		// PreparePromptMessages already handles group when passed botIDs, but we passed nil to avoid double-inject for untrusted.
		// So inject group now for trusted path if not already injected.
		// Re-check: PreparePromptMessages with isGroup=false didn't inject; do it now.
		primaryBotID := botID
		uidPtr := &userID
		if userID == 0 {
			uidPtr = nil
		}
		messages = byok.InjectGroupContext(messages, data.BotIDs, &primaryBotID, lang, uidPtr)
	}

	openaiMessages := FormatOpenAIMessages(messages)

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

	if data.Provider == "deepseek" && len(reqPayload.Messages) > 0 {
		lastIdx := len(reqPayload.Messages) - 1
		if reqPayload.Messages[lastIdx].Role == "assistant" {
			reqPayload.Messages[lastIdx].Role = "user"
		}
	}

	var openAIOpts *byok.OpenAIRequestOptions
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

	httpClient := utils.NewAIHttpClient()
	var openaiResp *byok.OpenAICompatResponse
	var statusCode int
	var err error

	if types.ShouldStream(data.IsStream) {
		streamClient := utils.NewAIHttpClientNoTimeout()
		sctx, cancel := context.WithTimeout(ctx, 200*time.Second)
		defer cancel()
		sanitizer := &byok.StreamSanitizer{}
		var streamResp *byok.OpenAICompatResponse
		streamErr := func() error {
			r, _, ce := byok.CallOpenAIStream(sctx, streamClient, decryptedKey, data.BaseURL, model, reqPayload, openAIOpts, func(delta string) error {
				if clean := sanitizer.Feed(delta); clean != "" {
					_ = services.PublishStream(uuid, clean)
				}
				return nil
			})
			if ce != nil {
				return ce
			}
			streamResp = r
			return nil
		}()
		if streamErr == nil && streamResp != nil {
			if rest := sanitizer.Flush(); rest != "" {
				_ = services.PublishStream(uuid, rest)
			}
			_ = services.PublishStreamDone(uuid)
			openaiResp = streamResp
			statusCode = 200
		} else {
			openaiResp, statusCode, err = byok.CallOpenAI(httpClient, decryptedKey, data.BaseURL, model, reqPayload, 120*time.Second, openAIOpts)
			if err != nil {
				return nil, 0, types.NewWorkerError("OpenAI API error: "+err.Error(), statusCode)
			}
		}
	} else {
		openaiResp, statusCode, err = byok.CallOpenAI(httpClient, decryptedKey, data.BaseURL, model, reqPayload, 120*time.Second, openAIOpts)
		if err != nil {
			return nil, 0, types.NewWorkerError("OpenAI API error: "+err.Error(), statusCode)
		}
	}
	if openaiResp == nil {
		return nil, 0, types.NewWorkerError("OpenAI API error: nil response", statusCode)
	}

	if len(openaiResp.Choices) > 0 {
		var rawContent string
		switch v := openaiResp.Choices[0].Message.Content.(type) {
		case string:
			rawContent = v
		default:
			b, _ := json.Marshal(v)
			_ = json.Unmarshal(b, &rawContent)
		}
		if rawContent != "" {
				content := byok.ProcessAIResponse(rawContent)
			parsed := byok.ParseBondTags(content)
			openaiResp.Choices[0].Message.Content = parsed.CleanText

			if isTrusted {
			var bot models.Bot
			database.DB.Where("bot_id = ?", data.BotID).First(&bot)
			var user models.User
			database.DB.First(&user, userID)
			if bot.BotID != "" && user.ID != 0 {
				origMessages, _ := DecodeMessages(data.Messages)
				lastUser := LastUserMessage(origMessages)
				byok.HandleChildSafety(&bot, &user, lastUser, content, parsed.Tags)
			}
		}

		respBytes, _ := json.Marshal(openaiResp)
		var respMap map[string]interface{}
		_ = json.Unmarshal(respBytes, &respMap)

		if isTrusted && !data.IsGroup && !data.IsDisableBond && data.BotID != "" {
			bondResult := byok.AddBondXp(data.BotID, parsed, userID)
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
		return respMap, 200, nil
		}
	}

	b, _ := json.Marshal(openaiResp)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m, 200, nil
}
