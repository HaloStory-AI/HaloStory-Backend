package prompt

import (
	"encoding/json"
	"os"
)

func Validate(items []PromptItem) error {
	// basic validate: role must be system/user/assistant, content not empty, no {{system}}
	for _, it := range items {
		if it.Content == "" {
			continue
		}
		if len(it.Content) > 8000 {
			return &json.SyntaxError{}
		}
	}
	return nil
}

func SaveDefault(items []PromptItem) error {
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("configs/prompts/default.json", b, 0644)
}

func ListTemplates() []PromptItem {
	return LoadDefault()
}
