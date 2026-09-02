package prompt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var allowedRoles = map[string]bool{"system": true, "user": true, "assistant": true}
var allowedConditions = map[string]bool{
	"": true, "always": true, "true": true,
	"hasBirthday": true, "hasSummary": true, "hasPersona": true, "hasPersonaUser": true,
	"hasExample": true, "hasGroup": true, "hasLorebook": true, "hasBond": true,
	"bot.birthday != nil": true, "summary != nil": true,
}

func Validate(items []PromptItem) error {
	for i, it := range items {
		if it.Content != "" && len(it.Content) > 8000 {
			return fmt.Errorf("item %d: content too long >8000", i)
		}
		if it.Content != "" && it.Role != "" && !allowedRoles[it.Role] {
			return fmt.Errorf("item %d: invalid role %q", i, it.Role)
		}
		if it.Condition != "" && !allowedConditions[strings.TrimSpace(it.Condition)] {
			return fmt.Errorf("item %d: invalid condition %q", i, it.Condition)
		}
		if strings.Contains(strings.ToLower(it.Content), "{{system") {
			return fmt.Errorf("item %d: blocked pattern {{system}}", i)
		}
		if strings.Contains(strings.ToLower(it.Content), "<script") {
			return fmt.Errorf("item %d: blocked pattern <script", i)
		}
	}
	return nil
}

func SaveDefault(items []PromptItem) error {
	if err := Validate(items); err != nil {
		return err
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	// resolve write path: PROMPTS_PATH env or first default path
	target := "configs/prompts/default.json"
	if p := os.Getenv("PROMPTS_PATH"); p != "" {
		target = p
	}
	// ensure dir exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	// also try to sync to Dir(Args[0]) fallback if different
	alt := filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/default.json")
	if alt != target {
		_ = os.MkdirAll(filepath.Dir(alt), 0755)
		_ = os.WriteFile(alt, b, 0644)
	}
	setDefaultCache(items)
	return nil
}

func ListTemplates() []PromptItem {
	return LoadDefault()
}
