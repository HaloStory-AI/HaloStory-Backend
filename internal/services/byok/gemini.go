package byok

import (
	"bufio"
	"bytes"
	"context"
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
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
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
	Temperature      float64         `json:"temperature,omitempty"`
	TopP             float64         `json:"topP,omitempty"`
	TopK             int             `json:"topK,omitempty"`
	MaxOutputTokens  int             `json:"maxOutputTokens,omitempty"`
	StopSequences    []string        `json:"stopSequences,omitempty"`
	PresencePenalty  float64         `json:"presencePenalty,omitempty"`
	FrequencyPenalty float64         `json:"frequencyPenalty,omitempty"`
	ThinkingConfig   *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

type GeminiRequest struct {
	Contents          []GeminiContent  `json:"contents"`
	SystemInstruction *GeminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  GenerationConfig `json:"generationConfig"`
	SafetySettings    []SafetySetting  `json:"safetySettings"`
}

type GeminiResponse struct {
	Candidates     []GeminiCandidate `json:"candidates"`
	PromptFeedback *PromptFeedback   `json:"promptFeedback"`
	UsageMetadata  *UsageMetadata    `json:"usageMetadata"`
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
	{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: "BLOCK_NONE"},
}

func CallGemini(apiKey string, model string, payload GeminiRequest) (*GeminiResponse, int, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-goog-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, resp.StatusCode, fmt.Errorf("rate limited (429)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, resp.StatusCode, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return &geminiResp, resp.StatusCode, nil
}

// CallGeminiStream memakai endpoint streamGenerateContent (SSE, alt=sse).
// Setiap delta teks diteruskan ke onDelta; hasil penuh dikembalikan sebagai
// GeminiResponse tunggal agar caller bisa memakai ConvertToOpenAIFormat yang sama.
func CallGeminiStream(ctx context.Context, httpClient *http.Client, apiKey string, model string, payload GeminiRequest, onDelta func(string) error) (*GeminiResponse, int, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse", model)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-goog-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, resp.StatusCode, fmt.Errorf("rate limited (429)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, resp.StatusCode, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var fullText strings.Builder
	var lastResp *GeminiResponse
	var blockReason, blockReasonMessage string
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk GeminiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.PromptFeedback != nil {
			if chunk.PromptFeedback.BlockReason != "" {
				blockReason = chunk.PromptFeedback.BlockReason
			}
			if chunk.PromptFeedback.BlockReasonMessage != "" {
				blockReasonMessage = chunk.PromptFeedback.BlockReasonMessage
			}
		}
		for _, cand := range chunk.Candidates {
			for _, part := range cand.Content.Parts {
				if part.Thought != nil && *part.Thought {
					continue
				}
				if part.Text != "" {
					fullText.WriteString(part.Text)
					if onDelta != nil {
						_ = onDelta(part.Text)
					}
				}
			}
		}
		lastResp = &chunk
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		if fullText.Len() == 0 && lastResp == nil {
			return nil, 0, fmt.Errorf("stream read error: %w", err)
		}
	}

	merged := &GeminiResponse{}
	if lastResp != nil {
		*merged = *lastResp
	}
	merged.Candidates = []GeminiCandidate{{
		Content: GeminiContent{
			Role:  "model",
			Parts: []GeminiPart{{Text: fullText.String()}},
		},
		FinishReason: "STOP",
	}}
	if len(merged.Candidates) > 0 && lastResp != nil && len(lastResp.Candidates) > 0 && lastResp.Candidates[0].FinishReason != "" {
		merged.Candidates[0].FinishReason = lastResp.Candidates[0].FinishReason
	}
	if blockReason != "" || blockReasonMessage != "" {
		merged.PromptFeedback = &PromptFeedback{BlockReason: blockReason, BlockReasonMessage: blockReasonMessage}
	}
	return merged, http.StatusOK, nil
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
