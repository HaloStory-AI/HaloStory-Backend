package systemai

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/byok"
)

func getSystemApiKeys(label string) []string {
	envKey, ok := ProviderEnvKeys[label]
	if !ok {
		return nil
	}
	raw := config.Env(envKey, "")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	return keys
}

func selectKey(keys []string) (string, int) {
	if len(keys) == 0 {
		return "", 0
	}
	idx := rand.Intn(len(keys))
	return keys[idx], idx
}

func rotateKey(provider string, currentIdx int) {
	idxKey := fmt.Sprintf("vip_key_index_%s", provider)
	services.RedisClient.SetEx(services.RedisCtx, idxKey, int64(currentIdx+1), time.Hour)
}

func isProviderCircuitBroken(provider string) bool {
	key := fmt.Sprintf("ai_circuit_failures_%s", provider)
	val, err := services.RedisClient.Get(services.RedisCtx, key).Int()
	if err != nil {
		return false
	}
	return val >= CircuitBreakerLimit
}

func recordProviderFailure(provider string) {
	key := fmt.Sprintf("ai_circuit_failures_%s", provider)
	val, err := services.RedisClient.Get(services.RedisCtx, key).Int()
	if err != nil {
		val = 0
	}
	services.RedisClient.SetEx(services.RedisCtx, key, val+1, CircuitBreakerWindow)
}

func resetProviderCircuit(provider string) {
	key := fmt.Sprintf("ai_circuit_failures_%s", provider)
	services.RedisClient.Del(services.RedisCtx, key)
}

func buildPriorityChain(isVip bool) []ChainEntry {
	if !isVip {
		return RegularChain
	}
	priorityEntries := make([]ChainEntry, 0)
	priorityKeys := getSystemApiKeys("huashang_priority")
	if len(priorityKeys) > 0 {
		priorityEntries = append(priorityEntries,
			ChainEntry{Provider: "huashang_priority", Model: "deepseek-ai/deepseek-v4-flash", Config: ProviderConfig{Label: "huashang_priority", BaseURL: "https://api.huashang.ai/v1"}},
		)
	}
	geminiPriorityKeys := getSystemApiKeys("gemini_priority")
	if len(geminiPriorityKeys) > 0 {
		for _, m := range PriorityModels {
			priorityEntries = append(priorityEntries, ChainEntry{Provider: "gemini_priority", Model: m, Config: ProviderConfig{Label: "gemini_priority", IsGemini: true}})
		}
	}
	if len(priorityEntries) == 0 {
		return RegularChain
	}
	chain := make([]ChainEntry, 0, len(priorityEntries)+len(RegularChain))
	chain = append(chain, priorityEntries...)
	chain = append(chain, RegularChain...)
	return chain
}

