package types

import (
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services/memory"
)

// LoreInjection scans lorebooks for the given bot and returns an injection string
// to be appended to the system prompt. Returns "" if no lorebooks or no matches.
func LoreInjection(botID string, messages []byok.Message) string {
	if botID == "" {
		return ""
	}
	books := memory.GetLorebooks(botID)
	if len(books) == 0 {
		return ""
	}
	texts := make([]string, 0, len(messages))
	for _, m := range messages {
		texts = append(texts, m.Content)
	}
	matched := memory.Scan(texts, books)
	if len(matched) == 0 {
		return ""
	}
	return memory.BuildLorebookInjection(matched)
}

// AppendLoreToMessages appends loreInjection to the system message in-place.
// If no system message exists, a new one is prepended.
func AppendLoreToMessages(messages []byok.Message, injection string) []byok.Message {
	if injection == "" {
		return messages
	}
	for i, m := range messages {
		if m.Role == "system" {
			messages[i].Content += injection
			return messages
		}
	}
	if len(messages) > 0 && messages[0].Role != "system" {
		return append([]byok.Message{{Role: "system", Content: injection}}, messages...)
	}
	// Empty messages or first is system already handled; fallback prepend
	return append([]byok.Message{{Role: "system", Content: injection}}, messages...)
}
