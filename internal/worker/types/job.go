package types

import (
	"encoding/json"
	"fmt"
)

type ReasoningConfig struct {
	Enabled   bool    `json:"enabled"`
	Effort    string  `json:"effort,omitempty"`
	MaxTokens float64 `json:"max_tokens,omitempty"`
	Exclude   bool    `json:"exclude,omitempty"`
}

type ChatData struct {
	Status           string           `json:"status"`
	APIKey           string           `json:"api_key"`
	Messages         string           `json:"messages"`
	Model            string           `json:"model"`
	Provider         string           `json:"provider"`
	BaseURL          string           `json:"base_url,omitempty"`
	BotID            string           `json:"bot_id"`
	Language         string           `json:"language,omitempty"`
	Temperature      float64          `json:"temperature,omitempty"`
	MaxTokens        float64          `json:"max_tokens,omitempty"`
	TopP             float64          `json:"top_p,omitempty"`
	TopK             float64          `json:"top_k,omitempty"`
	PresencePenalty  float64          `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64          `json:"frequency_penalty,omitempty"`
	Summary          string           `json:"summary,omitempty"`
	IsSummarize      bool             `json:"is_summarize,omitempty"`
	HistoryDepth     float64          `json:"history_depth,omitempty"`
	IsGroup          bool             `json:"is_group,omitempty"`
	UserPersona      string           `json:"user_persona,omitempty"`
	UserName         string           `json:"user_name,omitempty"`
	BotIDs           []string         `json:"bot_ids,omitempty"`
	ExistingMemories []string         `json:"existing_memories,omitempty"`
	Reasoning        *ReasoningConfig `json:"reasoning,omitempty"`
	Stop             []string         `json:"stop,omitempty"`
	ResponseLength   float64          `json:"response_length,omitempty"`
	TraceID          string           `json:"trace_id,omitempty"`
	ContextLimit     float64          `json:"context_limit,omitempty"`
	IsStream         bool             `json:"is_stream,omitempty"`
}

func DecodeChatData(rawData []byte) (ChatData, string, string, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(rawData, &payload); err != nil {
		return ChatData{}, "", "", fmt.Errorf("parse job payload: %w", err)
	}
	uuid, _ := payload["uuid"].(string)
	traceID, _ := payload["trace_id"].(string)
	jb, _ := json.Marshal(payload)
	var data ChatData
	if err := json.Unmarshal(jb, &data); err != nil {
		return ChatData{}, uuid, traceID, fmt.Errorf("parse chat data: %w", err)
	}
	return data, uuid, traceID, nil
}
