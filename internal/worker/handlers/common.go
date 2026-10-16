package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker/types"
)

func DecryptKey(encrypted string) (string, *types.WorkerError) {
	decrypted, err := utils.DecryptBYOKAPIKey(encrypted)
	if err != nil {
		return "", types.NewWorkerError("Failed to decrypt API key", 500)
	}
	return decrypted, nil
}

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

func HistoryDepth(v float64) int {
	if v == 0 {
		return 15
	}
	return int(v)
}

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

func SummaryPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func UserPersonaPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func PreparePromptMessages(
	messages []byok.Message,
	botID string,
	lang string,
	historyDepth int,
	contextLimit *int,
	summary *string,
	userName string,
	userPersona *string,
	model string,
	loreInjection string,
	botIDs []string,
	isGroup bool,
) []byok.Message {
	messages = byok.InjectByokSystemPromptWithModel(
		messages, &botID, &lang, &historyDepth, contextLimit,
		summary, userName, userPersona,
		model,
	)
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
		messages = byok.InjectGroupContext(messages, botIDs, &primaryBotID, lang, userName)
	}
	return messages
}

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

func EnsureJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

var _ = fmt.Sprintf
