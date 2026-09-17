package summarizer

import (
	"log"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services/systemai"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker/types"
)

// HandleSystemAI handles System AI summarization.
func HandleSystemAI(data types.ChatData, messages []byok.Message, userID uint) (types.SummaryResult, int) {
	lang := data.Language
	if lang == "" || lang == "id" {
		lang = "Indonesian"
	}

	prompt := byok.BuildSummarizationPrompt(messages, lang, data.ExistingMemories)
	promptMessages := []byok.Message{{Role: "user", Content: byok.NewTextContent(prompt)}}

	isVip := false
	var user models.User
	if userID > 0 {
		database.DB.First(&user, userID)
		isVip = user.Role == "vip"
	}

	params := &systemai.SystemAIChatParams{
		Language:    lang,
		IsSummarize: true,
	}

	httpClient := utils.NewAIHttpClient()
	start := time.Now()
	result := systemai.RunFallbackChain(httpClient, promptMessages, params, isVip)
	log.Printf("Worker: System AI summarization took %v, success=%v", time.Since(start), result.Error == "")

	if result.Error != "" {
		log.Printf("Worker: System AI summarization all failed: %s", result.Error)
		return types.NewSummaryError("[ERROR 503] All system AI models failed", 503), 503
	}

	rawContent := strings.TrimSpace(result.Content)
	summary, memories := byok.ParseSummaryResponse(rawContent)

	if summary == "" && rawContent != "" {
		log.Printf("Worker: System AI summary content empty, retrying with lower temperature")
		params.Temperature = 0.3
		params.IsSummarize = true
		result2 := systemai.RunFallbackChain(httpClient, promptMessages, params, isVip)
		if result2.Error == "" {
			rawContent = strings.TrimSpace(result2.Content)
			summary, memories = byok.ParseSummaryResponse(rawContent)
		}
	}

	if summary == "" {
		log.Printf("Worker: No System AI summary content returned after retry")
		return types.NewSummaryError("[ERROR 500] No content returned from API", 500), 500
	}

	if memories == nil {
		memories = []string{}
	}
	return types.SummaryResult{Summary: summary, MemoryWorthKeeping: memories}, 200
}
