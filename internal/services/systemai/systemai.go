package systemai

import (
	"time"
)

const (
	RequestTimeout       = 50 * time.Second
	ChainTimeout         = 60 * time.Second
	CooldownDuration     = 30 * time.Second
	CircuitBreakerLimit  = 5
	CircuitBreakerWindow = 2 * time.Minute
	ChatResultTTL        = 10 * time.Minute
	DefaultMaxTokens     = 2048
	DefaultTemperature   = 0.7
	DefaultHistoryDepth  = 15
	DefaultLanguage      = "id"
	DefaultResponseLen   = 1000
)

type ProviderConfig struct {
	Label   string // e.g. "gemini", "huashang", "nvidia"
	BaseURL string // empty for Gemini native
	IsGemini bool
}

type ChainEntry struct {
	Provider string
	Model    string
	Config   ProviderConfig
}

var RegularChain = []ChainEntry{
	{Provider: "nvidia", Model: "deepseek-ai/deepseek-v4-pro", Config: ProviderConfig{Label: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1"}},
	{Provider: "nvidia", Model: "deepseek-ai/deepseek-v4-flash", Config: ProviderConfig{Label: "nvidia", BaseURL: "https://integrate.api.nvidia.com/v1"}},
	{Provider: "huashang", Model: "deepseek-ai/deepseek-v4-flash", Config: ProviderConfig{Label: "huashang", BaseURL: "https://api.huashang.ai/v1"}},
	{Provider: "huashang", Model: "deepseek-v4-flash", Config: ProviderConfig{Label: "huashang", BaseURL: "https://api.huashang.ai/v1"}},
	{Provider: "gemini", Model: "gemma-4-31b-it", Config: ProviderConfig{Label: "gemini", IsGemini: true}},
	{Provider: "gemini", Model: "gemma-4-26b-a4b-it", Config: ProviderConfig{Label: "gemini", IsGemini: true}},
	{Provider: "mistral", Model: "ministral-14b-2512", Config: ProviderConfig{Label: "mistral", BaseURL: "https://api.mistral.ai/v1"}},
	{Provider: "mistral", Model: "ministral-8b-2512", Config: ProviderConfig{Label: "mistral", BaseURL: "https://api.mistral.ai/v1"}},
	{Provider: "mistral", Model: "ministral-3b-2512", Config: ProviderConfig{Label: "mistral", BaseURL: "https://api.mistral.ai/v1"}},
	{Provider: "gemini", Model: "gemini-3.1-flash-lite-preview", Config: ProviderConfig{Label: "gemini", IsGemini: true}},
	{Provider: "gemini", Model: "gemini-3.1-flash-preview", Config: ProviderConfig{Label: "gemini", IsGemini: true}},
	{Provider: "gemini", Model: "gemini-3-flash-preview", Config: ProviderConfig{Label: "gemini", IsGemini: true}},
}

var PriorityModels = []string{
	"gemini-2.5-flash",
	"gemini-2.0-flash",
	"gemini-1.5-flash",
	"gemma-3-27b-it",
	"gemma-3-12b-it",
}

var ProviderBaseURLs = map[string]string{
	"mistral":  "https://api.mistral.ai/v1",
	"deepseek": "https://api.deepseek.com/v1",
}

var ProviderEnvKeys = map[string]string{
	"gemini":            "GEMINI_SYSTEM_KEY",
	"gemini_priority":   "GEMINI_PRIORITY_KEYS",
	"groq":              "GROQ_SYSTEM_KEY",
	"openrouter":        "OPENROUTER_SYSTEM_KEY",
	"mistral":           "MISTRAL_SYSTEM_KEY",
	"cohere":            "COHERE_SYSTEM_KEY",
	"cerebras":          "CEREBRAS_SYSTEM_KEY",
	"huashang":          "HUASHANG_SYSTEM_KEY",
	"huashang_priority": "HUASHANG_PRIORITY_KEYS",
	"nvidia":            "NVIDIA_SYSTEM_KEY",
}

type SystemAIChatParams struct {
	Messages         []map[string]string
	BotID            string
	UserID           uint
	Language         string
	Temperature      float64
	MaxTokens        int
	TopP             float64
	TopK             int
	ResponseLength   int
	Stop             []string
	Summary          string
	IsSummarize      bool
	HistoryDepth     int
	IsGroup          bool
	IsDisableBond    bool
	UserPersona      string
	BotIDs           []string
	ExistingMemories []string
}

type PingLog struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
}

type ChainResult struct {
	Response   map[string]interface{} `json:"response,omitempty"`
	Content    string                 `json:"content,omitempty"`
	StatusCode int                    `json:"status_code"`
	Error      string                 `json:"error,omitempty"`
	Pings      []PingLog              `json:"pings,omitempty"`
}
