package llm

import (
	"net/url"
	"regexp"
	"strings"
)

type Capability struct {
	Stream       bool `json:"stream"`
	SystemRole   bool `json:"system_role"`
	Thinking     bool `json:"thinking"`
	MaxTokens    int  `json:"max_tokens"`
	ContextWindow int `json:"context_window"`
	TrustedHosts []string `json:"trusted_hosts,omitempty"`
}

type ProviderSpec struct {
	ID           string         `json:"id"`
	Label        string         `json:"label"`
	BaseURL      string         `json:"base_url,omitempty"`
	EnvKey       string         `json:"env_key,omitempty"`
	Capabilities Capability     `json:"capabilities"`
	ModelPattern *regexp.Regexp `json:"-"`
}

type Registry struct {
	providers map[string]ProviderSpec
}

var Default = NewRegistry()

func NewRegistry() *Registry {
	r := &Registry{providers: make(map[string]ProviderSpec)}
	// Hardcode 7 BYOK + system router (mirip modellist.ts hardcode)
	r.providers["gemini"] = ProviderSpec{
		ID: "gemini", Label: "Gemini", BaseURL: "https://generativelanguage.googleapis.com",
		Capabilities: Capability{Stream: true, SystemRole: true, Thinking: true, MaxTokens: 8192, ContextWindow: 128000},
		ModelPattern: regexp.MustCompile(`(?i)gemini`),
	}
	r.providers["claude"] = ProviderSpec{
		ID: "claude", Label: "Claude", BaseURL: "https://api.anthropic.com",
		Capabilities: Capability{Stream: true, SystemRole: true, Thinking: true, MaxTokens: 8192, ContextWindow: 200000, TrustedHosts: []string{"api.anthropic.com"}},
		ModelPattern: regexp.MustCompile(`(?i)claude`),
	}
	r.providers["mistral"] = ProviderSpec{
		ID: "mistral", Label: "Mistral", BaseURL: "https://api.mistral.ai/v1",
		Capabilities: Capability{Stream: true, SystemRole: true, MaxTokens: 8192, ContextWindow: 32000, TrustedHosts: []string{"api.mistral.ai"}},
		ModelPattern: regexp.MustCompile(`(?i)mistral`),
	}
	r.providers["deepseek"] = ProviderSpec{
		ID: "deepseek", Label: "DeepSeek", BaseURL: "https://api.deepseek.com/v1",
		Capabilities: Capability{Stream: true, SystemRole: true, MaxTokens: 8192, ContextWindow: 128000, TrustedHosts: []string{"api.deepseek.com"}},
		ModelPattern: regexp.MustCompile(`(?i)deepseek`),
	}
	r.providers["openrouter"] = ProviderSpec{
		ID: "openrouter", Label: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1",
		Capabilities: Capability{Stream: true, SystemRole: true, MaxTokens: 8192, ContextWindow: 128000, TrustedHosts: []string{"openrouter.ai"}},
		ModelPattern: regexp.MustCompile(`(?i)openrouter`),
	}
	r.providers["zai"] = ProviderSpec{
		ID: "zai", Label: "ZAI", BaseURL: "https://api.z.ai/api/paas/v4",
		Capabilities: Capability{Stream: true, SystemRole: true, MaxTokens: 8192, ContextWindow: 32000, TrustedHosts: []string{"api.z.ai"}},
		ModelPattern: regexp.MustCompile(`(?i)zai|glm`),
	}
	r.providers["openai-compatible"] = ProviderSpec{
		ID: "openai-compatible", Label: "Custom OpenAI", BaseURL: "",
		Capabilities: Capability{Stream: true, SystemRole: true, MaxTokens: 8192, ContextWindow: 128000},
		ModelPattern: regexp.MustCompile(`.*`),
	}
	r.providers["mock"] = ProviderSpec{
		ID: "mock", Label: "Mock", Capabilities: Capability{Stream: false, SystemRole: false, MaxTokens: 1024},
	}
	r.providers["system-ai"] = ProviderSpec{
		ID: "system-ai", Label: "System AI", BaseURL: "https://router.my-bini.my.id/v1",
		Capabilities: Capability{Stream: false, SystemRole: true, MaxTokens: 32000, ContextWindow: 32000},
	}
	return r
}

func (r *Registry) Get(provider string) (ProviderSpec, bool) {
	p, ok := r.providers[provider]
	return p, ok
}

func (r *Registry) List() []ProviderSpec {
	out := make([]ProviderSpec, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	return out
}

func (r *Registry) Validate(provider, model string, baseURL *string) error {
	if _, ok := r.providers[provider]; !ok {
		// unknown provider — allow but log
		return nil
	}
	// For openai-compatible, check TrustedBaseURLs via helper
	if provider == "openai-compatible" && baseURL != nil && *baseURL != "" {
		if !IsTrustedBaseURL(*baseURL) {
			// not trusted but still allow for admin; validator will handle injection guard
			return nil
		}
	}
	return nil
}

func IsTrustedBaseURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Host)
	trusted := []string{
		"api.openai.com", "api.deepseek.com", "api.mistral.ai", "openrouter.ai",
		"api.z.ai", "api.anthropic.com", "api.groq.com", "api.cohere.ai",
		"api.cerebras.ai", "integrate.api.nvidia.com", "api.huashang.ai",
	}
	for _, t := range trusted {
		if host == t || strings.HasSuffix(host, "."+t) {
			return true
		}
	}
	return false
}
