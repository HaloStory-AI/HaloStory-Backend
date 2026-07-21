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

type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAICompatRequest struct {
	Model            string               `json:"model"`
	Messages         []OpenAIChatMessage  `json:"messages"`
	Temperature      *float64             `json:"temperature,omitempty"`
	MaxTokens        *int                 `json:"max_tokens,omitempty"`
	TopP             *float64             `json:"top_p,omitempty"`
	FrequencyPenalty *float64             `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64             `json:"presence_penalty,omitempty"`
	Stop             []string             `json:"stop,omitempty"`
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

func CallOpenAI(apiKey, baseURL, model string, payload OpenAICompatRequest, timeout time.Duration, opts *OpenAIRequestOptions) (*OpenAICompatResponse, int, error) {
	endpoint := baseURL
	if !strings.Contains(endpoint, "/chat/completions") {
		endpoint = strings.TrimRight(endpoint, "/") + "/chat/completions"
	}

	if model != "" {
		payload.Model = model
	}

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

	maxRetries := 10
	var lastResp *http.Response
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		if opts != nil && opts.ExtraHeaders != nil {
			for k, v := range opts.ExtraHeaders {
				req.Header.Set(k, v)
			}
		}

		client := &http.Client{Timeout: timeout}
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

		respBody, err := io.ReadAll(lastResp.Body)
		lastResp.Body.Close()
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read response body: %w", err)
		}

		if lastResp.StatusCode < 200 || lastResp.StatusCode >= 300 {
			if attempt < maxRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, lastResp.StatusCode, fmt.Errorf("API error (status %d): %s", lastResp.StatusCode, string(respBody))
		}

		var openAIResp OpenAICompatResponse
		if err := json.Unmarshal(respBody, &openAIResp); err != nil {
			return nil, 0, fmt.Errorf("failed to parse response: %w", err)
		}

		return &openAIResp, lastResp.StatusCode, nil
	}

	return nil, 0, fmt.Errorf("unexpected error after retries")
}
