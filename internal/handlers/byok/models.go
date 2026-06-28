package byok

import (
	"encoding/json"
	"fmt"

	"rafiq8k-moga/halostory-go-backend/internal/services"

	"github.com/gofiber/fiber/v2"
)

type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ContextWindow int    `json:"context_window"`
}

type ModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

func FetchModels(c *fiber.Ctx) error {
	var req struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
		BaseURL  string `json:"base_url,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Provider == "" || req.APIKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Provider and API key are required"})
	}

	client := services.NewHTTPClient(30)

	var models []ModelInfo
	var err error

	switch req.Provider {
	case "gemini":
		models, err = fetchGeminiModels(client, req.APIKey)
	case "claude":
		models, err = fetchClaudeModels(client, req.APIKey)
	case "arli":
		models, err = fetchArliModels(client)
	case "mancer":
		models, err = fetchMancerModels(client)
	case "horde":
		models, err = fetchHordeModels(client)
	case "novel":
		models = fetchNovelModels()
	default:
		models, err = fetchOpenAICompatibleModels(client, req.APIKey, req.BaseURL)
	}

	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   "Failed to fetch models",
			"message": err.Error(),
		})
	}

	if models == nil {
		models = []ModelInfo{}
	}

	return c.JSON(ModelsResponse{Models: models})
}

func fetchGeminiModels(client *services.HTTPClient, apiKey string) ([]ModelInfo, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)
	headers := map[string]string{"Content-Type": "application/json"}

	body, err := client.Get(url, headers)
	if err != nil {
		return nil, err
	}

	var result struct {
		Models []struct {
			Name           string `json:"name"`
			DisplayName    string `json:"displayName"`
			Description    string `json:"description"`
			SupportedMethods []string `json:"supportedMethods"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result.Models {
		hasGenerate := false
		for _, method := range m.SupportedMethods {
			if method == "generateContent" {
				hasGenerate = true
				break
			}
		}
		if !hasGenerate {
			continue
		}
		models = append(models, ModelInfo{
			ID:          m.Name,
			Name:        m.DisplayName,
			Description: m.Description,
		})
	}

	return models, nil
}

func fetchClaudeModels(client *services.HTTPClient, apiKey string) ([]ModelInfo, error) {
	url := "https://api.anthropic.com/v1/models"
	headers := map[string]string{
		"Content-Type":  "application/json",
		"x-api-key":     apiKey,
		"anthropic-version": "2023-06-01",
	}

	body, err := client.Get(url, headers)
	if err != nil {
		return getHardcodedClaudeModels(), nil
	}

	var result struct {
		Data []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return getHardcodedClaudeModels(), nil
	}

	var models []ModelInfo
	for _, m := range result.Data {
		if m.Type == "model" {
			models = append(models, ModelInfo{
				ID:   m.ID,
				Name: m.DisplayName,
			})
		}
	}

	if len(models) == 0 {
		return getHardcodedClaudeModels(), nil
	}

	return models, nil
}

func getHardcodedClaudeModels() []ModelInfo {
	return []ModelInfo{
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4"},
		{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet"},
		{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku"},
		{ID: "claude-opus-4-20250514", Name: "Claude Opus 4"},
	}
}

func fetchOpenAICompatibleModels(client *services.HTTPClient, apiKey string, baseURL string) ([]ModelInfo, error) {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := fmt.Sprintf("%s/models", baseURL)
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	body, err := client.Get(url, headers)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			ID  string `json:"id"`
			Object string `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result.Data {
		models = append(models, ModelInfo{
			ID:   m.ID,
			Name: m.ID,
		})
	}

	return models, nil
}

func fetchArliModels(client *services.HTTPClient) ([]ModelInfo, error) {
	body, err := client.Get("https://api.arliai.com/model/all", nil)
	if err != nil {
		return nil, err
	}

	var result []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result {
		models = append(models, ModelInfo{
			ID:   m.ID,
			Name: m.ID,
		})
	}

	return models, nil
}

func fetchMancerModels(client *services.HTTPClient) ([]ModelInfo, error) {
	body, err := client.Get("https://mancer.tech/internal/api/models", nil)
	if err != nil {
		return nil, err
	}

	var result []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result {
		models = append(models, ModelInfo{
			ID:   m.ID,
			Name: m.ID,
		})
	}

	return models, nil
}

func fetchHordeModels(client *services.HTTPClient) ([]ModelInfo, error) {
	body, err := client.Get("https://stablehorde.net/api/v2/status/models", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for k, v := range result {
		if entry, ok := v.(map[string]interface{}); ok {
			if entry["type"] == "text" {
				models = append(models, ModelInfo{
					ID:   k,
					Name: k,
				})
			}
		}
	}

	return models, nil
}

func fetchNovelModels() []ModelInfo {
	return []ModelInfo{
		{ID: "llama-3-erato-v2", Name: "Llama 3 Erato v2"},
		{ID: "kayra-v1", Name: "Kayra v1"},
		{ID: "clio-v1", Name: "Clio v1"},
		{ID: "krake-v2", Name: "Krake v2"},
		{ID: "euterpe-v2", Name: "Euterpe v2"},
	}
}
