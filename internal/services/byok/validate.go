package byok

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
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
	TopA             *float64         `json:"top_a,omitempty"`
	MinP             *float64         `json:"min_p,omitempty"`
	RepetitionPenalty *float64        `json:"repetition_penalty,omitempty"`
	ContextLimit     *int             `json:"context_limit,omitempty"`
	Stream           *bool            `json:"stream,omitempty"`
	SourceIP         string           `json:"-"`
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
	DefaultOpenAIModel = "gpt-3.5-turbo"
	maxModelLength     = 128
	maxBaseURLLength   = 512
	maxAPIKeyLength    = 1024
)

func SanitizeByokInput(req *BYOKRequest) *ValidationError {
	if len(req.Messages) == 0 {
		err := &ValidationError{Message: "messages must be an array with at least one message"}
		logValidationFailure(req, err)
		return err
	}

	// Message structural checks: content must be a non-empty string, roles must be valid
	for i, msg := range req.Messages {
		idxStr := strconv.Itoa(i)
		if msg.Role == "" || msg.Content == "" {
			err := &ValidationError{Message: "messages[" + idxStr + "] must contain 'role' and 'content' fields"}
			logValidationFailure(req, err)
			return err
		}
		allowedRoles := map[string]bool{"user": true, "assistant": true, "system": true}
		if !allowedRoles[msg.Role] {
			err := &ValidationError{Message: "messages[" + idxStr + "].role must be one of: user, assistant, system"}
			logValidationFailure(req, err)
			return err
		}
		if controlChars.MatchString(msg.Content) {
			err := &ValidationError{Message: "messages[" + idxStr + "].content contains invalid control characters"}
			logValidationFailure(req, err)
			return err
		}
	}

	if len(req.APIKey) > maxAPIKeyLength {
		err := &ValidationError{Message: "api_key field exceeds maximum allowed length"}
		logValidationFailure(req, err)
		return err
	}

	if len(req.Model) > maxModelLength {
		err := &ValidationError{Message: "model field exceeds maximum allowed length"}
		logValidationFailure(req, err)
		return err
	}
	if controlChars.MatchString(req.Model) {
		err := &ValidationError{Message: "model field contains invalid control characters"}
		logValidationFailure(req, err)
		return err
	}

	if req.BaseURL != nil && *req.BaseURL != "" {
		if len(*req.BaseURL) > maxBaseURLLength {
			err := &ValidationError{Message: "base_url exceeds maximum allowed length"}
			logValidationFailure(req, err)
			return err
		}
		parsed, err := url.ParseRequestURI(*req.BaseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			err := &ValidationError{Message: "base_url is not a valid URL"}
			logValidationFailure(req, err)
			return err
		}
		if parsed.Scheme != "https" {
			err := &ValidationError{Message: "base_url must use HTTPS"}
			logValidationFailure(req, err)
			return err
		}
	}

	numericChecks := map[string][2]float64{
		"temperature":        {0.0, 2.0},
		"top_p":              {0.0, 1.0},
		"top_a":              {0.0, 1.0},
		"min_p":              {0.0, 1.0},
		"repetition_penalty": {0.0, 2.0},
		"frequency_penalty":  {-2.0, 2.0},
		"presence_penalty":   {-2.0, 2.0},
	}

	if req.Temperature != nil {
		if err := checkNumeric("temperature", *req.Temperature, numericChecks["temperature"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.TopP != nil {
		if err := checkNumeric("top_p", *req.TopP, numericChecks["top_p"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.TopA != nil {
		if err := checkNumeric("top_a", *req.TopA, numericChecks["top_a"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.MinP != nil {
		if err := checkNumeric("min_p", *req.MinP, numericChecks["min_p"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.RepetitionPenalty != nil {
		if err := checkNumeric("repetition_penalty", *req.RepetitionPenalty, numericChecks["repetition_penalty"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.FrequencyPenalty != nil {
		if err := checkNumeric("frequency_penalty", *req.FrequencyPenalty, numericChecks["frequency_penalty"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.PresencePenalty != nil {
		if err := checkNumeric("presence_penalty", *req.PresencePenalty, numericChecks["presence_penalty"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}

	intChecks := map[string][2]int{
		"max_tokens":      {1, 128000},
		"response_length": {1, 32000},
		"top_k":           {0, 500},
		"history_depth":   {1, 500},
		"context_limit":   {1, 128000},
	}

	if req.MaxTokens != nil {
		if err := checkInt("max_tokens", *req.MaxTokens, intChecks["max_tokens"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.ResponseLength != nil {
		if err := checkInt("response_length", *req.ResponseLength, intChecks["response_length"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.TopK != nil {
		if err := checkInt("top_k", *req.TopK, intChecks["top_k"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.HistoryDepth != nil {
		if err := checkInt("history_depth", *req.HistoryDepth, intChecks["history_depth"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}
	if req.ContextLimit != nil {
		if err := checkInt("context_limit", *req.ContextLimit, intChecks["context_limit"]); err != nil {
			logValidationFailure(req, err)
			return err
		}
	}

	return nil
}

func logValidationFailure(req *BYOKRequest, err *ValidationError) {
	ip := req.SourceIP
	if ip == "" {
		ip = "unknown"
	}
	model := req.Model
	if model == "" {
		model = "unknown"
	}
	log.Printf("[BYOK Validation Failed] IP: %s | Model: %s | Error: %s", ip, model, err.Message)
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

var TrustedBaseURLs = []string{
	"api.openai.com",
	"api.deepseek.com",
	"api.mistral.ai",
	"openrouter.ai",
	"api.z.ai",
	"api.anthropic.com",
	"api.groq.com",
	"api.cohere.ai",
	"api.cerebras.ai",
	"integrate.api.nvidia.com",
	"ollama.com",
}

func IsTrustedBaseURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Host)
	for _, trusted := range TrustedBaseURLs {
		if host == trusted || strings.HasSuffix(host, "."+trusted) {
			return true
		}
	}
	return false
}
