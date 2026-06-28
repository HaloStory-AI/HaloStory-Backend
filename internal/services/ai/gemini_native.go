package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	SafetySettings   []geminiSafetySetting    `json:"safetySettings"`
	GenerationConfig geminiGenerationConfig   `json:"generationConfig"`
	ThinkingConfig   *geminiThinkingConfig    `json:"thinkingConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
}

type geminiThinkingConfig struct {
	Type string `json:"type"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

func ProxyGeminiNativeStream(model string, openaiPayload map[string]interface{}, apiKey string, streamTimeout time.Duration) map[string]interface{} {
	isGemma := strings.Contains(strings.ToLower(model), "gemma")

	geminiReq := convertToGeminiPayload(openaiPayload, isGemma, model)

	// All safety settings to BLOCK_NONE
	geminiReq.SafetySettings = []geminiSafetySetting{
		{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
		{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: "BLOCK_NONE"},
	}

	// Thinking config for reasoning models
	if !isGemma {
		geminiReq.ThinkingConfig = &geminiThinkingConfig{Type: "ENABLED"}
	} else {
		geminiReq.ThinkingConfig = &geminiThinkingConfig{Type: "MINIMAL"}
	}

	// Generation config
	if temp, ok := openaiPayload["temperature"].(float64); ok {
		geminiReq.GenerationConfig.Temperature = temp
	}
	if mt, ok := openaiPayload["max_tokens"].(float64); ok {
		geminiReq.GenerationConfig.MaxOutputTokens = int(mt)
	}
	if topP, ok := openaiPayload["top_p"].(float64); ok {
		geminiReq.GenerationConfig.TopP = topP
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	payloadBytes, err := json.Marshal(geminiReq)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to marshal: %v", err)}
	}

	client := &http.Client{Timeout: streamTimeout}

	for attempt := 0; attempt < 10; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("request error: %v", err)}
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			if attempt < 9 {
				time.Sleep(2 * time.Second)
				continue
			}
			return map[string]interface{}{"error": fmt.Sprintf("request failed: %v", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests && attempt < 9 {
			resp.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("read error: %v", err)}
		}

		var geminiResp geminiResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			// Try streaming mode
			return consumeGeminiStream(body, model)
		}

		return convertToOpenAIFormat(&geminiResp, model)
	}

	return map[string]interface{}{"error": "max retries exceeded"}
}

func convertToGeminiPayload(openaiPayload map[string]interface{}, isGemma bool, model string) geminiRequest {
	var req geminiRequest

	messages, _ := openaiPayload["messages"].([]interface{})

	var systemText string
	var contents []geminiContent

	for _, msg := range messages {
		m := msg.(map[string]interface{})
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)

		if role == "system" {
			systemText += content + "\n"
			continue
		}

		gRole := "user"
		if role == "assistant" {
			gRole = "model"
		}

		contents = append(contents, geminiContent{
			Role:  gRole,
			Parts: []geminiPart{{Text: content}},
		})
	}

	if isGemma {
		if systemText != "" {
			if len(contents) > 0 {
				contents[0].Parts[0].Text = systemText + "\n" + contents[0].Parts[0].Text
			}
		}
	} else {
		if systemText != "" {
			req.SystemInstruction = &geminiSystemInstruction{
				Parts: []geminiPart{{Text: systemText}},
			}
		}
	}

	req.Contents = contents
	return req
}

func convertToOpenAIFormat(resp *geminiResponse, model string) map[string]interface{} {
	var content string
	var finishReason string

	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		for _, part := range candidate.Content.Parts {
			content += part.Text
		}
		finishReason = candidate.FinishReason
	}

	if resp.PromptFeedback.BlockReason != "" {
		return map[string]interface{}{
			"content": "Maaf, Sensei. Permintaanmu diblokir oleh sistem keamanan.",
			"model":   model,
			"error":   fmt.Sprintf("blocked: %s", resp.PromptFeedback.BlockReason),
		}
	}

	openAIFormat := map[string]interface{}{
		"content": content,
		"model":   model,
	}

	switch finishReason {
	case "STOP":
		openAIFormat["finish_reason"] = "stop"
	case "MAX_TOKENS":
		openAIFormat["finish_reason"] = "length"
	case "SAFETY":
		openAIFormat["finish_reason"] = "content_filter"
		openAIFormat["content"] = "Maaf, Sensei. Response diblokir karena filter keamanan."
	case "RECITATION":
		openAIFormat["finish_reason"] = "recitation"
		openAIFormat["content"] = "Maaf, Sensei. Response mengandung konten yang tidak pantas."
	}

	return openAIFormat
}

func consumeGeminiStream(body []byte, model string) map[string]interface{} {
	lines := strings.Split(string(body), "\n")
	var fullContent string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamResp geminiResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Candidates) > 0 {
			for _, part := range streamResp.Candidates[0].Content.Parts {
				fullContent += part.Text
			}
		}
	}

	return map[string]interface{}{
		"content": fullContent,
		"model":   model,
	}
}
