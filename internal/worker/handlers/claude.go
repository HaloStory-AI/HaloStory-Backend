package handlers

import (
	"context"
	"log"
	"time"

	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker/types"
)

// HandleClaude handles a BYOK Claude chat job (non-summarization).
func HandleClaude(ctx context.Context, uuid string, data types.ChatData) (map[string]interface{}, int, *types.WorkerError) {
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
			log.Printf("Worker: Job %s skipped â€” overlapping lock held", uuid)
			return nil, 0, nil
		}
		if err == nil && acquired {
			defer types.ReleaseChatLock(userID, botID, data.IsGroup)
		}
	}

	loreInjection := types.LoreInjection(botID, messages)
	messages = PreparePromptMessages(messages, botID, lang, historyDepth, contextLimit, summary, userID, userPersona, data.Model, loreInjection, data.BotIDs, data.IsGroup)

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
		"user_persona":      data.UserPersona,
		"bot_ids":           data.BotIDs,
		"existing_memories": data.ExistingMemories,
	}

	claudeReq := byok.BuildClaudePayload(messages, data.Model, payloadMap)

	var claudeResp *byok.ClaudeResponse
	var statusCode int
	var err error

	if types.ShouldStream(data.IsStream) {
		streamClient := utils.NewAIHttpClientNoTimeout()
		sctx, cancel := context.WithTimeout(ctx, 200*time.Second)
		defer cancel()
		sanitizer := &byok.StreamSanitizer{}
		var streamResp *byok.ClaudeResponse
		streamErr := func() error {
			r, _, ce := byok.CallClaudeStream(sctx, streamClient, decryptedKey, data.Model, claudeReq, func(delta string) error {
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
			claudeResp = streamResp
			statusCode = 200
		} else {
			claudeResp, statusCode, err = byok.CallClaude(decryptedKey, data.Model, claudeReq, 200*time.Second)
			if err != nil {
				return nil, 0, types.NewWorkerError("Claude API error: "+err.Error(), statusCode)
			}
		}
	} else {
		claudeResp, statusCode, err = byok.CallClaude(decryptedKey, data.Model, claudeReq, 200*time.Second)
		if err != nil {
			return nil, 0, types.NewWorkerError("Claude API error: "+err.Error(), statusCode)
		}
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
	}

	return openaiFormat, 200, nil
}
