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

type OpenAIChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string OR []ContentPart (vision)
}

type OpenAICompatRequest struct {
	Model             string               `json:"model"`
	Messages          []OpenAIChatMessage  `json:"messages"`
	Stream            *bool                `json:"stream,omitempty"`
	Temperature       *float64             `json:"temperature,omitempty"`
	MaxTokens         *int                 `json:"max_tokens,omitempty"`
	TopP              *float64             `json:"top_p,omitempty"`
	FrequencyPenalty  *float64             `json:"frequency_penalty,omitempty"`
	PresencePenalty   *float64             `json:"presence_penalty,omitempty"`
	MinP              *float64             `json:"min_p,omitempty"`
	RepetitionPenalty *float64             `json:"repetition_penalty,omitempty"`
	Stop              []string             `json:"stop,omitempty"`
}

type OpenAICompatChoice struct {
	Index        int              `json:"index"`
	Message      OpenAIChatMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type OpenAICompatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAICompatResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []OpenAICompatChoice `json:"choices"`
	Usage   *OpenAICompatUsage   `json:"usage,omitempty"`
}

type OpenAIRequestOptions struct {
	ExtraHeaders map[string]string      `json:"extra_headers,omitempty"`
	ExtraBody    map[string]interface{} `json:"extra_body,omitempty"`
}

func CallOpenAI(httpClient *http.Client, apiKey, baseURL, model string, payload OpenAICompatRequest, timeout time.Duration, opts *OpenAIRequestOptions) (*OpenAICompatResponse, int, error) {
	endpoint := baseURL
	if !strings.Contains(endpoint, "/chat/completions") {
		endpoint = strings.TrimRight(endpoint, "/") + "/chat/completions"
	}

	if model != "" {
		payload.Model = model
	}

	// Always request a non-streaming JSON response. Some OpenAI-compatible
	// providers/routers (e.g. 9Router) stream by default when `stream` is
	// omitted; this pipeline expects a single JSON body.
	stream := false
	payload.Stream = &stream

	bodyMap := make(map[string]interface{})
	bodyBytes, _ := json.Marshal(payload)
	json.Unmarshal(bodyBytes, &bodyMap)

	if opts != nil && opts.ExtraBody != nil {
		for k, v := range opts.ExtraBody {
			bodyMap[k] = v
		}
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if opts != nil && opts.ExtraHeaders != nil {
		for k, v := range opts.ExtraHeaders {
			req.Header.Set(k, v)
		}
	}

	resp, err := httpClient.Do(req)
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
		return nil, resp.StatusCode, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openAIResp OpenAICompatResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return &openAIResp, resp.StatusCode, nil
}

// Fase 3: streaming via SSE (OpenAI-compatible)
// Parses `data: {...}` lines, calls onDelta per content chunk.
func CallOpenAIStream(ctx context.Context, httpClient *http.Client, apiKey, baseURL, model string, payload OpenAICompatRequest, opts *OpenAIRequestOptions, onDelta func(string) error) (*OpenAICompatResponse, int, error) {
	endpoint := baseURL
	if !strings.Contains(endpoint, "/chat/completions") {
		endpoint = strings.TrimRight(endpoint, "/") + "/chat/completions"
	}
	if model != "" {
		payload.Model = model
	}
	stream := true
	payload.Stream = &stream

	bodyMap := make(map[string]interface{})
	bodyBytes, _ := json.Marshal(payload)
	_ = json.Unmarshal(bodyBytes, &bodyMap)
	if opts != nil && opts.ExtraBody != nil {
		for k, v := range opts.ExtraBody {
			bodyMap[k] = v
		}
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if opts != nil && opts.ExtraHeaders != nil {
		for k, v := range opts.ExtraHeaders {
			req.Header.Set(k, v)
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		b, _ := io.ReadAll(resp.Body)
		return nil, resp.StatusCode, fmt.Errorf("rate limited (429): %s", string(b))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, resp.StatusCode, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(b))
	}
	// SSE parse
	var fullContent strings.Builder
	var finishReason string
	var usage OpenAICompatUsage
	scanner := bufio.NewScanner(resp.Body)
	// increase buffer for long lines
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
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
				Index        int     `json:"index"`
			} `json:"choices"`
			Usage *OpenAICompatUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				fullContent.WriteString(ch.Delta.Content)
				if onDelta != nil {
					_ = onDelta(ch.Delta.Content)
				}
			}
			if ch.FinishReason != nil && *ch.FinishReason != "" {
				finishReason = *ch.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		// treat as error only if no content
		if fullContent.Len() == 0 {
			return nil, 0, fmt.Errorf("stream read error: %w", err)
		}
	}
	respObj := &OpenAICompatResponse{
		Model: model,
		Choices: []OpenAICompatChoice{
			{Index: 0, Message: OpenAIChatMessage{Role: "assistant", Content: fullContent.String()}, FinishReason: finishReason},
		},
		Usage: &usage,
	}
	return respObj, http.StatusOK, nil
}
