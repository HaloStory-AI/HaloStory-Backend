package byok

import (
	"encoding/base64"
	"encoding/json"
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
	UserPersona      *string          `json:"user_persona,omitempty"`
	UserName         *string          `json:"user_name,omitempty"`
	BotIDs           []string         `json:"bot_ids,omitempty"`
	ExistingMemories []string         `json:"existing_memories,omitempty"`
	TopA             *float64         `json:"top_a,omitempty"`
	MinP             *float64         `json:"min_p,omitempty"`
	RepetitionPenalty *float64        `json:"repetition_penalty,omitempty"`
	ContextLimit     *int             `json:"context_limit,omitempty"`
	Stream           *bool            `json:"stream,omitempty"`
	SourceIP         string           `json:"-"`
}

const ChatImageMaxBytes = 5 << 20 // 5 MB decoded

type ImageURL struct {
	URL    string  `json:"url"`
	Detail *string `json:"detail,omitempty"`
}

type ContentPart struct {
	Type     string    `json:"type"`
	Text     *string   `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// VisionContent is polymorphic: string OR []ContentPart (OpenAI standard).
type VisionContent struct {
	isArray bool
	text    string
	parts   []ContentPart
}

func (v VisionContent) IsArray() bool          { return v.isArray }
func (v VisionContent) AsText() string {
	if v.isArray {
		var sb strings.Builder
		for _, p := range v.parts {
			if p.Type == "text" && p.Text != nil {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(*p.Text)
			}
		}
		return sb.String()
	}
	return v.text
}
func (v VisionContent) Parts() []ContentPart {
	if v.isArray {
		return v.parts
	}
	return nil
}
func (v VisionContent) HasImage() bool {
	if !v.isArray {
		return false
	}
	for _, p := range v.parts {
		if p.Type == "image_url" && p.ImageURL != nil && p.ImageURL.URL != "" {
			return true
		}
	}
	return false
}
func (v VisionContent) ImageURLs() []string {
	var out []string
	for _, p := range v.parts {
		if p.Type == "image_url" && p.ImageURL != nil {
			out = append(out, p.ImageURL.URL)
		}
	}
	return out
}
func (v VisionContent) MarshalJSON() ([]byte, error) {
	if v.isArray {
		return json.Marshal(v.parts)
	}
	return json.Marshal(v.text)
}
func (v *VisionContent) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		v.isArray = false
		v.text = ""
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		v.isArray = false
		v.text = s
		v.parts = nil
		return nil
	}
	if trimmed[0] == '[' {
		var parts []ContentPart
		if err := json.Unmarshal(data, &parts); err != nil {
			return err
		}
		v.isArray = true
		v.parts = parts
		v.text = ""
		return nil
	}
	// fallback try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v.isArray = false
		v.text = s
		return nil
	}
	return fmt.Errorf("VisionContent must be string or array")
}

func NewTextContent(text string) VisionContent {
	return VisionContent{isArray: false, text: text}
}
func NewVisionContent(parts []ContentPart) VisionContent {
	return VisionContent{isArray: true, parts: parts}
}

type Message struct {
	Role    string        `json:"role"`
	Content VisionContent `json:"content"`
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

	// Message structural checks: roles must be valid, content polymorphic (string OR [text+image_url])
	for i, msg := range req.Messages {
		idxStr := strconv.Itoa(i)
		if msg.Role == "" {
			err := &ValidationError{Message: "messages[" + idxStr + "] must contain 'role' field"}
			logValidationFailure(req, err)
			return err
		}
		allowedRoles := map[string]bool{"user": true, "assistant": true, "system": true}
		if !allowedRoles[msg.Role] {
			err := &ValidationError{Message: "messages[" + idxStr + "].role must be one of: user, assistant, system"}
			logValidationFailure(req, err)
			return err
		}
		if err := validateVisionContent(idxStr, msg.Content); err != nil {
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

func validateVisionContent(idxStr string, c VisionContent) *ValidationError {
	if c.IsArray() {
		parts := c.Parts()
		if len(parts) == 0 {
			return &ValidationError{Message: "messages[" + idxStr + "].content array must not be empty"}
		}
		if len(parts) > 10 {
			return &ValidationError{Message: "messages[" + idxStr + "].content too many parts (max 10)"}
		}
		hasText := false
		for j, p := range parts {
			switch p.Type {
			case "text":
				if p.Text == nil || strings.TrimSpace(*p.Text) == "" {
					return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].text must be non-empty", idxStr, j)}
				}
				if controlChars.MatchString(*p.Text) {
					return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].text contains invalid control characters", idxStr, j)}
				}
				hasText = true
			case "image_url":
				if p.ImageURL == nil || strings.TrimSpace(p.ImageURL.URL) == "" {
					return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.url must be non-empty", idxStr, j)}
				}
				if p.ImageURL.Detail != nil {
					d := strings.ToLower(strings.TrimSpace(*p.ImageURL.Detail))
					if d != "auto" && d != "low" && d != "high" {
						return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.detail must be auto|low|high", idxStr, j)}
					}
				}
				u := strings.TrimSpace(p.ImageURL.URL)
				if strings.HasPrefix(u, "data:") {
					// data:image/<mime>;base64,<data>
					if !strings.HasPrefix(u, "data:image/") {
						return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.url data URI must be image", idxStr, j)}
					}
					comma := strings.Index(u, ",")
					if comma < 0 {
						return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.url invalid data URI", idxStr, j)}
					}
					meta := u[:comma]
					if !strings.Contains(meta, ";base64") {
						return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.url data URI must be base64", idxStr, j)}
					}
					b64 := u[comma+1:]
					// quick size check: decoded size approx 3/4 * len
					est := (len(b64) * 3 / 4)
					if est > ChatImageMaxBytes {
						return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.url exceeds %d bytes", idxStr, j, ChatImageMaxBytes)}
					}
					if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
						if _, err2 := base64.RawStdEncoding.DecodeString(b64); err2 != nil {
							return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.url invalid base64", idxStr, j)}
						}
					}
				} else {
					parsed, err := url.ParseRequestURI(u)
					if err != nil || parsed.Scheme == "" || parsed.Host == "" {
						return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.url must be valid URL", idxStr, j)}
					}
					if parsed.Scheme != "https" && parsed.Scheme != "http" {
						return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].image_url.url must be http(s)", idxStr, j)}
					}
				}
			default:
				return &ValidationError{Message: fmt.Sprintf("messages[%s].content[%d].type must be text|image_url", idxStr, j)}
			}
		}
		if !hasText {
			// allow image-only? require at least one text per OpenAI spec? we allow image-only for now
		}
		return nil
	}
	text := c.AsText()
	if strings.TrimSpace(text) == "" {
		return &ValidationError{Message: "messages[" + idxStr + "] must contain 'role' and 'content' fields"}
	}
	if controlChars.MatchString(text) {
		return &ValidationError{Message: "messages[" + idxStr + "].content contains invalid control characters"}
	}
	return nil
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
