package systemai

import (
	"encoding/json"
	"fmt"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/services/byok"
)

func buildChain(isVip bool) []ChainEntry {
	model := FreeUserModel
	temp := 0.8
	topP := 0.95
	if isVip {
		model = VipUserModel
		topP = 0.8
	}
	return []ChainEntry{{
		Provider: "router",
		Model:    model,
		Config: ProviderConfig{
			Label:   "router",
			BaseURL: config.Env("ROUTER_SYSTEM_URL", RouterBaseURL),
		},
		Temperature:      temp,
		TopP:             topP,
		MaxTokens:        32000,
		PresencePenalty:  0.5,
		FrequencyPenalty: 0.2,
	}}
}

func buildOpenAIRequest(messages []byok.Message, entry ChainEntry) byok.OpenAICompatRequest {
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

	temp := entry.Temperature
	if temp == 0 {
		temp = 0.8
	}
	topP := entry.TopP
	if topP == 0 {
		topP = 0.95
	}
	maxOut := entry.MaxTokens
	if maxOut == 0 {
		maxOut = 16000
	}
	presPenalty := entry.PresencePenalty
	freqPenalty := entry.FrequencyPenalty

	return byok.OpenAICompatRequest{
		Model:            entry.Model,
		Messages:         openaiMessages,
		Temperature:      &temp,
		MaxTokens:        &maxOut,
		TopP:             &topP,
		PresencePenalty:  &presPenalty,
		FrequencyPenalty: &freqPenalty,
	}
}

func normalizeResponse(response interface{}) (map[string]interface{}, string) {
	switch v := response.(type) {
	case *byok.OpenAIResponse:
		content := ""
		if len(v.Choices) > 0 {
			content = v.Choices[0].Message.Content
		}
		data, _ := json.Marshal(v)
		var result map[string]interface{}
		json.Unmarshal(data, &result)
		return result, content
	case *byok.OpenAICompatResponse:
		content := ""
		if len(v.Choices) > 0 {
			content = v.Choices[0].Message.Content
		}
		data, _ := json.Marshal(v)
		var result map[string]interface{}
		json.Unmarshal(data, &result)
		return result, content
	case map[string]interface{}:
		content := extractContent(v)
		return v, content
	}
	return nil, ""
}

func extractContent(m map[string]interface{}) string {
	choices, ok := m["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return ""
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return ""
	}
	msg, ok := choice["message"].(map[string]interface{})
	if !ok {
		return ""
	}
	content, _ := msg["content"].(string)
	return content
}

func RunFallbackChain(messages []byok.Message, params *SystemAIChatParams, isVip bool) *ChainResult {
	chain := buildChain(isVip)
	entry := chain[0]

	pings := make([]PingLog, 0)

	apiKey := config.Env("ROUTER_SYSTEM_KEY", "")
	if apiKey == "" {
		return &ChainResult{
			StatusCode: 503,
			Error:      "router API key not configured",
			Pings: []PingLog{{
				Provider: entry.Provider,
				Model:    entry.Model,
				Status:   "error",
				Detail:   "no API key configured",
			}},
		}
	}

	reqPayload := buildOpenAIRequest(messages, entry)
	openaiResp, code, err := byok.CallOpenAI(apiKey, entry.Config.BaseURL, entry.Model, reqPayload, RequestTimeout, nil)
	if err != nil {
		pings = append(pings, PingLog{
			Provider: entry.Provider,
			Model:    entry.Model,
			Status:   "error",
			Detail:   err.Error(),
		})
		return &ChainResult{
			StatusCode: code,
			Error:      fmt.Sprintf("router request failed: %v", err),
			Pings:      pings,
		}
	}

	normalized, content := normalizeResponse(openaiResp)
	if content == "" {
		pings = append(pings, PingLog{
			Provider: entry.Provider,
			Model:    entry.Model,
			Status:   "error",
			Detail:   "empty response content",
		})
		return &ChainResult{
			StatusCode: code,
			Error:      "router returned empty response content",
			Pings:      pings,
		}
	}

	pings = append(pings, PingLog{
		Provider: entry.Provider,
		Model:    entry.Model,
		Status:   "success",
	})
	return &ChainResult{
		Response:   normalized,
		Content:    content,
		StatusCode: code,
		Pings:      pings,
	}
}
