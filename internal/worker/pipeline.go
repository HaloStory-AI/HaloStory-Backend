package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services/llm"
	"halostory_go_backend/internal/services/observability"
	"halostory_go_backend/internal/services/systemai"
	"halostory_go_backend/internal/worker/handlers"
	"halostory_go_backend/internal/worker/summarizer"
)

// ProcessJob is the unified entry point for a single queued job.
// It handles decode, observability, activity logging, dispatch, and completion.
func ProcessJob(rawData []byte) {
	start := time.Now()
	data, uuid, traceID, err := DecodeChatData(rawData)
	if err != nil {
		if uuid == "" {
			log.Printf("Worker: Failed to parse job payload: %v", err)
			return
		}
		markError(uuid, "Failed to parse chat data", 500)
		return
	}
	if uuid == "" {
		log.Printf("Worker: Missing uuid in payload")
		return
	}
	if traceID != "" && observability.ShouldSample() {
		observability.Info("worker.job.start", "trace_id", traceID, "uuid", uuid, "provider", data.Provider)
	}

	// Store initial payload
	rawJSON, _ := json.Marshal(map[string]interface{}{
		"uuid":     uuid,
		"trace_id": traceID,
		"provider": data.Provider,
	})
	_ = rawJSON // for later trace preservation
	if err := services.StoreChatResult(uuid, rawData, 10*time.Minute); err != nil {
		log.Printf("Worker: Failed to store initial payload for %s: %v", uuid, err)
	}

	// Mark processing
	data.Status = "processing"
	updateJSON, _ := json.Marshal(data)
	services.StoreProcessingResilient(uuid, updateJSON)

	// Fire-and-forget activity
	if data.UserID > 0 {
		userID := uint(data.UserID)
		at := "chat"
		if data.IsSummarize {
			at = "chat_summarize"
		}
		botID := data.BotID
		prov := data.Provider
		go func(uid uint, a, b, p string) {
			var bID, provPtr *string
			if b != "" {
				bID = &b
			}
			if p != "" {
				provPtr = &p
			}
			_ = database.DB.Create(&models.UserActivity{UserID: uid, ActivityType: a, BotID: bID, Provider: provPtr}).Error
		}(userID, at, botID, prov)
	}

	// Dispatcher validate
	if llm.IsDispatcherEnabled() {
		_ = llm.Default.Validate(data.Provider, data.Model, &data.BaseURL)
		if !llm.Allow(data.Provider, data.Model) {
			observability.Warn("llm.circuit_open", "trace_id", traceID, "provider", data.Provider, "model", data.Model)
		}
	}

	// Dispatch by provider
	if data.IsSummarize {
		handleSummarize(uuid, data, traceID)
	} else {
		handleChat(uuid, data, traceID)
	}

	latency := time.Since(start)
	observability.ObserveLatency(data.Provider, latency)
	observability.IncRequest(data.Provider, data.Model, "completed")
	if traceID != "" {
		observability.Info("worker.job.done", "trace_id", traceID, "uuid", uuid, "provider", data.Provider, "model", data.Model, "latency_ms", latency.Milliseconds())
	}
}

func handleSummarize(uuid string, data ChatData, traceID string) {
	var result SummaryResult
	var statusCode int

	switch data.Provider {
	case "gemini":
		decrypted, err := handlers.DecryptKey(data.APIKey)
		if err != nil {
			markSummaryCompleted(uuid, NewSummaryError(err.Message, err.StatusCode), err.StatusCode)
			return
		}
		result, statusCode = summarizer.HandleGemini(data, decrypted)
	case "openai-compatible", "mistral", "deepseek", "openrouter", "zai":
		decrypted, err := handlers.DecryptKey(data.APIKey)
		if err != nil {
			markSummaryCompleted(uuid, NewSummaryError(err.Message, err.StatusCode), err.StatusCode)
			return
		}
		messagesJSON, _ := base64.StdEncoding.DecodeString(data.Messages)
		var messages []byok.Message
		_ = json.Unmarshal(messagesJSON, &messages)
		model := data.Model
		if model == "" {
			model = "gpt-3.5-turbo"
		}
		result, statusCode = summarizer.HandleOpenAI(data, decrypted, messages, model, data.BaseURL)
	case "claude":
		decrypted, err := handlers.DecryptKey(data.APIKey)
		if err != nil {
			markSummaryCompleted(uuid, NewSummaryError(err.Message, err.StatusCode), err.StatusCode)
			return
		}
		messagesJSON, _ := base64.StdEncoding.DecodeString(data.Messages)
		var messages []byok.Message
		_ = json.Unmarshal(messagesJSON, &messages)
		result, statusCode = summarizer.HandleClaude(data, decrypted, messages, messagesJSON)
	case "system-ai":
		messagesJSON, _ := base64.StdEncoding.DecodeString(data.Messages)
		var messages []byok.Message
		_ = json.Unmarshal(messagesJSON, &messages)
		userID := uint(data.UserID)
		result, statusCode = summarizer.HandleSystemAI(data, messages, userID)
	default:
		markError(uuid, "Unsupported provider: "+data.Provider, 400)
		return
	}

	markSummaryCompleted(uuid, result, statusCode)
}

