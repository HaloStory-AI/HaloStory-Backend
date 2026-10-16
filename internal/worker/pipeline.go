package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"time"

	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/byok"
	"halostory_go_backend/internal/services/llm"
	"halostory_go_backend/internal/services/observability"
	"halostory_go_backend/internal/worker/handlers"
	"halostory_go_backend/internal/worker/summarizer"
)

// ProcessJob is the unified entry point for a single queued job.
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

	rawJSON, _ := json.Marshal(map[string]interface{}{
		"uuid":     uuid,
		"trace_id": traceID,
		"provider": data.Provider,
	})
	_ = rawJSON
	if err := services.StoreChatResult(uuid, rawData, 10*time.Minute); err != nil {
		log.Printf("Worker: Failed to store initial payload for %s: %v", uuid, err)
	}

	data.Status = "processing"
	updateJSON, _ := json.Marshal(data)
	services.StoreProcessingResilient(uuid, updateJSON)

	if llm.IsDispatcherEnabled() {
		_ = llm.Default.Validate(data.Provider, data.Model, &data.BaseURL)
		if !llm.Allow(data.Provider, data.Model) {
			observability.Warn("llm.circuit_open", "trace_id", traceID, "provider", data.Provider, "model", data.Model)
		}
	}

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

	switch data.Provider {
	case "openai-compatible", "mistral", "deepseek", "openrouter", "zai":
		resp, statusCode, werr = handlers.HandleOpenAI(ctx, uuid, data)
	case "claude":
		resp, statusCode, werr = handlers.HandleClaude(ctx, uuid, data)
	case "gemini":
		resp, statusCode, werr = handlers.HandleGemini(ctx, uuid, data)
	default:
		markError(uuid, "Unsupported provider: "+data.Provider, 400)
		return
	}

	if werr != nil {
		markError(uuid, werr.Message, werr.StatusCode)
		return
	}
	if resp == nil && statusCode == 0 {
		return
	}

	markCompleted(uuid, resp, statusCode)
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
