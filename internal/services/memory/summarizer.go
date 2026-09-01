package memory

import (
	"halostory_go_backend/internal/services/byok"
)

// Wrapper for prompt.go summarization (refactor, no change logic)
func BuildSummarizationPrompt(messages []byok.Message, language string, existingMemories []string) string {
	return byok.BuildSummarizationPrompt(messages, language, existingMemories)
}

func ParseSummaryResponse(resp string) (string, []string) {
	return byok.ParseSummaryResponse(resp)
}
