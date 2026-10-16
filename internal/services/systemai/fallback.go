package systemai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"
)

func buildChain(isVip bool) ([]ChainEntry, error) {
	model, msg := RequireNvidiaModel(isVip)
	if msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	temp := 0.8
	topP := 0.95
	if isVip {
		topP = 0.8
	}
	return []ChainEntry{{
		Provider: "nvidia",
		Model:    model,
		Config: ProviderConfig{
			Label:   "nvidia",
			BaseURL: config.Env("NVIDIA_SYSTEM_URL", NvidiaBaseURL),
		},
		Temperature:      temp,
		TopP:             topP,
		MaxTokens:        32000,
		PresencePenalty:  0.5,
		FrequencyPenalty: 0.2,
	}}, nil
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
		var content interface{}
		if msg.Content.HasImage() {
			parts := make([]byok.ContentPart, 0, len(msg.Content.Parts()))
			for _, p := range msg.Content.Parts() {
				parts = append(parts, p)
			}
			content = parts
		} else {
			content = msg.Content.AsText()
		}
		openaiMessages = append(openaiMessages, byok.OpenAIChatMessage{
			Role:    role,
			Content: content,
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
			if s, ok := v.Choices[0].Message.Content.(string); ok {
				content = s
			}
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

func RunFallbackChain(httpClient *http.Client, messages []byok.Message, params *SystemAIChatParams, isVip bool) *ChainResult {
	keys := GetNvidiaKeys()
	if len(keys) == 0 {
		model, _ := RequireNvidiaModel(isVip)
		provModel := model
		if provModel == "" {
			provModel = "nvidia"
		}
		return &ChainResult{
			StatusCode: 503,
			Error:      "nvidia keys not configured (NVIDIA_SYSTEM_KEYS)",
			Pings: []PingLog{{
				Provider: "nvidia",
				Model:    provModel,
				Status:   "error",
				Detail:   "no API key configured",
			}},
		}
	}
	chain, err := buildChain(isVip)
	if err != nil {
		return &ChainResult{
			StatusCode: 503,
			Error:      err.Error(),
			Pings: []PingLog{{
				Provider: "nvidia",
				Model:    "nvidia",
				Status:   "error",
				Detail:   err.Error(),
			}},
		}
	}
	entry := chain[0]

	pings := make([]PingLog, 0)

	apiKey := PickRandomKey(keys)

	reqPayload := buildOpenAIRequest(messages, entry)
	openaiResp, code, err := byok.CallOpenAI(httpClient, apiKey, entry.Config.BaseURL, entry.Model, reqPayload, RequestTimeout, nil)
	if err != nil {
		pings = append(pings, PingLog{
			Provider: entry.Provider,
			Model:    entry.Model,
			Status:   "error",
			Detail:   err.Error(),
		})
		return &ChainResult{
			StatusCode: code,
			Error:      fmt.Sprintf("nvidia request failed: %v", err),
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
			Error:      "nvidia returned empty response content",
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

// RunFallbackChainStream sama seperti RunFallbackChain tapi memakai SSE streaming
// ke upstream (nvidia OpenAI-compatible). Setiap delta mentah difilter via
// StreamSanitizer sebelum diteruskan ke onDelta agar tag legacy/<think> tidak
// bocor ke frontend. Hasil akhir authoritatif tetap via ParseBondTags
// post-completion di worker; onDone menerima sisa flush sanitizer.
func RunFallbackChainStream(ctx context.Context, messages []byok.Message, params *SystemAIChatParams, isVip bool, onDelta func(string) error) *ChainResult {
	keys := GetNvidiaKeys()
	if len(keys) == 0 {
		model, _ := RequireNvidiaModel(isVip)
		provModel := model
		if provModel == "" {
			provModel = "nvidia"
		}
		return &ChainResult{
			StatusCode: 503,
			Error:      "nvidia keys not configured (NVIDIA_SYSTEM_KEYS)",
			Pings: []PingLog{{
				Provider: "nvidia",
				Model:    provModel,
				Status:   "error",
				Detail:   "no API key configured",
			}},
		}
	}
	chain, errChain := buildChain(isVip)
	if errChain != nil {
		return &ChainResult{
			StatusCode: 503,
			Error:      errChain.Error(),
			Pings: []PingLog{{
				Provider: "nvidia",
				Model:    "nvidia",
				Status:   "error",
				Detail:   errChain.Error(),
			}},
		}
	}
	entry := chain[0]

	pings := make([]PingLog, 0)

	apiKey := PickRandomKey(keys)

	reqPayload := buildOpenAIRequest(messages, entry)
	sanitizer := &byok.StreamSanitizer{}
	streamClient := utils.NewAIHttpClientNoTimeout()
	filteredDelta := func(raw string) error {
		if clean := sanitizer.Feed(raw); clean != "" && onDelta != nil {
			if err := onDelta(clean); err != nil {
				return err
			}
		}
		return nil
	}
	openaiResp, code, err := byok.CallOpenAIStream(ctx, streamClient, apiKey, entry.Config.BaseURL, entry.Model, reqPayload, nil, filteredDelta)
	if err != nil {
		pings = append(pings, PingLog{
			Provider: entry.Provider,
			Model:    entry.Model,
			Status:   "error",
			Detail:   err.Error(),
		})
		return &ChainResult{
			StatusCode: code,
			Error:      fmt.Sprintf("nvidia stream request failed: %v", err),
			Pings:      pings,
		}
	}

	// Flush sisa sanitizer agar delta terakhir tidak hilang.
	if rest := sanitizer.Flush(); rest != "" && onDelta != nil {
		_ = onDelta(rest)
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
			Error:      "nvidia returned empty response content",
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
