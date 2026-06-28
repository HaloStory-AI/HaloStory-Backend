package ai

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/services"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

type FallbackEntry struct {
	Provider string
	Model    string
	Force    string
}

type FallbackChainService struct {
	cfg    *config.Config
	cache  *services.CacheService
	client *resty.Client
	logger *zap.Logger
}

func NewFallbackChainService(cfg *config.Config, cache *services.CacheService) *FallbackChainService {
	logger, _ := zap.NewProduction()

	return &FallbackChainService{
		cfg:    cfg,
		cache:  cache,
		client: resty.New().SetTimeout(cfg.AIRequestTimeout),
		logger: logger,
	}
}

func (s *FallbackChainService) GetSystemAPIKey(provider string, isPriority bool) string {
	keyName := ""
	switch provider {
	case "Gemini":
		if isPriority {
			keyName = "gemini_priority"
		} else {
			keyName = "gemini"
		}
	case "GeminiPriority":
		keyName = "gemini_priority"
	case "Groq":
		keyName = "groq"
	case "OpenRouter":
		keyName = "openrouter"
	case "MistralAI":
		keyName = "mistral"
	case "Cohere":
		keyName = "cohere"
	case "Cerebras":
		keyName = "cerebras"
	case "Huashang":
		keyName = "huashang"
	case "HuashangPriority":
		keyName = "huashang_priority"
	default:
		return ""
	}

	keysStr := s.cfg.AISystemKeys[keyName]
	if keysStr == "" {
		return ""
	}

	keys := strings.Split(keysStr, ",")
	if len(keys) == 0 {
		return ""
	}

	if isPriority && (provider == "GeminiPriority" || provider == "HuashangPriority") {
		idx, err := s.cache.GetKeyIndex(provider)
		if err != nil {
			idx = 0
		}
		if idx < 0 || idx >= len(keys) {
			idx = 0
		}
		return strings.TrimSpace(keys[idx])
	}

	return strings.TrimSpace(keys[rand.Intn(len(keys))])
}

func (s *FallbackChainService) RotateProviderKey(provider string) {
	if provider == "GeminiPriority" || provider == "HuashangPriority" {
		s.cache.RotateKeyIndex(provider)
	}
}

func (s *FallbackChainService) IsCircuitBroken(provider string) bool {
	return s.cache.IsCircuitBroken(provider)
}

func (s *FallbackChainService) RecordFailure(provider string) {
	s.cache.IncrCircuitFailure(provider)
}

func (s *FallbackChainService) ResetCircuit(provider string) {
	s.cache.ResetCircuit(provider)
}

func (s *FallbackChainService) GetBaseURL(provider string) string {
	switch provider {
	case "OpenRouter":
		return "https://openrouter.ai/api/v1/chat/completions"
	case "Groq":
		return "https://api.groq.com/openai/v1/chat/completions"
	case "Huashang", "HuashangPriority":
		return "https://aihub.071129.xyz/v1/chat/completions"
	case "Gemini", "GeminiPriority":
		return "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
	case "MistralAI":
		return "https://api.mistral.ai/v1/chat/completions"
	case "Cohere":
		return "https://api.cohere.com/v2/chat"
	case "Cerebras":
		return "https://api.cerebras.ai/v1/chat/completions"
	default:
		return ""
	}
}

func (s *FallbackChainService) GetHeaders(provider string, apiKey string) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	authPrefix := "Bearer "
	if provider == "Cohere" {
		authPrefix = "bearer "
	}

	headers["Authorization"] = authPrefix + apiKey

	if provider == "OpenRouter" {
		headers["HTTP-Referer"] = s.cfg.AppURL
		headers["X-Title"] = s.cfg.AppName
	}

	return headers
}

func (s *FallbackChainService) SendChat(prompt string, systemPrompt string, chain []FallbackEntry, opts map[string]interface{}, isPriority bool) (map[string]interface{}, string, error) {
	for _, entry := range chain {
		if s.IsCircuitBroken(entry.Provider) {
			s.logger.Debug("circuit broken, skipping", zap.String("provider", entry.Provider))
			continue
		}

		apiKey := s.GetSystemAPIKey(entry.Provider, isPriority)
		if apiKey == "" {
			s.logger.Debug("no api key", zap.String("provider", entry.Provider))
			continue
		}

		messages := []map[string]interface{}{}
		if systemPrompt != "" {
			messages = append(messages, map[string]interface{}{
				"role":    "system",
				"content": systemPrompt,
			})
		}
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": prompt,
		})

		payload := map[string]interface{}{
			"model":    entry.Model,
			"messages": messages,
			"stream":   true,
		}

		for k, v := range opts {
			payload[k] = v
		}

		baseURL := s.GetBaseURL(entry.Provider)
		if baseURL == "" {
			continue
		}

		isGeminiNative := strings.Contains(baseURL, "generativelanguage.googleapis.com") ||
			strings.Contains(entry.Provider, "Gemini")

		var result map[string]interface{}

		if isGeminiNative && !strings.Contains(baseURL, "openai") {
			result = ProxyGeminiNativeStream(entry.Model, payload, apiKey, s.cfg.AIStreamTimeout)
		} else {
			var err error
			result, err = s.tryProviderHTTP(baseURL, s.GetHeaders(entry.Provider, apiKey), payload, entry.Provider, entry.Model)
			if err != nil {
				s.RecordFailure(entry.Provider)
				s.RotateProviderKey(entry.Provider)
				continue
			}
		}

		if !IsValidAIResponse(result) {
			s.RecordFailure(entry.Provider)
			s.RotateProviderKey(entry.Provider)
			continue
		}

		result = SanitizeAIResponse(result, entry.Model)
		s.ResetCircuit(entry.Provider)

		return result, entry.Model, nil
	}

	return nil, "", fmt.Errorf("all providers failed")
}

