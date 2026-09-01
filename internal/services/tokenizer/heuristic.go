package tokenizer

import (
	"unicode"
)

// Heuristic fallback: char*4 for latin, char/2.5 for CJK heavy, chars/3.5 for Claude-like
func heuristicCount(text string, model string) int {
	if text == "" {
		return 0
	}
	// Detect CJK ratio
	cjk := 0
	total := 0
	for _, r := range text {
		total++
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			cjk++
		}
	}
	if total > 0 && float64(cjk)/float64(total) > 0.3 {
		// CJK heavy: ~1 token per 1.5 chars
		return (len([]rune(text))*2 + 2) / 3 // ~0.66 * runes
	}
	// Claude approximation
	if isClaudeModel(model) {
		return (len(text) + 2) / 4 // chars/3.5 ~ chars/4 for safety (overestimate slightly)
	}
	// Gemini
	if isGeminiModel(model) {
		return (len(text) + 3) / 4
	}
	// Default GPT-like
	return (len(text) + 3) / 4
}

func isClaudeModel(model string) bool {
	// simple contains
	return containsFold(model, "claude")
}

func isGeminiModel(model string) bool {
	return containsFold(model, "gemini") || containsFold(model, "gemma")
}

func isGptModel(model string) bool {
	return containsFold(model, "gpt") || containsFold(model, "o1") || containsFold(model, "o3")
}

func containsFold(s, sub string) bool {
	// case-insensitive contains without importing strings for speed
	ls := toLower(s)
	lsub := toLower(sub)
	return indexOf(ls, lsub) >= 0
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func indexOf(s, sub string) int {
	n := len(sub)
	if n == 0 {
		return 0
	}
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == sub {
			return i
		}
	}
	return -1
}
