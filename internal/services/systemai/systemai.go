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

	RouterBaseURL = "https://router.my-bini.my.id/v1"
	FreeUserModel = "Free_User"
	VipUserModel  = "VIP_User"
)

type ProviderConfig struct {
	Label   string // e.g. "router"
	BaseURL string
}

type ChainEntry struct {
	Provider string
	Model    string
	Config   ProviderConfig
}

type SystemAIChatParams struct {
	Messages          []map[string]string
	BotID             string
	UserID            uint
	Language          string
	Temperature       float64
	MaxTokens         int
	TopP              float64
	TopK              int
	ResponseLength    int
	Stop              []string
	Summary           string
	IsSummarize       bool
	HistoryDepth      int
	IsGroup           bool
	IsDisableBond     bool
	UserPersona       string
	BotIDs            []string
	ExistingMemories  []string
	PresencePenalty   float64
	FrequencyPenalty  float64
	MinP              float64
	RepetitionPenalty float64
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
