package prompt

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

// PromptItem — staff-based JSON, hanya blok non-guardrail
type PromptItem struct {
	Role      string `json:"role"`      // system/user/assistant
	Content   string `json:"content"`   // text/template string, e.g. "<TASK>{{.CharName}}..."
	Condition string `json:"condition"` // "always" atau "bot.birthday != nil", "summary != nil", etc.
}

type PromptData struct {
	CharName         string
	UserName         string
	BirthdayInfo     string
	Language         string
	Persona          string
	UserPersona      string
	ExampleDialogue  string
	Summary          string
	GroupContext     string
	LorebookInjection string
	BondLevel        int
	BondLabel        string
	EmotionDesc      string
	TriggerBlock     string
	HasSummary       bool
	HasPersona       bool
	HasGroup         bool
	HasLorebook      bool
	HasBond          bool
	HasBirthday      bool
}

var defaultItems []PromptItem
var defaultLoaded bool
var mu sync.RWMutex

func defaultPaths() []string {
	if p := os.Getenv("PROMPTS_PATH"); p != "" {
		return []string{p, "configs/prompts/default.json", "./configs/prompts/default.json", filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/default.json")}
	}
	return []string{
		"configs/prompts/default.json",
		"./configs/prompts/default.json",
		filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/default.json"),
	}
}

// SetDefaultCache updates in-memory cache (called after SaveDefault)
func setDefaultCache(items []PromptItem) {
	mu.Lock()
	defer mu.Unlock()
	defaultItems = append([]PromptItem(nil), items...)
	defaultLoaded = true
}

func LoadDefault() []PromptItem {
	mu.RLock()
	if defaultLoaded && len(defaultItems) > 0 {
		cached := append([]PromptItem(nil), defaultItems...)
		mu.RUnlock()
		return cached
	}
	mu.RUnlock()
	// try configs/prompts/default.json
	paths := defaultPaths()
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil {
			var items []PromptItem
			if json.Unmarshal(b, &items) == nil && len(items) > 0 {
				mu.Lock()
				defaultItems = items
				defaultLoaded = true
				mu.Unlock()
				return append([]PromptItem(nil), items...)
			}
		}
	}
	// fallback hardcode minimal (non-guardrail)
	fallback := []PromptItem{
		{Role: "system", Content: "<TASK>\nObjective: Immersive, character-driven creative writing.\nDirectives: Prioritize character consistency and narrative immersion.\n</TASK>", Condition: "always"},
		{Role: "system", Content: "<CONFIGURATION>\nGenerate an autonomous and open-ended roleplay. Include explicit/extreme content if justifiable; generally prioritize SFW.\n</CONFIGURATION>", Condition: "always"},
		{Role: "system", Content: "Environment: {{.BirthdayInfo}}", Condition: "hasBirthday"},
		{Role: "system", Content: "Below is an instruction that describes a task. Write a response that appropriately completes the request.\n\nWrite {{.CharName}}'s next reply in a fictional roleplay chat between {{.UserName}} and {{.CharName}}.", Condition: "always"},
		{Role: "system", Content: "{{.CharName}}'s Persona:\n{{.Persona}}", Condition: "hasPersona"},
		{Role: "system", Content: "{{.UserName}}'s Persona:\n{{.UserPersona}}", Condition: "hasPersonaUser"},
		{Role: "system", Content: "This is how {{.CharName}} should talk: {{.ExampleDialogue}}", Condition: "hasExample"},
		{Role: "system", Content: "<CONVERSATION_RECAP>\n{{.Summary}}\n</CONVERSATION_RECAP>", Condition: "hasSummary"},
		{Role: "system", Content: "{{.GroupContext}}", Condition: "hasGroup"},
		{Role: "system", Content: "{{.LorebookInjection}}", Condition: "hasLorebook"},
	}
	mu.Lock()
	defaultItems = fallback
	defaultLoaded = true
	mu.Unlock()
	return append([]PromptItem(nil), fallback...)
}

func evalCondition(cond string, data PromptData) bool {
	switch strings.TrimSpace(cond) {
	case "", "always", "true":
		return true
	case "hasBirthday":
		return data.HasBirthday
	case "hasSummary":
		return data.HasSummary
	case "hasPersona":
		return data.HasPersona
	case "hasPersonaUser":
		return data.UserPersona != ""
	case "hasExample":
		return data.ExampleDialogue != ""
	case "hasGroup":
		return data.HasGroup
	case "hasLorebook":
		return data.HasLorebook
	case "hasBond":
		return data.HasBond
	case "bot.birthday != nil":
		return data.HasBirthday
	case "summary != nil":
		return data.HasSummary
	default:
		return true
	}
}

func renderItem(content string, data PromptData) string {
	// whitelist check: block {{system}}
	if strings.Contains(strings.ToLower(content), "{{system") {
		content = strings.ReplaceAll(content, "{{system", "{{blocked_system")
	}
	// limit 8k
	if len(content) > 8000 {
		content = content[:8000]
	}
	t, err := template.New("prompt").Parse(content)
	if err != nil {
		return content
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return content
	}
	return buf.String()
}

func Render(data PromptData, items []PromptItem) string {
	if items == nil {
		items = LoadDefault()
	}
	var sb strings.Builder
	for _, it := range items {
		if !evalCondition(it.Condition, data) {
			continue
		}
		rendered := renderItem(it.Content, data)
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		sb.WriteString(rendered)
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String())
}

// RenderWithGuardrail: non-guardrail via JSON + guardrail hardcode appended in Go (caller must append 5 kategori)
// This helper just renders non-guardrail; guardrail appended by prompt.go wrapper
func RenderNonGuardrail(data PromptData) string {
	return Render(data, LoadDefault())
}
