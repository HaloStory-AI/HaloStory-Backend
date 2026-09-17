package summarizer

import (
	"fmt"
	"log"
	"strings"
	"time"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker/types"
)

// HandleOpenAI handles OpenAI-compatible summarization.
func HandleOpenAI(data types.ChatData, decryptedKey string, messages []byok.Message, model, baseURL string) (types.SummaryResult, int) {
	lang := data.Language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	prompt := byok.BuildSummarizationPrompt(messages, lang, data.ExistingMemories)

	temp := 0.9
	maxTokens := 16000
	reqPayload := byok.OpenAICompatRequest{
		Model: model,
		Messages: []byok.OpenAIChatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	var sumOpts *byok.OpenAIRequestOptions
	if data.Provider == "openrouter" {
		sumOpts = &byok.OpenAIRequestOptions{
			ExtraHeaders: map[string]string{
				"HTTP-Referer": config.Env("APP_URL", "https://hsai-stagging.my-bini.my.id"),
				"X-Title":      "HaloStory (Summarizer)",
			},
		}
	}

	httpClient := utils.NewAIHttpClient()
	openaiResp, statusCode, err := byok.CallOpenAI(httpClient, decryptedKey, baseURL, model, reqPayload, 60*time.Second, sumOpts)
	if err != nil {
		errMsg := err.Error()
		if statusCode > 0 {
			errMsg = fmt.Sprintf("[ERROR %d] %s", statusCode, errMsg)
		}
		return types.NewSummaryError(errMsg, statusCode), statusCode
	}

	rawContent := ""
	if len(openaiResp.Choices) > 0 {
		if s, ok := openaiResp.Choices[0].Message.Content.(string); ok {
			rawContent = strings.TrimSpace(s)
		}
	}

	summary, memories := byok.ParseSummaryResponse(rawContent)

	if summary == "" && rawContent != "" {
		log.Printf("Worker: OpenAI summary content empty, retrying with lower temperature")
		temp = 0.3
		reqPayload.Temperature = &temp
		openaiResp, statusCode, err = byok.CallOpenAI(httpClient, decryptedKey, baseURL, model, reqPayload, 60*time.Second, sumOpts)
		if err == nil && len(openaiResp.Choices) > 0 {
			if s, ok := openaiResp.Choices[0].Message.Content.(string); ok {
				rawContent = strings.TrimSpace(s)
			}
			summary, memories = byok.ParseSummaryResponse(rawContent)
		}
	}

	if summary == "" {
		return types.NewSummaryError("[ERROR 500] No content returned from API", 500), 500
	}

	if memories == nil {
		memories = []string{}
	}
	return types.SummaryResult{Summary: summary, MemoryWorthKeeping: memories}, statusCode
}
