package summarizer

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/worker/types"
)

// HandleClaude handles Claude summarization.
func HandleClaude(data types.ChatData, decryptedKey string, messages []byok.Message, messagesJSON []byte) (types.SummaryResult, int) {
	lang := data.Language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	var parsedMessages []byok.Message
	if err := json.Unmarshal(messagesJSON, &parsedMessages); err != nil {
		return types.NewSummaryError("Failed to parse messages", 400), 400
	}

	_ = messages
	prompt := byok.BuildSummarizationPrompt(parsedMessages, lang, data.ExistingMemories)

	payload := byok.ClaudeRequest{
		Model:       data.Model,
		MaxTokens:   16000,
		Temperature: func() *float64 { t := 0.9; return &t }(),
		Messages: []byok.ClaudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	maxRetries := 10
	var claudeResp *byok.ClaudeResponse
	var statusCode int
	var callErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		claudeResp, statusCode, callErr = byok.CallClaude(decryptedKey, data.Model, payload, 60*time.Second)
		if callErr == nil {
			break
		}
		if statusCode == 429 {
			break
		}
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}

	if callErr != nil {
		errMsg := callErr.Error()
		if statusCode > 0 {
			errMsg = fmt.Sprintf("[ERROR %d] %s", statusCode, errMsg)
		}
		return types.NewSummaryError(errMsg, statusCode), statusCode
	}

	rawContent := ""
	for _, block := range claudeResp.Content {
		if block.Type == "text" {
			rawContent += block.Text
		}
	}
	rawContent = strings.TrimSpace(rawContent)

	summary, memories := byok.ParseSummaryResponse(rawContent)

	if summary == "" && rawContent != "" {
		log.Printf("Worker: Claude summary empty, retrying with lower temperature")
		t := 0.3
		payload.Temperature = &t
		claudeResp, statusCode, callErr = byok.CallClaude(decryptedKey, data.Model, payload, 60*time.Second)
		if callErr == nil {
			retryContent := ""
			for _, block := range claudeResp.Content {
				if block.Type == "text" {
					retryContent += block.Text
				}
			}
			retryContent = strings.TrimSpace(retryContent)
			if retryContent != "" {
				summary, memories = byok.ParseSummaryResponse(retryContent)
			}
		}
	}

	if summary == "" {
		return types.NewSummaryError("[ERROR 500] No content returned from Claude API", 500), 500
	}

	if memories == nil {
		memories = []string{}
	}
	return types.SummaryResult{Summary: summary, MemoryWorthKeeping: memories}, statusCode
}
