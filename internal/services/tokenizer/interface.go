package tokenizer

// ChatMessage is a minimal copy of byok.Message to avoid circular import.
type ChatMessage struct {
	Role    string
	Content string
}

// Tokenizer counts tokens for a given model.
type Tokenizer interface {
	CountTokens(text string, model string) int
	CountChatTokens(messages []ChatMessage, model string) int
	TruncateToBudget(messages []ChatMessage, budget int, model string) []ChatMessage
}

// Global router instance (initialized in router.go init)
var Default Router
