package summarizer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/worker/types"
)

// HandleGemini handles Gemini summarization and returns a SummaryResult.
func HandleGemini(data types.ChatData, decryptedKey string) (types.SummaryResult, int) {
	messagesJSON, _ := base64.StdEncoding.DecodeString(data.Messages)
	var messages []byok.Message
	if err := json.Unmarshal(messagesJSON, &messages); err != nil {
		return types.NewSummaryError("Failed to parse messages", 400), 400
	}

	lang := data.Language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	prompt := byok.BuildSummarizationPrompt(messages, lang, data.ExistingMemories)

	contents := []byok.GeminiContent{
		{Role: "user", Parts: []byok.GeminiPart{{Text: prompt}}},
	}

	genConfig := byok.GenerationConfig{
		Temperature:     0.9,
		MaxOutputTokens: 4096,
	}

	isGemma := strings.Contains(data.Model, "gemma")
	if isGemma {
		genConfig.MaxOutputTokens = 32000
		if genConfig.ThinkingConfig == nil {
			genConfig.ThinkingConfig = &byok.ThinkingConfig{}
		}
		genConfig.ThinkingConfig.ThinkingLevel = "MINIMAL"
	}

	req := byok.GeminiRequest{
		Contents:         contents,
		GenerationConfig: genConfig,
		SafetySettings:   byok.DefaultSafetySettings,
	}

	geminiResp, statusCode, err := byok.CallGemini(decryptedKey, data.Model, req)
	if err != nil {
		errMsg := err.Error()
		if statusCode > 0 {
			errMsg = fmt.Sprintf("[ERROR %d] %s", statusCode, errMsg)
		}
		return types.NewSummaryError(errMsg, statusCode), statusCode
	}

	openaiResp := byok.ConvertToOpenAIFormat(geminiResp, data.Model)
	rawContent := ""
	if len(openaiResp.Choices) > 0 {
		rawContent = strings.TrimSpace(openaiResp.Choices[0].Message.Content)
	}

	summary, memories := byok.ParseSummaryResponse(rawContent)

	if summary == "" && rawContent != "" {
		log.Printf("Worker: Summary content empty, retrying with lower temperature")
		genConfig.Temperature = 0.3
		req.GenerationConfig = genConfig
		geminiResp, statusCode, err = byok.CallGemini(decryptedKey, data.Model, req)
		if err == nil {
			openaiResp = byok.ConvertToOpenAIFormat(geminiResp, data.Model)
			if len(openaiResp.Choices) > 0 {
				rawContent = strings.TrimSpace(openaiResp.Choices[0].Message.Content)
			}
			summary, memories = byok.ParseSummaryResponse(rawContent)
		}
	}

	if summary == "" {
		return types.NewSummaryError("[ERROR 500] No content returned from Gemini API", 500), 500
	}

	if memories == nil {
		memories = []string{}
	}
	return types.SummaryResult{Summary: summary, MemoryWorthKeeping: memories}, statusCode
}
