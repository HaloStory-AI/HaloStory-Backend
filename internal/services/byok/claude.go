package byok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type ClaudeRequest struct {
	Model         string               `json:"model"`
	Messages      []ClaudeMessage      `json:"messages"`
	MaxTokens     int                  `json:"max_tokens"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	TopK          *int                 `json:"top_k,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	System        string               `json:"system,omitempty"`
	Thinking      *ClaudeThinkingConfig `json:"thinking,omitempty"`
}

type ClaudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ClaudeResponse struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Role       string               `json:"role"`
	Content    []ClaudeContentBlock `json:"content"`
	Model      string               `json:"model"`
	StopReason string               `json:"stop_reason"`
	Usage      ClaudeUsage          `json:"usage"`
}

type ClaudeErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func CallClaude(apiKey, model string, payload ClaudeRequest, timeout time.Duration) (*ClaudeResponse, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, resp.StatusCode, fmt.Errorf("rate limited (429)")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp ClaudeErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, resp.StatusCode, fmt.Errorf("Claude API error: %s", errResp.Error.Message)
		}
		return nil, resp.StatusCode, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return &claudeResp, resp.StatusCode, nil
}

func ConvertClaudeToOpenAIFormat(claudeResp *ClaudeResponse, model string) map[string]interface{} {
	text := ""
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return map[string]interface{}{
		"id":      claudeResp.ID,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": mapClaudeFinishReason(claudeResp.StopReason),
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     claudeResp.Usage.InputTokens,
			"completion_tokens": claudeResp.Usage.OutputTokens,
			"total_tokens":      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}
}

func mapClaudeFinishReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

func BuildClaudePayload(messages []Message, model string, data map[string]interface{}) ClaudeRequest {
	systemPrompt := ""
	formattedMessages := make([]ClaudeMessage, 0)
	lastRole := ""

	for _, msg := range messages {
		if msg.Role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n"
			}
			systemPrompt += msg.Content
			continue
		}

		role := "user"
		if msg.Role == "assistant" {
			role = "assistant"
		}

		if lastRole == role && len(formattedMessages) > 0 {
			formattedMessages[len(formattedMessages)-1].Content += "\n\n" + msg.Content
		} else {
			formattedMessages = append(formattedMessages, ClaudeMessage{Role: role, Content: msg.Content})
			lastRole = role
		}
	}

	if len(formattedMessages) > 0 && formattedMessages[0].Role != "user" {
		formattedMessages = append([]ClaudeMessage{{Role: "user", Content: "..."}}, formattedMessages...)
	}

	genLimit := 1000
	if rl, ok := data["response_length"].(float64); ok && rl > 0 {
		genLimit = int(rl)
	} else if mt, ok := data["max_tokens"].(float64); ok && mt > 0 {
		genLimit = int(mt)
	}

	temp := 1.0
	if t, ok := data["temperature"].(float64); ok && t != 0 {
		temp = t
	}

	payload := ClaudeRequest{
		Model:     model,
		Messages:  formattedMessages,
		MaxTokens: genLimit,
		Temperature: &temp,
	}

	if tp, ok := data["top_p"].(float64); ok && tp != 0 {
		payload.TopP = &tp
	}
	if tk, ok := data["top_k"].(float64); ok && tk != 0 {
		tkInt := int(tk)
		payload.TopK = &tkInt
	}
	if systemPrompt != "" {
		payload.System = systemPrompt
	}

	stopSeqs := []string{"\n\nHuman:", "\n\nAssistant:"}
	if existingStop, ok := data["stop"].([]interface{}); ok {
		for _, s := range existingStop {
			if str, ok := s.(string); ok {
				stopSeqs = append(stopSeqs, str)
			}
		}
	}
	payload.StopSequences = stopSeqs

	if reasoning, ok := data["reasoning"].(map[string]interface{}); ok {
		if enabled, _ := reasoning["enabled"].(bool); enabled {
			budget := 1024
			if b, ok := reasoning["budget"].(float64); ok && int(b) > 1024 {
				budget = int(b)
			}
			payload.Thinking = &ClaudeThinkingConfig{
				Type:         "enabled",
				BudgetTokens: budget,
			}
			payload.Temperature = nil
			payload.TopK = nil
			payload.MaxTokens = budget + genLimit
		}
	}

	return payload
}
