package byok

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Model struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	ContextWindow *int   `json:"context_window,omitempty"`
}

type ServiceError struct {
	Code    int
	Message string
	Details interface{}
}

func FetchModels(apiKey, provider, baseURL string) ([]Model, *ServiceError) {
	switch provider {
	case "gemini":
		return fetchGeminiModels(apiKey)
	case "claude":
		return fetchClaudeModels(apiKey)
	case "openrouter":
		return fetchOpenAICompatibleModels(apiKey, "https://openrouter.ai/api/v1/models")
	case "deepseek":
		return fetchOpenAICompatibleModels(apiKey, "https://api.deepseek.com/v1/models")
	case "groq":
		return fetchOpenAICompatibleModels(apiKey, "https://api.groq.com/openai/v1/models")
	case "mistral":
		return fetchOpenAICompatibleModels(apiKey, "https://api.mistral.ai/v1/models")
	case "featherless":
		return fetchOpenAICompatibleModels(apiKey, "https://api.featherless.ai/v1/models")
	case "nanogpt":
		return fetchOpenAICompatibleModels(apiKey, "https://nano-gpt.com/api/v1/models")
	case "venus":
		return fetchOpenAICompatibleModels(apiKey, "https://api.venus.chat/v1/models")
	case "zai", "z.ai":
		return fetchOpenAICompatibleModels(apiKey, "https://api.z.ai/api/paas/v4/models")
	case "chutes":
		if baseURL == "" {
			baseURL = "https://chutes.ai/v1"
		}
		return fetchOpenAICompatibleModels(apiKey, strings.TrimRight(baseURL, "/")+"/models")
	case "openai-compatible":
		if baseURL == "" {
			return nil, &ServiceError{Code: 400, Message: "base_url is required for Custom OpenAI Compatible"}
		}
		return fetchOpenAICompatibleModels(apiKey, strings.TrimRight(baseURL, "/")+"/models")
	case "arli", "arliai":
		return fetchArliModels()
	case "mancer":
		return fetchMancerModels()
	case "horde":
		return fetchHordeModels()
	case "novel", "novelai":
		return fetchNovelModels()
	default:
		return nil, &ServiceError{
			Code:    400,
			Message: fmt.Sprintf("The provider '%s' is not supported for dynamic model fetching.", provider),
		}
	}
}

func fetchGeminiModels(apiKey string) ([]Model, *ServiceError) {
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", "https://generativelanguage.googleapis.com/v1beta/models", nil)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to create request"}
	}
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to fetch from Gemini API"}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to read response"}
	}

	if resp.StatusCode != 200 {
		return nil, &ServiceError{Code: resp.StatusCode, Message: "Gemini API error", Details: string(body)}
	}

	var data struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			InputTokenLimit            *int     `json:"inputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to parse Gemini response"}
	}

	var models []Model
	for _, m := range data.Models {
		hasGenerateContent := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				hasGenerateContent = true
				break
			}
		}
		if !hasGenerateContent {
			continue
		}

		id := strings.Replace(m.Name, "models/", "", 1)
		model := Model{
			ID:          id,
			Name:        m.DisplayName,
			Description: m.Description,
		}
		if m.InputTokenLimit != nil {
			model.ContextWindow = m.InputTokenLimit
		}
		models = append(models, model)
	}

	if models == nil {
		models = make([]Model, 0)
	}
	return models, nil
}

func fetchClaudeModels(apiKey string) ([]Model, *ServiceError) {
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", "https://api.anthropic.com/v1/models", nil)
	if err != nil {
		return fetchHardcodedClaudeModels()
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		return fetchHardcodedClaudeModels()
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fetchHardcodedClaudeModels()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fetchHardcodedClaudeModels()
	}

	var data struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return fetchHardcodedClaudeModels()
	}

	var models []Model
	for _, m := range data.Data {
		name := m.DisplayName
		if name == "" {
			name = m.ID
		}
		models = append(models, Model{ID: m.ID, Name: name})
	}
	if models == nil {
		models = make([]Model, 0)
	}
	return models, nil
}

func fetchHardcodedClaudeModels() ([]Model, *ServiceError) {
	return []Model{
		{ID: "claude-3-7-sonnet-latest", Name: "Claude 3.7 Sonnet", ContextWindow: intPtr(200000)},
		{ID: "claude-3-5-sonnet-latest", Name: "Claude 3.5 Sonnet", ContextWindow: intPtr(200000)},
		{ID: "claude-3-5-haiku-latest",  Name: "Claude 3.5 Haiku",  ContextWindow: intPtr(200000)},
		{ID: "claude-3-opus-latest",     Name: "Claude 3 Opus",     ContextWindow: intPtr(200000)},
	}, nil
}

func fetchOpenAICompatibleModels(apiKey, url string) ([]Model, *ServiceError) {
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to create request"}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	if strings.Contains(url, "mancer.tech") {
		req.Header.Set("X-API-KEY", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to fetch from API"}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to read response"}
	}

	if resp.StatusCode != 200 {
		return nil, &ServiceError{Code: resp.StatusCode, Message: "API error", Details: string(body)}
	}

	var data struct {
		Data []struct {
			ID           string `json:"id"`
			Name         string `json:"name,omitempty"`
			Description  string `json:"description,omitempty"`
			ContextLength *int  `json:"context_length,omitempty"`
			MaxTokens    *int  `json:"max_tokens,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to parse response"}
	}

	var models []Model
	for _, m := range data.Data {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		model := Model{
			ID:   m.ID,
			Name: name,
		}
		if m.ContextLength != nil {
			model.ContextWindow = m.ContextLength
		} else if m.MaxTokens != nil {
			model.ContextWindow = m.MaxTokens
		}
		models = append(models, model)
	}
	if models == nil {
		models = make([]Model, 0)
	}
	return models, nil
}

