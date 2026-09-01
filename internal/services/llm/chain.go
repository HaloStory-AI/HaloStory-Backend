package llm

import (
	"sync"
	"time"
)

// Simple circuit breaker per provider+model (Limit 5, Window 2m) via in-memory counters
type ChainEntry struct {
	Provider string
	Model    string
	BaseURL  string
}

type breakerState struct {
	count int
	reset time.Time
	mu    sync.Mutex
}

var (
	breakers   = make(map[string]*breakerState)
	breakersMu sync.Mutex
)

const CircuitLimit = 5
const CircuitWindow = 2 * time.Minute

func Allow(provider, model string) bool {
	key := provider + ":" + model
	breakersMu.Lock()
	bs, ok := breakers[key]
	if !ok {
		bs = &breakerState{reset: time.Now().Add(CircuitWindow)}
		breakers[key] = bs
	}
	breakersMu.Unlock()
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if time.Now().After(bs.reset) {
		bs.count = 0
		bs.reset = time.Now().Add(CircuitWindow)
	}
	return bs.count < CircuitLimit
}

func RecordFailure(provider, model string) {
	key := provider + ":" + model
	breakersMu.Lock()
	bs, ok := breakers[key]
	if !ok {
		bs = &breakerState{reset: time.Now().Add(CircuitWindow)}
		breakers[key] = bs
	}
	breakersMu.Unlock()
	bs.mu.Lock()
	bs.count++
	bs.mu.Unlock()
}

func RecordSuccess(provider, model string) {
	key := provider + ":" + model
	breakersMu.Lock()
	bs, ok := breakers[key]
	breakersMu.Unlock()
	if ok {
		bs.mu.Lock()
		bs.count = 0
		bs.mu.Unlock()
	}
}
