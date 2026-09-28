package handlers

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker/types"
)

// HandleGemini handles a BYOK Gemini chat job (non-summarization).
func HandleGemini(ctx context.Context, uuid string, data types.ChatData) (map[string]interface{}, int, *types.WorkerError) {
	decryptedKey, werr := DecryptKey(data.APIKey)
	if werr != nil {
		return nil, 0, werr
	}

	_, messages, werr := DecodeMessagesRaw(data.Messages)
	if werr != nil {
		return nil, 0, werr
	}

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

	loreInjection := types.LoreInjection(botID, messages)
	messages = PreparePromptMessages(messages, botID, lang, historyDepth, contextLimit, summary, userID, data.IsDisableBond, userPersona, data.Model, true, loreInjection, data.BotIDs, data.IsGroup)

	contents, systemInstruct, systemText := buildGeminiContents(messages, data.Model)

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
	presPenalty := data.PresencePenalty
	freqPenalty := data.FrequencyPenalty

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
		// Global low — paksa 0.2*maxTokens (budget minimal, hemat latensi)
		maxOutTokens := int(data.MaxTokens)
		if maxOutTokens == 0 {
			maxOutTokens = 2048
		}
		thinkingBudget := int(float64(maxOutTokens) * 0.2)
		includeThoughts := !data.Reasoning.Exclude
		genConfig.ThinkingConfig = &byok.ThinkingConfig{
			IncludeThoughts: &includeThoughts,
			ThinkingBudget:  &thinkingBudget,
		}
	}

	isGemma := strings.Contains(data.Model, "gemma")
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
	_ = systemText

	var geminiResp *byok.GeminiResponse
	var statusCode int
	var err error

	if types.ShouldStream(data.IsStream) {
		streamClient := utils.NewAIHttpClientNoTimeout()
		sctx, cancel := context.WithTimeout(ctx, 200*time.Second)
		defer cancel()
		sanitizer := &byok.StreamSanitizer{}
		var streamResp *byok.GeminiResponse
		streamErr := func() error {
			r, _, ce := byok.CallGeminiStream(sctx, streamClient, decryptedKey, data.Model, req, func(delta string) error {
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
			geminiResp = streamResp
			statusCode = 200
		} else {
			geminiResp, statusCode, err = byok.CallGemini(decryptedKey, data.Model, req)
		}
	} else {
		geminiResp, statusCode, err = byok.CallGemini(decryptedKey, data.Model, req)
	}

	if err != nil {
		return nil, 0, types.NewWorkerError("[GoogleAI] response failed: "+err.Error(), statusCode)
	}

	if geminiResp.PromptFeedback != nil {
		blocked := geminiResp.PromptFeedback.BlockReasonMessage
		if blocked == "" {
			blocked = geminiResp.PromptFeedback.BlockReason
		}
		if blocked != "" {
			openaiResp := byok.ConvertToOpenAIFormat(geminiResp, data.Model)
			openaiResp.Choices[0].Message.Content = "I'm sorry, tapi prompt ini diblokir oleh sensor Google... (" + blocked + ")"
			openaiResp.Choices[0].FinishReason = "content_filter"
			respBytes, _ := json.Marshal(openaiResp)
			var respMap map[string]interface{}
			_ = json.Unmarshal(respBytes, &respMap)
			respMap["error"] = "[GoogleAI] Prompt was blocked: " + blocked
			return respMap, 400, nil
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
		database.DB.First(&user, userID)
		if bot.BotID != "" && user.ID != 0 {
			rawMessages, _ := DecodeMessages(data.Messages)
			lastUser := LastUserMessage(rawMessages)
			byok.HandleChildSafety(&bot, &user, lastUser, content, parsed.Tags)
		}

		respBytes, _ := json.Marshal(openaiResp)
		var respMap map[string]interface{}
		_ = json.Unmarshal(respBytes, &respMap)

		if !data.IsGroup && !data.IsDisableBond && data.BotID != "" {
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

	b, _ := json.Marshal(openaiResp)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m, 200, nil
}

func buildGeminiContents(messages []byok.Message, model string) ([]byok.GeminiContent, *byok.GeminiContent, string) {
	isGemma := strings.Contains(model, "gemma")
	isModernGemma := false
	if isGemma && len(model) >= 7 {
		v := model[6]
		isModernGemma = v >= '2' && v <= '9'
	}
	supportsSystemInstruction := !isGemma || isModernGemma

	var systemInstruct *byok.GeminiContent
	systemText := ""
	contents := make([]byok.GeminiContent, 0)

	for _, msg := range messages {
		if msg.Role == "system" {
			if !supportsSystemInstruction {
				systemText = msg.Content.AsText()
			} else {
				systemInstruct = &byok.GeminiContent{
					Parts: []byok.GeminiPart{{Text: msg.Content.AsText()}},
				}
			}
			continue
		}
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}
		text := msg.Content.AsText()
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
	return contents, systemInstruct, systemText
}