func buildGeminiRequest(messages []byok.Message, model string, params *SystemAIChatParams) byok.GeminiRequest {
	contents := make([]byok.GeminiContent, 0)
	var systemInstruct *byok.GeminiContent
	systemText := ""

	isGemma := strings.Contains(model, "gemma")
	isModernGemma := false
	if len(model) >= 10 {
		lastChar := model[len(model)-1]
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

	temp := DefaultTemperature
	if params.Temperature != 0 {
		temp = params.Temperature
	}
	topP := 1.0
	if params.TopP != 0 {
		topP = params.TopP
	}
	topK := 40
	if params.TopK != 0 {
		topK = params.TopK
	}
	maxOut := DefaultResponseLen
	if params.ResponseLength != 0 {
		maxOut = params.ResponseLength
	} else if params.MaxTokens != 0 {
		maxOut = params.MaxTokens
	}

	genConfig := byok.GenerationConfig{
		Temperature:     temp,
		TopP:            topP,
		TopK:            topK,
		MaxOutputTokens: maxOut,
		StopSequences:   params.Stop,
	}

	if isGemma {
		if genConfig.ThinkingConfig == nil {
			genConfig.ThinkingConfig = &byok.ThinkingConfig{}
		}
		genConfig.ThinkingConfig.ThinkingLevel = "MINIMAL"
	}

	req := byok.GeminiRequest{
		Contents:       contents,
		GenerationConfig: genConfig,
		SafetySettings: byok.DefaultSafetySettings,
	}
	if systemInstruct != nil {
		req.SystemInstruction = systemInstruct
	}
	return req
}

func buildOpenAIRequest(messages []byok.Message, model string, params *SystemAIChatParams) byok.OpenAICompatRequest {
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
	if params.Temperature != 0 {
		temp = params.Temperature
	}
	topP := 1.0
	if params.TopP != 0 {
		topP = params.TopP
	}
	maxOut := DefaultResponseLen
	if params.ResponseLength != 0 {
		maxOut = params.ResponseLength
	} else if params.MaxTokens != 0 {
		maxOut = params.MaxTokens
	}

	return byok.OpenAICompatRequest{
		Model:       model,
		Messages:    openaiMessages,
		Temperature: &temp,
		MaxTokens:   &maxOut,
		TopP:        &topP,
		Stop:        params.Stop,
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
	result := &ChainResult{
		Pings: make([]PingLog, 0),
	}
	startTime := time.Now()
	chain := buildPriorityChain(isVip)

	for _, entry := range chain {
		if time.Since(startTime) > ChainTimeout {
			result.Pings = append(result.Pings, PingLog{
				Provider: entry.Provider,
				Model:    entry.Model,
				Status:   "skipped",
				Detail:   "chain timeout exceeded",
			})
			break
		}

		label := entry.Config.Label

		keys := getSystemApiKeys(label)
		if len(keys) == 0 {
			result.Pings = append(result.Pings, PingLog{
				Provider: entry.Provider,
				Model:    entry.Model,
				Status:   "skipped",
				Detail:   "no API keys configured",
			})
			continue
		}

		if isProviderCircuitBroken(entry.Provider) {
			result.Pings = append(result.Pings, PingLog{
				Provider: entry.Provider,
				Model:    entry.Model,
				Status:   "skipped",
				Detail:   "circuit breaker open",
			})
			continue
		}

		for attempt := 0; attempt < len(keys); attempt++ {
			var key string
			var idx int
			if isVip {
				idx = attempt
				key = keys[idx%len(keys)]
			} else {
				key, idx = selectKey(keys)
			}

			var resp interface{}
			var statusCode int
			var callErr error

			if entry.Config.IsGemini {
				req := buildGeminiRequest(messages, entry.Model, params)
				geminiResp, code, err := byok.CallGemini(key, entry.Model, req)
				statusCode = code
				callErr = err
				if err == nil {
					resp = byok.ConvertToOpenAIFormat(geminiResp, entry.Model)
				}
			} else {
				baseURL := entry.Config.BaseURL
				if baseURL == "" {
					if u, ok := ProviderBaseURLs[entry.Provider]; ok {
						baseURL = u
					} else {
						result.Pings = append(result.Pings, PingLog{
							Provider: entry.Provider,
							Model:    entry.Model,
							Status:   "error",
							Detail:   "no base URL for provider",
						})
						continue
					}
				}
				reqPayload := buildOpenAIRequest(messages, entry.Model, params)
				openaiResp, code, err := byok.CallOpenAI(key, baseURL, entry.Model, reqPayload, RequestTimeout, nil)
				statusCode = code
				callErr = err
				if err == nil {
					resp = openaiResp
				}
			}

			if callErr == nil && resp != nil {
				normalized, content := normalizeResponse(resp)
				if content == "" {
					result.Pings = append(result.Pings, PingLog{
						Provider: entry.Provider,
						Model:    entry.Model,
						Status:   "error",
						Detail:   "empty response content",
					})
					continue
				}
				resetProviderCircuit(entry.Provider)
				if isVip {
					rotateKey(entry.Provider, idx)
				}
				result.Response = normalized
				result.Content = content
				result.StatusCode = statusCode
				result.Pings = append(result.Pings, PingLog{
					Provider: entry.Provider,
					Model:    entry.Model,
					Status:   "success",
				})
				return result
			}

			errMsg := ""
			if callErr != nil {
				errMsg = callErr.Error()
			}
			recordProviderFailure(entry.Provider)
			result.Pings = append(result.Pings, PingLog{
				Provider: entry.Provider,
				Model:    entry.Model,
				Status:   "error",
				Detail:   fmt.Sprintf("attempt %d: HTTP %d - %s", attempt+1, statusCode, errMsg),
			})
			time.Sleep(300 * time.Millisecond)
		}
	}

	result.Error = "All system AI models failed"
	result.StatusCode = 503
	return result
}
