package byok

import (
	"fmt"
	"net/url"
	"regexp"
)

type ValidationError struct {
	Message string
}

type BYOKRequest struct {
	Messages         []Message        `json:"messages"`
	APIKey           string           `json:"api_key"`
	Model            string           `json:"model"`
	BaseURL          *string          `json:"base_url,omitempty"`
	Temperature      *float64         `json:"temperature,omitempty"`
	TopP             *float64         `json:"top_p,omitempty"`
	TopK             *int             `json:"top_k,omitempty"`
	MaxTokens        *int             `json:"max_tokens,omitempty"`
	ResponseLength   *int             `json:"response_length,omitempty"`
	Stop             []string         `json:"stop,omitempty"`
	PresencePenalty  *float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64         `json:"frequency_penalty,omitempty"`
	Reasoning        *ReasoningConfig `json:"reasoning,omitempty"`
	Summary          *string          `json:"summary,omitempty"`
	IsSummarize      *bool            `json:"is_summarize,omitempty"`
	BotID            *string          `json:"bot_id,omitempty"`
	Language         *string          `json:"language,omitempty"`
	HistoryDepth     *int             `json:"history_depth,omitempty"`
	IsGroup          *bool            `json:"is_group,omitempty"`
	IsDisableBond    *bool            `json:"isdisablebond,omitempty"`
	UserPersona      *string          `json:"user_persona,omitempty"`
	BotIDs           []string         `json:"bot_ids,omitempty"`
	ExistingMemories []string         `json:"existing_memories,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ReasoningConfig struct {
	Enabled   bool   `json:"enabled"`
	Effort    string `json:"effort,omitempty"`
	MaxTokens *int   `json:"max_tokens,omitempty"`
	Exclude   *bool  `json:"exclude,omitempty"`
}

var controlChars = regexp.MustCompile("[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]")

const (
	maxModelLength   = 128
	maxBaseURLLength = 512
	maxAPIKeyLength  = 1024
)

func SanitizeByokInput(req *BYOKRequest) *ValidationError {
	if len(req.APIKey) > maxAPIKeyLength {
		return &ValidationError{Message: "api_key field exceeds maximum allowed length"}
	}

	if len(req.Model) > maxModelLength {
		return &ValidationError{Message: "model field exceeds maximum allowed length"}
	}
	if controlChars.MatchString(req.Model) {
		return &ValidationError{Message: "model field contains invalid control characters"}
	}

	if req.BaseURL != nil && *req.BaseURL != "" {
		if len(*req.BaseURL) > maxBaseURLLength {
			return &ValidationError{Message: "base_url exceeds maximum allowed length"}
		}
		parsed, err := url.ParseRequestURI(*req.BaseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return &ValidationError{Message: "base_url is not a valid URL"}
		}
		if parsed.Scheme != "https" {
			return &ValidationError{Message: "base_url must use HTTPS"}
		}
	}

	numericChecks := map[string][2]float64{
		"temperature":       {0.0, 2.0},
		"top_p":             {0.0, 1.0},
		"frequency_penalty": {-2.0, 2.0},
		"presence_penalty":  {-2.0, 2.0},
	}

	if req.Temperature != nil {
		if err := checkNumeric("temperature", *req.Temperature, numericChecks["temperature"]); err != nil {
			return err
		}
	}
	if req.TopP != nil {
		if err := checkNumeric("top_p", *req.TopP, numericChecks["top_p"]); err != nil {
			return err
		}
	}
	if req.FrequencyPenalty != nil {
		if err := checkNumeric("frequency_penalty", *req.FrequencyPenalty, numericChecks["frequency_penalty"]); err != nil {
			return err
		}
	}
	if req.PresencePenalty != nil {
		if err := checkNumeric("presence_penalty", *req.PresencePenalty, numericChecks["presence_penalty"]); err != nil {
			return err
		}
	}

	intChecks := map[string][2]int{
		"max_tokens":      {1, 128000},
		"response_length": {1, 32000},
		"top_k":           {0, 500},
		"history_depth":   {1, 500},
	}

	if req.MaxTokens != nil {
		if err := checkInt("max_tokens", *req.MaxTokens, intChecks["max_tokens"]); err != nil {
			return err
		}
	}
	if req.ResponseLength != nil {
		if err := checkInt("response_length", *req.ResponseLength, intChecks["response_length"]); err != nil {
			return err
		}
	}
	if req.TopK != nil {
		if err := checkInt("top_k", *req.TopK, intChecks["top_k"]); err != nil {
			return err
		}
	}
	if req.HistoryDepth != nil {
		if err := checkInt("history_depth", *req.HistoryDepth, intChecks["history_depth"]); err != nil {
			return err
		}
	}

	for i, msg := range req.Messages {
		if msg.Role == "" || msg.Content == "" {
			return &ValidationError{Message: "messages[" + string(rune(i)) + "] must contain 'role' and 'content' fields"}
		}
		allowedRoles := map[string]bool{"user": true, "assistant": true, "system": true}
		if !allowedRoles[msg.Role] {
			return &ValidationError{Message: "messages[" + string(rune(i)) + "].role must be one of: user, assistant, system"}
		}
		if controlChars.MatchString(msg.Content) {
			return &ValidationError{Message: "messages[" + string(rune(i)) + "].content contains invalid control characters"}
		}
	}

	return nil
}

func checkNumeric(field string, val float64, bounds [2]float64) *ValidationError {
	if val < bounds[0] || val > bounds[1] {
		return &ValidationError{Message: field + " must be between " + fmtFloat(bounds[0]) + " and " + fmtFloat(bounds[1])}
	}
	return nil
}

func checkInt(field string, val int, bounds [2]int) *ValidationError {
	if val < bounds[0] || val > bounds[1] {
		return &ValidationError{Message: field + " must be between " + fmtInt(bounds[0]) + " and " + fmtInt(bounds[1])}
	}
	return nil
}

func fmtFloat(f float64) string {
	s := ""
	if f == float64(int(f)) {
		s = fmtInt(int(f))
	} else {
		s = fmt.Sprintf("%.1f", f)
	}
	return s
}

func fmtInt(i int) string {
	return fmt.Sprintf("%d", i)
}
