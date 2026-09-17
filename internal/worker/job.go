package worker

import "halostory_go_backend/internal/worker/types"

// Re-export types for backward compat (main.go and external callers use worker.ChatData).
type ChatData = types.ChatData
type ReasoningConfig = types.ReasoningConfig

var DecodeChatData = types.DecodeChatData
