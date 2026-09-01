package tokenizer

import (
	"hash/crc32"
	"sync"
)

// Simple LRU-ish cache (map + mutex, evict oldest when > max)
type lruCache struct {
	mu   sync.Mutex
	m    map[string]int
	keys []string
	max  int
}

func newLRU(max int) *lruCache {
	return &lruCache{m: make(map[string]int), max: max}
}

func (c *lruCache) Get(k string) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}

func (c *lruCache) Set(k string, v int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.m[k]; !ok {
		c.keys = append(c.keys, k)
		if len(c.keys) > c.max {
			oldest := c.keys[0]
			c.keys = c.keys[1:]
			delete(c.m, oldest)
		}
	}
	c.m[k] = v
}

type Router struct {
	cache *lruCache
	// tiktoken lazy (nil until first use if dep available)
	tiktoken *tiktokenWrapper
}

func NewRouter() Router {
	return Router{
		cache:    newLRU(2048),
		tiktoken: newTiktokenWrapper(),
	}
}

func init() {
	Default = NewRouter()
}

func (r *Router) CountTokens(text string, model string) int {
	if text == "" {
		return 0
	}
	key := model + ":" + crcKey(text, model)
	if v, ok := r.cache.Get(key); ok {
		return v
	}
	var n int
	// Try tiktoken for GPT/OpenAI-like models
	if r.tiktoken != nil && r.tiktoken.Available() && shouldUseTiktoken(model) {
		if cnt, err := r.tiktoken.Count(text, model); err == nil {
			n = cnt
		} else {
			n = heuristicCount(text, model)
		}
	} else {
		n = heuristicCount(text, model)
	}
	r.cache.Set(key, n)
	return n
}

func (r *Router) CountChatTokens(messages []ChatMessage, model string) int {
	overhead := 3
	if isGptModel(model) {
		overhead = 5 // RisuAI ChatTokenizer 5 for gpt, 3 else
	}
	total := 0
	for _, m := range messages {
		// content + role overhead + name (not used in HaloStory, keep +1)
		total += r.CountTokens(m.Content, model) + overhead + 1
		if m.Role == "system" {
			total += 1 // extra for system
		}
	}
	return total
}

func (r *Router) TruncateToBudget(messages []ChatMessage, budget int, model string) []ChatMessage {
	if budget <= 0 || len(messages) == 0 {
		return messages
	}
	// Fast path: if within budget keep all
	if r.CountChatTokens(messages, model) <= budget {
		return messages
	}
	// From back, keep until budget
	kept := make([]ChatMessage, 0, len(messages))
	used := 0
	for i := len(messages) - 1; i >= 0; i-- {
		cost := r.CountTokens(messages[i].Content, model) + 4
		if used+cost > budget && len(kept) > 0 {
			break
		}
		kept = append([]ChatMessage{messages[i]}, kept...)
		used += cost
		if used >= budget {
			break
		}
	}
	if len(kept) == 0 && len(messages) > 0 {
		kept = []ChatMessage{messages[len(messages)-1]}
	}
	return kept
}

// Helpers to convert []byok.Message <-> []ChatMessage without import cycle (called via reflection if needed)
func ToChatMessages(messages []struct{ Role, Content string }) []ChatMessage {
	out := make([]ChatMessage, len(messages))
	for i, m := range messages {
		out[i] = ChatMessage{Role: m.Role, Content: m.Content}
	}
	return out
}

func crcKey(text, model string) string {
	h := crc32.ChecksumIEEE([]byte(text + "|" + model))
	// quick hex
	const hex = "0123456789abcdef"
	b := make([]byte, 8)
	for i := 0; i < 4; i++ {
		b[i*2] = hex[(h>>uint(28-i*8))&0xF]
		b[i*2+1] = hex[(h>>uint(24-i*8))&0xF]
	}
	return string(b)
}

func shouldUseTiktoken(model string) bool {
	// Use tiktoken for GPT/openai-compatible; heuristic for others unless tiktoken supports
	if isGptModel(model) {
		return true
	}
	// OpenRouter/Mistral/DeepSeek often gpt-like tokenization; allow tiktoken fallback
	if containsFold(model, "mistral") || containsFold(model, "deepseek") || containsFold(model, "openrouter") || containsFold(model, "zai") {
		return true
	}
	return false
}

func (r *Router) IsTiktoken(model string) bool {
	return shouldUseTiktoken(model) && r.tiktoken != nil && r.tiktoken.Available()
}
