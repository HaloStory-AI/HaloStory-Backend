package ai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type StreamEvent struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func ConsumeSSEStream(body io.Reader, provider string, timeout time.Duration) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 1024*64), 1024*64)

	var sb strings.Builder
	done := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var event StreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			for _, choice := range event.Choices {
				sb.WriteString(choice.Delta.Content)
				if choice.FinishReason != nil && *choice.FinishReason == "stop" {
					done <- nil
					return
				}
			}
		}
		done <- scanner.Err()
	}()

	var err error
	if timeout > 0 {
		select {
		case err = <-done:
		case <-time.After(timeout):
			err = fmt.Errorf("stream timeout after %v", timeout)
		}
	} else {
		err = <-done
	}

	return sb.String(), err
}

func ConsumeStreamAsJSON(url string, headers map[string]string, payload map[string]interface{}, provider string, model string, timeout time.Duration) (map[string]interface{}, error) {
	if _, exists := payload["stream"]; !exists {
		payload["stream"] = true
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	maxRetries := 4

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries-1 {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusInternalServerError && attempt < maxRetries-1 {
			resp.Body.Close()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		content, err := ConsumeSSEStream(resp.Body, provider, timeout)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": content,
			"model":   model,
		}, nil
	}

	return nil, fmt.Errorf("all retries failed for %s", provider)
}

func IsValidAIResponse(data map[string]interface{}) bool {
	content, ok := data["content"].(string)
	if !ok || content == "" {
		return false
	}
	if errStr, ok := data["error"].(string); ok && errStr != "" {
		return false
	}
	return true
}

func SanitizeAIResponse(data map[string]interface{}, fallbackModel string) map[string]interface{} {
	if content, ok := data["content"].(string); ok {
		data["content"] = ProcessAIResponse(content)
	}

	if !IsValidAIResponse(data) {
		data["content"] = "Maaf, Sensei. Sepertinya aku tidak bisa merespons saat ini. Mungkin lain kali kita bisa bicara lagi ya?"
	}

	if data["model"] == nil || data["model"] == "" {
		data["model"] = fallbackModel
	}

	return data
}
