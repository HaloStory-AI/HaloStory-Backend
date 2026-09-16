package systemai

import (
	"crypto/rand"
	"math/big"
	mrand "math/rand"
	"strings"
	"time"

	"halostory_go_backend/internal/config"
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

	NvidiaBaseURL = "https://integrate.api.nvidia.com/v1"
)

func GetNvidiaKeys() []string {
	raw := config.Env("NVIDIA_SYSTEM_KEYS", "")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func PickRandomKey(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	if len(keys) == 1 {
		return keys[0]
	}
	if n, err := rand.Int(rand.Reader, big.NewInt(int64(len(keys)))); err == nil {
		return keys[n.Int64()]
	}
	return keys[mrand.Intn(len(keys))]
}

func RequireNvidiaModel(isVip bool) (string, string) {
	if isVip {
		if m := strings.TrimSpace(config.Env("NVIDIA_VIP_MODEL", "")); m != "" {
			return m, ""
		}
		return "", "nvidia vip model not configured (NVIDIA_VIP_MODEL)"
	}
	if m := strings.TrimSpace(config.Env("NVIDIA_FREE_MODEL", "")); m != "" {
		return m, ""
	}
	return "", "nvidia free model not configured (NVIDIA_FREE_MODEL)"
}

type ProviderConfig struct {
	Label   string // e.g. "router"
	BaseURL string
}

type ChainEntry struct {
	Provider         string
	Model            string
	Config           ProviderConfig
	Temperature      float64
	TopP             float64
	MaxTokens        int
	PresencePenalty  float64
	FrequencyPenalty float64
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