func (s *FallbackChainService) tryProviderHTTP(url string, headers map[string]string, payload map[string]interface{}, provider string, model string) (map[string]interface{}, error) {
	timeout := s.cfg.AIStreamTimeout

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.R().
		SetHeaders(headers).
		SetBody(string(payloadBytes)).
		SetDoNotParseResponse(true).
		Post(url)

	if err != nil {
		return nil, err
	}
	defer resp.RawBody().Close()

	body := resp.RawBody()
	content, err := ConsumeSSEStream(body, provider, timeout)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": content,
		"model":   model,
	}, nil
}

func (s *FallbackChainService) GetFallbackChain() []FallbackEntry {
	return []FallbackEntry{
		{Provider: "Huashang", Model: "mimo-v2.5-pro", Force: ""},
		{Provider: "Huashang", Model: "mimo-v2.5", Force: ""},
		{Provider: "Gemini", Model: "gemma-4-31b-it", Force: ""},
		{Provider: "Gemini", Model: "gemma-4-26b-a4b-it", Force: ""},
		{Provider: "MistralAI", Model: "ministral-14b", Force: ""},
		{Provider: "MistralAI", Model: "ministral-8b", Force: ""},
		{Provider: "MistralAI", Model: "ministral-3b", Force: ""},
		{Provider: "Gemini", Model: "gemini-1.5-flash-lite", Force: ""},
		{Provider: "Gemini", Model: "gemini-1.5-flash", Force: ""},
		{Provider: "Gemini", Model: "gemini-2.0-flash", Force: ""},
		{Provider: "Gemini", Model: "gemini-2.5-flash-lite", Force: ""},
		{Provider: "Gemini", Model: "gemini-2.5-flash", Force: ""},
		{Provider: "Groq", Model: "llama-3.3-70b-versatile", Force: ""},
		{Provider: "Groq", Model: "llama-3.1-8b-instant", Force: ""},
		{Provider: "Groq", Model: "llama-4-scout-17b-16e-instruct", Force: ""},
		{Provider: "Groq", Model: "groq-llama-3.3-70b-versatile", Force: "compound"},
		{Provider: "OpenRouter", Model: "meta-llama/llama-3.3-70b-instruct", Force: ""},
		{Provider: "OpenRouter", Model: "meta-llama/llama-3.2-11b-vision-instruct", Force: ""},
		{Provider: "OpenRouter", Model: "meta-llama/llama-3.2-1b-instruct", Force: ""},
		{Provider: "OpenRouter", Model: "mistralai/mistral-small-3.1-24b-instruct", Force: ""},
	}
}

func (s *FallbackChainService) GetPriorityFallbackChain() []FallbackEntry {
	chain := []FallbackEntry{
		{Provider: "HuashangPriority", Model: "mimo-v2.5-pro", Force: ""},
		{Provider: "HuashangPriority", Model: "mimo-v2.5", Force: ""},
	}

	for _, m := range s.cfg.AIPriorityModels {
		m = strings.TrimSpace(m)
		if m != "" {
			chain = append(chain, FallbackEntry{Provider: "GeminiPriority", Model: m, Force: ""})
		}
	}

	return chain
}

// BatchSummarize handles the summarization flow using fallback chain
func (s *FallbackChainService) BatchSummarize(summarizationPrompt string, isPriority bool) (string, error) {
	opts := map[string]interface{}{
		"temperature": 0.7,
		"max_tokens":  500,
	}

	var chain []FallbackEntry
	if isPriority {
		chain = s.GetPriorityFallbackChain()
	} else {
		chain = s.GetFallbackChain()
	}

	result, _, err := s.SendChat(summarizationPrompt, "", chain, opts, isPriority)
	if err != nil {
		return "", err
	}

	if content, ok := result["content"].(string); ok {
		return content, nil
	}

	return "", fmt.Errorf("summarization returned no content")
}
