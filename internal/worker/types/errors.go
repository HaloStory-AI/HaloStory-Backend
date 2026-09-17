package types

import "encoding/json"

// WorkerError is the unified error envelope for chat jobs.
// All providers return {"error": "...", "error_status_code": N} via MarkChatCompletedResilient.
type WorkerError struct {
	Message    string `json:"error"`
	StatusCode int    `json:"error_status_code"`
}

func (e *WorkerError) Error() string { return e.Message }

func (e *WorkerError) Marshal() json.RawMessage {
	b, _ := json.Marshal(map[string]interface{}{
		"error":             e.Message,
		"error_status_code": e.StatusCode,
	})
	return json.RawMessage(b)
}

// NewWorkerError creates a WorkerError with a message and HTTP status code.
func NewWorkerError(msg string, statusCode int) *WorkerError {
	return &WorkerError{Message: msg, StatusCode: statusCode}
}

// SummaryResult is the unified response for summarization jobs.
type SummaryResult struct {
	Summary            string   `json:"summary"`
	MemoryWorthKeeping []string `json:"memory_worth_keeping"`
	IsError            bool     `json:"error,omitempty"`
	ErrorStatusCode    int      `json:"error_status_code,omitempty"`
}

func (s SummaryResult) Marshal() json.RawMessage {
	m := map[string]interface{}{
		"summary":              s.Summary,
		"memory_worth_keeping": s.MemoryWorthKeeping,
	}
	if s.IsError {
		m["error"] = true
		m["error_status_code"] = s.ErrorStatusCode
	}
	b, _ := json.Marshal(m)
	return json.RawMessage(b)
}

func NewSummaryError(msg string, statusCode int) SummaryResult {
	if msg == "" {
		msg = "[ERROR 500] No content returned"
	}
	return SummaryResult{
		Summary:            msg,
		MemoryWorthKeeping: []string{},
		IsError:            true,
		ErrorStatusCode:    statusCode,
	}
}