func fetchArliModels() ([]Model, *ServiceError) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get("https://api.arliai.com/model/all")
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to fetch ArliAI models"}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to read response"}
	}

	if resp.StatusCode != 200 {
		return nil, &ServiceError{Code: resp.StatusCode, Message: "ArliAI API error"}
	}

	var data []struct {
		Name        string `json:"name"`
		Status      bool   `json:"status"`
		ModelSize   string `json:"modelSize,omitempty"`
		ContextSize *int   `json:"contextSize,omitempty"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to parse ArliAI response"}
	}

	var models []Model
	for _, m := range data {
		if !m.Status {
			continue
		}
		model := Model{
			ID:          m.Name,
			Name:        m.Name,
			Description: m.ModelSize,
		}
		if m.ContextSize != nil {
			model.ContextWindow = m.ContextSize
		}
		models = append(models, model)
	}
	if models == nil {
		models = make([]Model, 0)
	}
	return models, nil
}

func fetchMancerModels() ([]Model, *ServiceError) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get("https://mancer.tech/internal/api/models")
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to fetch Mancer models"}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to read response"}
	}

	if resp.StatusCode != 200 {
		return nil, &ServiceError{Code: resp.StatusCode, Message: "Mancer API error"}
	}

	var data struct {
		Models []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			Description    string `json:"description,omitempty"`
			Context        *int   `json:"context,omitempty"`
			PerTokenOutput string `json:"perTokenOutput,omitempty"`
			PerTokenPrompt string `json:"perTokenPrompt,omitempty"`
			PaidOnly       bool   `json:"paidOnly,omitempty"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to parse Mancer response"}
	}

	var models []Model
	for _, m := range data.Models {
		costInfo := fmt.Sprintf("(%s/out + %s/in)", m.PerTokenOutput, m.PerTokenPrompt)
		paidMarker := ""
		if m.PaidOnly {
			paidMarker = "[PAID] "
		}
		model := Model{
			ID:          m.ID,
			Name:        paidMarker + m.Name + " " + costInfo,
			Description: m.Description,
		}
		if m.Context != nil {
			model.ContextWindow = m.Context
		}
		models = append(models, model)
	}
	if models == nil {
		models = make([]Model, 0)
	}
	return models, nil
}

func fetchHordeModels() ([]Model, *ServiceError) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get("https://stablehorde.net/api/v2/status/models")
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to fetch AI Horde models"}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to read response"}
	}

	if resp.StatusCode != 200 {
		return nil, &ServiceError{Code: resp.StatusCode, Message: "AI Horde API error"}
	}

	var data []struct {
		Name        string  `json:"name"`
		Type        string  `json:"type,omitempty"`
		Count       int     `json:"count"`
		Queued      int     `json:"queued"`
		Performance float64 `json:"performance"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, &ServiceError{Code: 500, Message: "Failed to parse AI Horde response"}
	}

	var models []Model
	for _, m := range data {
		if m.Type != "" && m.Type != "text" {
			continue
		}
		desc := fmt.Sprintf("Queued: %d / Performance: %.0f", m.Queued, m.Performance)
		models = append(models, Model{
			ID:          m.Name,
			Name:        fmt.Sprintf("%s (Workers: %d)", m.Name, m.Count),
			Description: desc,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Name > models[j].Name
	})

	if models == nil {
		models = make([]Model, 0)
	}
	return models, nil
}

func fetchNovelModels() ([]Model, *ServiceError) {
	return []Model{
		{ID: "llama-3-erato-v1", Name: "Llama 3 Erato", ContextWindow: intPtr(8192)},
		{ID: "kayra-v1",        Name: "Kayra",          ContextWindow: intPtr(8192)},
		{ID: "clio-v1",         Name: "Clio",           ContextWindow: intPtr(8192)},
		{ID: "krake-v2",        Name: "Krake (Legacy)", ContextWindow: intPtr(2048)},
		{ID: "euterpe-v2",      Name: "Euterpe (Legacy)", ContextWindow: intPtr(2048)},
	}, nil
}

func intPtr(i int) *int { return &i }
