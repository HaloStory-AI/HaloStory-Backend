package prompt

import (
	"os"
	"path/filepath"
	"strings"
)

// NormalizeLegacySyntax converts legacy PHP placeholders to Go text/template vars.
// Handles: {$char} -> {{.CharName}}, {$user} -> {{.UserName}}, {{char}} -> {{.CharName}}, {{user}} -> {{.UserName}}
// Also strips escaped chars like \< -> < and \_ -> _
func NormalizeLegacySyntax(content string) string {
	// Unescape markdown escapes: \< -> <, \_ -> _
	content = strings.ReplaceAll(content, `\<`, `<`)
	content = strings.ReplaceAll(content, `\>`, `>`)
	content = strings.ReplaceAll(content, `\_`, `_`)
	// Legacy PHP vars
	content = strings.ReplaceAll(content, `{$char}`, `{{.CharName}}`)
	content = strings.ReplaceAll(content, `{$user}`, `{{.UserName}}`)
	content = strings.ReplaceAll(content, `{$language}`, `{{.Language}}`)
	// Double-brace legacy (Risu/Jailbreak style)
	content = strings.ReplaceAll(content, `{{char}}`, `{{.CharName}}`)
	content = strings.ReplaceAll(content, `{{user}}`, `{{.UserName}}`)
	// Fix HasAIHelpers.php typo: {$char}} -> {{.CharName}}
	content = strings.ReplaceAll(content, `{{.CharName}}}`, `{{.CharName}}`)
	return content
}

// tryLoadSystemFile attempts to load a single system.md file and return as one PromptItem
func tryLoadSystemFile(path string) ([]PromptItem, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	content := strings.TrimSpace(string(b))
	if content == "" {
		return nil, false
	}
	// Normalize legacy syntax
	content = NormalizeLegacySyntax(content)
	if content == "" {
		return nil, false
	}
	items := []PromptItem{
		{Role: "system", Content: content, Condition: "always"},
	}
	return items, true
}

// resolveSystemPaths returns candidate paths for system.md (single-file mode)
// Supports PROMPTS_PATH as file or dir
func resolveSystemPaths() []string {
	if p := os.Getenv("PROMPTS_PATH"); p != "" {
		// Check if p is a file with .md extension or existing file
		if strings.HasSuffix(strings.ToLower(p), ".md") {
			return []string{
				p,
				filepath.Join(p, "system.md"),
				"configs/prompts/system.md",
				"./configs/prompts/system.md",
				filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/system.md"),
			}
		}
		// Try stat to detect dir vs file
		if info, err := os.Stat(p); err == nil {
			if info.IsDir() {
				return []string{
					filepath.Join(p, "system.md"),
					"configs/prompts/system.md",
					"./configs/prompts/system.md",
					filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/system.md"),
				}
			}
			// is file
			return []string{
				p,
				"configs/prompts/system.md",
				"./configs/prompts/system.md",
				filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/system.md"),
			}
		}
		// fallback: treat as file path
		return []string{
			p,
			filepath.Join(p, "system.md"),
			"configs/prompts/system.md",
			"./configs/prompts/system.md",
			filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/system.md"),
		}
	}
	return []string{
		"configs/prompts/system.md",
		"./configs/prompts/system.md",
		filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/system.md"),
	}
}

// tryLoadJailbreakFile attempts to load jailbreak.md as one PromptItem (global always)
func tryLoadJailbreakFile(path string) ([]PromptItem, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	content := strings.TrimSpace(string(b))
	if content == "" {
		return nil, false
	}
	content = NormalizeLegacySyntax(content)
	if content == "" {
		return nil, false
	}
	items := []PromptItem{
		{Role: "system", Content: content, Condition: "always"},
	}
	return items, true
}

// resolveJailbreakPaths returns candidate paths for jailbreak.md (global file, configs/prompts only)
func resolveJailbreakPaths() []string {
	return []string{
		"configs/prompts/jailbreak.md",
		"./configs/prompts/jailbreak.md",
		filepath.Join(filepath.Dir(os.Args[0]), "configs/prompts/jailbreak.md"),
	}
}

// LoadJailbreakMd tries to load jailbreak.md global file, returns nil if not found or empty
func LoadJailbreakMd() []PromptItem {
	paths := resolveJailbreakPaths()
	for _, p := range paths {
		if items, ok := tryLoadJailbreakFile(p); ok {
			return items
		}
	}
	return nil
}

// LoadSystemMd tries to load system.md single file, returns nil if not found
func LoadSystemMd() []PromptItem {
	paths := resolveSystemPaths()
	for _, p := range paths {
		if items, ok := tryLoadSystemFile(p); ok {
			return items
		}
	}
	return nil
}
