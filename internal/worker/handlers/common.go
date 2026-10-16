package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker/types"
)

// DecryptKey decrypts the BYOK API key from the queued payload.
func DecryptKey(encrypted string) (string, *types.WorkerError) {
	decrypted, err := utils.DecryptBYOKAPIKey(encrypted)
	if err != nil {
		return "", types.NewWorkerError("Failed to decrypt API key", 500)
	}
	return decrypted, nil
}

// DecodeMessagesRaw returns the raw JSON bytes plus parsed messages.
func DecodeMessagesRaw(b64 string) ([]byte, []byok.Message, *types.WorkerError) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, nil, types.NewWorkerError("Failed to decode messages", 400)
	}
	var messages []byok.Message
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, nil, types.NewWorkerError("Failed to parse messages", 400)
	}
	return raw, messages, nil
}

// HistoryDepth returns the effective history depth (default 15).
func HistoryDepth(v float64) int {
	if v == 0 {
		return 15
	}
	return int(v)
}

// ContextLimitPtr returns a *int context limit from the float64 field, or nil if 0.
// Falls back to maxTokens when contextLimit is unset.
func ContextLimitPtr(contextLimit, maxTokens float64) *int {
	if contextLimit > 0 {
		v := int(contextLimit)
		return &v
	}
	if maxTokens > 0 {
		v := int(maxTokens)
		return &v
	}
	return nil
}

// SummaryPtr returns a *string summary or nil if empty.
func SummaryPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// UserPersonaPtr returns a *string persona or nil if empty.
func UserPersonaPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// PreparePromptMessages applies lore injection, system prompt injection, and group context
// in a single reusable pipeline. isTrusted controls bond inclusion (BYOK openai-compatible).
func PreparePromptMessages(
	messages []byok.Message,
	botID string,
	lang string,
	historyDepth int,
	contextLimit *int,
	summary *string,
	userID uint,
	isDisableBond bool,
	userPersona *string,
	model string,
	isTrusted bool,
	loreInjection string,
	botIDs []string,
	isGroup bool,
) []byok.Message {
	uidPtr := &userID
	if userID == 0 {
		uidPtr = nil
	}
	messages = byok.InjectByokSystemPromptWithModelTrusted(
		messages, &botID, &lang, &historyDepth, contextLimit,
		summary, uidPtr, isDisableBond, userPersona,
		model, isTrusted,
	)
	// Append lore after system prompt injection
	if loreInjection != "" {
		for i, m := range messages {
			if m.Role == "system" {
				messages[i].Content = byok.NewTextContent(m.Content.AsText() + loreInjection)
				loreInjection = ""
				break
			}
		}
		if loreInjection != "" {
			messages = append([]byok.Message{{Role: "system", Content: byok.NewTextContent(loreInjection)}}, messages...)
		}
	}
	if isGroup && len(botIDs) > 1 {
		primaryBotID := botID
		messages = byok.InjectGroupContext(messages, botIDs, &primaryBotID, lang, uidPtr)
	}
	return messages
}

// FormatOpenAIMessages converts internal messages to OpenAI chat messages (vision-aware).
func FormatOpenAIMessages(messages []byok.Message) []byok.OpenAIChatMessage {
	out := make([]byok.OpenAIChatMessage, 0, len(messages))
	for _, m := range messages {
		role := m.Role
		if role != "system" && role != "assistant" {
			role = "user"
		}
		var content interface{}
		if m.Content.HasImage() {
			parts := make([]byok.ContentPart, 0, len(m.Content.Parts())+1)
			for _, p := range m.Content.Parts() {
				parts = append(parts, p)
			}
			// Ensure text part exists; if AsText empty, keep image only
			hasText := false
			for _, p := range parts {
				if p.Type == "text" {
					hasText = true
					break
				}
			}
			if !hasText {
				txt := m.Content.AsText()
				if txt != "" {
					parts = append(parts, byok.ContentPart{Type: "text", Text: &txt})
				}
			}
			content = parts
		} else {
			content = m.Content.AsText()
		}
		out = append(out, byok.OpenAIChatMessage{Role: role, Content: content})
	}
	return out
}

// EnsureJSON is a helper to marshal without error (panic on marshal failure is impossible for maps).
func EnsureJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

var _ = fmt.Sprintf // prevent unused import if needed
