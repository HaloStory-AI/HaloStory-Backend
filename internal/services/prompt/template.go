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
		// single-file system.md can be up to 20k, multi-item json still 8k per item but allow 20k for forward compat
		if it.Content != "" && len(it.Content) > 20000 {
			return fmt.Errorf("item %d: content too long >20000", i)
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
	// If single system.md item exists and PROMPTS_PATH or existing system.md detected, save as markdown
	if len(items) == 1 && isSystemMdMode() {
		return saveSystemMd(items[0].Content)
	}
	// Also if system.md exists on disk and items len==1, prefer markdown save
	if len(items) == 1 {
		if _, err := os.Stat("configs/prompts/system.md"); err == nil {
			return saveSystemMd(items[0].Content)
		}
		if p := os.Getenv("PROMPTS_PATH"); p != "" {
			if info, err := os.Stat(p); err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(p), ".md") {
				return saveSystemMd(items[0].Content)
			}
		}
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	// resolve write path: PROMPTS_PATH env or first default path
	target := "configs/prompts/default.json"
	if p := os.Getenv("PROMPTS_PATH"); p != "" {
		// if PROMPTS_PATH is a dir or .md file, keep json fallback path
		if info, err := os.Stat(p); err == nil {
			if info.IsDir() {
				target = filepath.Join(p, "default.json")
			} else if strings.HasSuffix(strings.ToLower(p), ".md") {
				target = "configs/prompts/default.json"
			} else {
				target = p
			}
		} else {
			if strings.HasSuffix(strings.ToLower(p), ".md") {
				target = "configs/prompts/default.json"
			} else {
				target = p
			}
		}
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

func isSystemMdMode() bool {
	if p := os.Getenv("PROMPTS_PATH"); p != "" {
		if strings.HasSuffix(strings.ToLower(p), ".md") {
			return true
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(p, "system.md")); err == nil {
				return true
			}
		}
	}
	if _, err := os.Stat("configs/prompts/system.md"); err == nil {
		return true
	}
	return false
}

func saveSystemMd(content string) error {
	target := "configs/prompts/system.md"
	if p := os.Getenv("PROMPTS_PATH"); p != "" {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			target = filepath.Join(p, "system.md")
		} else if strings.HasSuffix(strings.ToLower(p), ".md") {
			target = p
		} else if p != "" {
			// if PROMPTS_PATH is file .md
			if strings.HasSuffix(strings.ToLower(p), ".md") {
				target = p
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	alt := filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/system.md")
	if alt != target {
		_ = os.MkdirAll(filepath.Dir(alt), 0755)
		_ = os.WriteFile(alt, []byte(content), 0644)
	}
	setDefaultCache([]PromptItem{{Role: "system", Content: content, Condition: "always"}})
	return nil
}

func ListTemplates() []PromptItem {
	return LoadDefault()
}
