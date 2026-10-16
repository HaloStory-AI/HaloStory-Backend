package handlers

import (
	"context"
	"encoding/json"
	"time"

	"halostory_go_backend/internal/config"
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

	botID := data.BotID
	lang := data.Language
	if lang == "" {
		lang = "id"
	}
	historyDepth := HistoryDepth(data.HistoryDepth)
	contextLimit := ContextLimitPtr(data.ContextLimit, data.MaxTokens)
	summary := SummaryPtr(data.Summary)
	userPersona := UserPersonaPtr(data.UserPersona)
	userName := data.UserName
	if userName == "" {
		userName = "Sensei"
	}

	isTrusted := data.Provider != "openai-compatible" || byok.IsTrustedBaseURL(data.BaseURL)

	loreInjection := types.LoreInjection(botID, messages)
	messages = PreparePromptMessages(messages, botID, lang, historyDepth, contextLimit, summary, userName, userPersona, model, loreInjection, nil, false)
	if isTrusted && data.IsGroup && len(data.BotIDs) > 1 {
		primaryBotID := botID
		messages = byok.InjectGroupContext(messages, data.BotIDs, &primaryBotID, lang, userName)
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

			respBytes, _ := json.Marshal(openaiResp)
			var respMap map[string]interface{}
			_ = json.Unmarshal(respBytes, &respMap)

			return respMap, 200, nil
		}
	}

	b, _ := json.Marshal(openaiResp)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m, 200, nil
}
