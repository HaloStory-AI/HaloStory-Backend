package ai

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type BYOKRequest struct {
	Messages         []Message `json:"messages"`
	APIKey           string    `json:"api_key"`
	Model            string    `json:"model"`
	BaseURL          *string   `json:"base_url,omitempty"`
	Temperature      *float64  `json:"temperature,omitempty"`
	TopP             *float64  `json:"top_p,omitempty"`
	TopA             *float64  `json:"top_a,omitempty"`
	MinP             *float64  `json:"min_p,omitempty"`
	FrequencyPenalty *float64  `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64  `json:"presence_penalty,omitempty"`
	RepetitionPenalty *float64 `json:"repetition_penalty,omitempty"`
	MaxTokens        *int      `json:"max_tokens,omitempty"`
	ResponseLength   *int      `json:"response_length,omitempty"`
	TopK             *int      `json:"top_k,omitempty"`
	HistoryDepth     *int      `json:"history_depth,omitempty"`
	BotID            *string   `json:"bot_id,omitempty"`
	Language         *string   `json:"language,omitempty"`
	Summary          *string   `json:"summary,omitempty"`
	IsDisableBond    *bool     `json:"isdisablebond,omitempty"`
	IsGroup          *bool     `json:"is_group,omitempty"`
	BotIDs           []string  `json:"bot_ids,omitempty"`
	UserPersona      *string   `json:"user_persona,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const (
	maxAPIKeyLength    = 1024
	maxModelLength     = 128
	maxBaseURLLength   = 512
	contentControlChars = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f"
)

func SanitizeBYOKInput(c *fiber.Ctx) (*BYOKRequest, *fiber.Map) {
	var req BYOKRequest
	if err := c.BodyParser(&req); err != nil {
		return nil, &fiber.Map{"error": "Invalid request body", "message": "Unable to parse request"}
	}

	if len(req.APIKey) > maxAPIKeyLength {
		return nil, byokError("API key is too long (max 1024 characters)")
	}

	if len(req.Model) > maxModelLength {
		return nil, byokError("Model is too long (max 128 characters)")
	}
	if hasControlChars(req.Model) {
		return nil, byokError("Model contains invalid characters")
	}

	if req.BaseURL != nil {
		if len(*req.BaseURL) > maxBaseURLLength {
			return nil, byokError("Base URL is too long (max 512 characters)")
		}
		u, err := url.Parse(*req.BaseURL)
		if err != nil || u.Scheme != "https" {
			return nil, byokError("Base URL must be a valid HTTPS URL")
		}
	}

	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return nil, byokError("Temperature must be between 0 and 2")
	}
	if req.TopP != nil && (*req.TopP < 0 || *req.TopP > 1) {
		return nil, byokError("Top P must be between 0 and 1")
	}
	if req.TopA != nil && (*req.TopA < 0 || *req.TopA > 1) {
		return nil, byokError("Top A must be between 0 and 1")
	}
	if req.MinP != nil && (*req.MinP < 0 || *req.MinP > 1) {
		return nil, byokError("Min P must be between 0 and 1")
	}
	if req.FrequencyPenalty != nil && (*req.FrequencyPenalty < -2 || *req.FrequencyPenalty > 2) {
		return nil, byokError("Frequency penalty must be between -2 and 2")
	}
	if req.PresencePenalty != nil && (*req.PresencePenalty < -2 || *req.PresencePenalty > 2) {
		return nil, byokError("Presence penalty must be between -2 and 2")
	}
	if req.RepetitionPenalty != nil && (*req.RepetitionPenalty < 1 || *req.RepetitionPenalty > 2) {
		return nil, byokError("Repetition penalty must be between 1 and 2")
	}

	if req.MaxTokens != nil && (*req.MaxTokens < 1 || *req.MaxTokens > 128000) {
		return nil, byokError("Max tokens must be between 1 and 128000")
	}
	if req.ResponseLength != nil && (*req.ResponseLength < 1 || *req.ResponseLength > 32000) {
		return nil, byokError("Response length must be between 1 and 32000")
	}
	if req.TopK != nil && (*req.TopK < 0 || *req.TopK > 500) {
		return nil, byokError("Top K must be between 0 and 500")
	}
	if req.HistoryDepth != nil && (*req.HistoryDepth < 1 || *req.HistoryDepth > 500) {
		return nil, byokError("History depth must be between 1 and 500")
	}

	if len(req.Messages) == 0 {
		return nil, byokError("Messages array is required")
	}
	for _, msg := range req.Messages {
		role := strings.ToLower(msg.Role)
		if role != "user" && role != "assistant" && role != "system" {
			return nil, byokError("Invalid message role")
		}
		if hasControlChars(msg.Content) {
			return nil, byokError("Message content contains invalid characters")
		}
	}

	return &req, nil
}

func byokError(msg string) *fiber.Map {
	return &fiber.Map{"error": "Validation failed", "message": msg}
}

type ChatResult struct {
	Content string `json:"content"`
	Model   string `json:"model"`
	Provider string `json:"provider"`
}

type BYOKProviderConfig struct {
	Name         string
	BaseURL      string
	AuthHeader   func(key string) string
	BuildPayload func(req BYOKRequest, messagesJSON string) map[string]interface{}
	ParseResponse func(body []byte) (*ChatResult, error)
}

// BYOK provider configurations
var BYOKProviders = map[string]BYOKProviderConfig{
	"gemini": {
		Name:    "Gemini",
		BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"claude": {
		Name:    "Claude",
		BaseURL: "https://api.anthropic.com/v1/messages",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"deepseek": {
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"groq": {
		Name:    "Groq",
		BaseURL: "https://api.groq.com/openai/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"mistral": {
		Name:    "Mistral AI",
		BaseURL: "https://api.mistral.ai/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"openrouter": {
		Name:    "OpenRouter",
		BaseURL: "https://openrouter.ai/api/v1/chat/completions",
		AuthHeader: func(key string) string {
			return "Bearer " + key
		},
	},
	"arli": {
		Name:    "ArliAI",
		BaseURL: "https://api.arliai.com/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"mancer": {
		Name:    "Mancer",
		BaseURL: "https://mancer.tech/api/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"featherless": {
		Name:    "Featherless",
		BaseURL: "https://api.featherless.ai/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"chutes": {
		Name:    "Chutes",
		BaseURL: "https://api.chutes.ai/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"nanogpt": {
		Name:    "NanoGPT",
		BaseURL: "https://api.nanogpt.dev/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"venus": {
		Name:    "Venus AI",
		BaseURL: "https://api.venus.ai/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"zai": {
		Name:    "Z.ai",
		BaseURL: "https://api.z.ai/v1/chat/completions",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"novel": {
		Name:    "NovelAI",
		BaseURL: "https://api.novelai.net/ai/generate",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"horde": {
		Name:    "AI Horde",
		BaseURL: "https://stablehorde.net/api/v2/generate/text/async",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
	"openai-compatible": {
		Name:    "OpenAI Compatible",
		BaseURL: "",
		AuthHeader: func(key string) string { return "Bearer " + key },
	},
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			return true
		}
	}
	return false
}

func init() {
	// Prevent json unused import
	_ = json.Marshal
}
