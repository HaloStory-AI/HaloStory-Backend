package llm

import (
	"context"
	"os"
)

// Handler abstraction for Fase 2+3
type ChatRequest struct {
	Provider string
	Model    string
	TraceID  string
}

type Handler interface {
	Chat(ctx context.Context, req ChatRequest) error
}

type Dispatcher struct {
	registry *Registry
	handlers map[string]Handler
}

func NewDispatcher(registry *Registry) *Dispatcher {
	return &Dispatcher{registry: registry, handlers: make(map[string]Handler)}
}

func (d *Dispatcher) Register(provider string, h Handler) {
	d.handlers[provider] = h
}

func (d *Dispatcher) Dispatch(ctx context.Context, provider string, traceID string) error {
	if os.Getenv("USE_DISPATCHER_V2") == "false" || os.Getenv("USE_DISPATCHER_V2") == "0" {
		return nil // fallback to old switch
	}
	// Fase 2 stub: validate via registry, record metrics, then fallback to old handler
	if _, ok := d.registry.Get(provider); !ok {
		// unknown provider, allow
	}
	// No-op for now; real dispatch will call provider handlers in Fase 3
	_ = traceID
	return nil
}

func IsDispatcherEnabled() bool {
	v := os.Getenv("USE_DISPATCHER_V2")
	return v != "false" && v != "0"
}
