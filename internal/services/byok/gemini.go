package byok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GeminiPart struct {
	Text    string `json:"text,omitempty"`
	Thought *bool  `json:"thought,omitempty"`
}

type GeminiContent struct {
	Role  string        `json:"role"`
	Parts []GeminiPart  `json:"parts"`
}

type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type ThinkingConfig struct {
	IncludeThoughts *bool  `json:"includeThoughts,omitempty"`
	ThinkingBudget  *int   `json:"thinkingBudget,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
}

type GenerationConfig struct {
	Temperature      float64        `json:"temperature,omitempty"`
	TopP             float64        `json:"topP,omitempty"`
	TopK             int            `json:"topK,omitempty"`
	MaxOutputTokens  int            `json:"maxOutputTokens,omitempty"`
	StopSequences    []string       `json:"stopSequences,omitempty"`
	PresencePenalty  float64        `json:"presencePenalty,omitempty"`
	FrequencyPenalty float64        `json:"frequencyPenalty,omitempty"`
	ThinkingConfig   *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

type GeminiRequest struct {
	Contents          []GeminiContent  `json:"contents"`
	SystemInstruction *GeminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  GenerationConfig `json:"generationConfig"`
	SafetySettings    []SafetySetting  `json:"safetySettings"`
}

type GeminiResponse struct {
	Candidates    []GeminiCandidate `json:"candidates"`
	PromptFeedback *PromptFeedback  `json:"promptFeedback"`
	UsageMetadata  *UsageMetadata   `json:"usageMetadata"`
}

type GeminiCandidate struct {
	Content       GeminiContent `json:"content"`
	FinishReason  string        `json:"finishReason"`
	SafetyRatings []interface{} `json:"safetyRatings,omitempty"`
}

type PromptFeedback struct {
	BlockReason        string `json:"blockReason"`
	BlockReasonMessage string `json:"blockReasonMessage"`
}

type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   UsageMetadata  `json:"usage"`
}

var DefaultSafetySettings = []SafetySetting{
	{Category: "HARM_CATEGORY_HARASSMENT",        Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_HATE_SPEECH",       Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_CIVIC_INTEGRITY",   Threshold: "BLOCK_NONE"},
}

func CallGemini(apiKey string, model string, payload GeminiRequest) (*GeminiResponse, int, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	maxRetries := 10
	var lastResp *http.Response
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("x-goog-api-key", apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 120 * time.Second}
		lastResp, lastErr = client.Do(req)
		if lastErr != nil {
			if attempt < maxRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, 0, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
		}
		if lastResp.StatusCode == 429 {
			lastResp.Body.Close()
			return nil, lastResp.StatusCode, fmt.Errorf("rate limited (429)")
		}

		if lastResp.StatusCode < 200 || lastResp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(lastResp.Body)
			lastResp.Body.Close()
			if attempt < maxRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, lastResp.StatusCode, fmt.Errorf("API error (status %d): %s", lastResp.StatusCode, string(respBody))
		}

		respBody, err := io.ReadAll(lastResp.Body)
		lastResp.Body.Close()
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read response body: %w", err)
		}

		var geminiResp GeminiResponse
		if err := json.Unmarshal(respBody, &geminiResp); err != nil {
			return nil, 0, fmt.Errorf("failed to parse response: %w", err)
		}

		return &geminiResp, lastResp.StatusCode, nil
	}

	return nil, 0, fmt.Errorf("unexpected error after retries")
}

func ConvertToOpenAIFormat(geminiResp *GeminiResponse, model string) *OpenAIResponse {
	resp := &OpenAIResponse{
		ID:      fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []OpenAIChoice{
			{
				Index:        0,
				Message:      OpenAIMessage{Role: "assistant", Content: ""},
				FinishReason: "stop",
			},
		},
	}

	if geminiResp.UsageMetadata != nil {
		resp.Usage = *geminiResp.UsageMetadata
	}

	if len(geminiResp.Candidates) == 0 {
		return resp
	}

	candidate := geminiResp.Candidates[0]

	if len(candidate.Content.Parts) == 0 {
		return resp
	}

	var textChunks []string
	for _, part := range candidate.Content.Parts {
		if part.Thought != nil && *part.Thought {
			continue
		}
		if part.Text != "" {
			textChunks = append(textChunks, part.Text)
		}
	}

	resp.Choices[0].Message.Content = strings.Join(textChunks, "")

	switch candidate.FinishReason {
	case "STOP":
		resp.Choices[0].FinishReason = "stop"
	case "MAX_TOKENS":
		resp.Choices[0].FinishReason = "length"
	case "SAFETY", "RECITATION":
		resp.Choices[0].FinishReason = "content_filter"
	default:
		resp.Choices[0].FinishReason = "stop"
	}

	return resp
}