func handleChat(uuid string, data ChatData, traceID string) {
	ctx := context.Background()
	var resp map[string]interface{}
	var statusCode int
	var werr *WorkerError
	var systemAIUserID uint

	switch data.Provider {
	case "openai-compatible", "mistral", "deepseek", "openrouter", "zai":
		resp, statusCode, werr = handlers.HandleOpenAI(ctx, uuid, data)
	case "claude":
		resp, statusCode, werr = handlers.HandleClaude(ctx, uuid, data)
	case "gemini":
		resp, statusCode, werr = handlers.HandleGemini(ctx, uuid, data)
	case "system-ai":
		resp, statusCode, werr, systemAIUserID = handlers.HandleSystemAI(ctx, uuid, data)
	default:
		markError(uuid, "Unsupported provider: "+data.Provider, 400)
		return
	}

	if werr != nil {
		markError(uuid, werr.Message, werr.StatusCode)
		return
	}
	if resp == nil && statusCode == 0 {
		// Dropped due to overlapping lock — no completion
		return
	}

	if data.Provider == "system-ai" {
		markSystemAICompleted(uuid, resp, statusCode, systemAIUserID)
	} else {
		markCompleted(uuid, resp, statusCode)
	}
}

func markCompleted(uuid string, response interface{}, statusCode int) {
	respJSON, _ := json.Marshal(response)
	_ = services.MarkChatCompletedResilient(uuid, json.RawMessage(respJSON), statusCode)
	log.Printf("Worker: Completed job %s with status %d", uuid, statusCode)
}

func markError(uuid, message string, statusCode int) {
	werr := NewWorkerError(message, statusCode)
	_ = services.MarkChatCompletedResilient(uuid, werr.Marshal(), statusCode)
	log.Printf("Worker: Error job %s: %s (status %d)", uuid, message, statusCode)
}

func markSummaryCompleted(uuid string, result SummaryResult, statusCode int) {
	_ = services.MarkChatCompletedResilient(uuid, result.Marshal(), statusCode)
	log.Printf("Worker: Completed summarization job %s with status %d", uuid, statusCode)
}

func markSystemAICompleted(uuid string, response map[string]interface{}, statusCode int, userID uint) {
	payload := map[string]interface{}{
		"status":      "completed",
		"response":    response,
		"status_code": statusCode,
	}
	if existing, err := services.GetChatResultResilient(uuid); err == nil {
		var existingMap map[string]interface{}
		if json.Unmarshal(existing, &existingMap) == nil {
			if tid, ok := existingMap["trace_id"].(string); ok && tid != "" {
				payload["trace_id"] = tid
			}
		}
	}
	jsonBytes, _ := json.Marshal(payload)
	if err := services.StoreChatResult(uuid, jsonBytes, 10*time.Minute); err != nil {
		inner, _ := json.Marshal(response)
		if derr := services.MarkChatCompletedFallback(uuid, json.RawMessage(inner), statusCode); derr != nil {
			log.Printf("Worker: System AI fallback store gagal untuk %s: %v", uuid, derr)
		}
	}
	log.Printf("Worker: Completed system AI job %s with status %d", uuid, statusCode)

	// Drain pending queue
	nextUUID, err := services.PopPending(userID)
	if err != nil || nextUUID == "" {
		return
	}
	nextData, err := services.GetChatResultResilient(nextUUID)
	if err != nil {
		log.Printf("Worker: Pending data for %s not found, skipping", nextUUID)
		return
	}
	var nextPayload map[string]interface{}
	if err := json.Unmarshal(nextData, &nextPayload); err != nil {
		log.Printf("Worker: Failed to parse pending payload for %s: %v", nextUUID, err)
		return
	}
	nextPayload["uuid"] = nextUUID
	enqBytes, _ := json.Marshal(nextPayload)
	_ = services.EnqueueResilient("ai-chat", nextUUID, enqBytes)
	_ = services.SetCooldown(userID, systemai.CooldownDuration)
	log.Printf("Worker: Dispatched pending job %s from queue", nextUUID)
}
