package tokenizer

import (
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

type tiktokenWrapper struct {
	once      sync.Once
	available bool
	mu        sync.Mutex
	encs      map[string]*tiktoken.Tiktoken
}

func newTiktokenWrapper() *tiktokenWrapper {
	return &tiktokenWrapper{encs: make(map[string]*tiktoken.Tiktoken)}
}

func (w *tiktokenWrapper) Available() bool {
	w.once.Do(func() {
		w.available = true
	})
	return w.available
}

func (w *tiktokenWrapper) Count(text string, model string) (int, error) {
	encName := encodingForModel(model)
	enc, err := w.getEncoding(encName)
	if err != nil {
		return heuristicCount(text, model), nil
	}
	tokens := enc.Encode(text, nil, nil)
	return len(tokens), nil
}

func (w *tiktokenWrapper) getEncoding(name string) (*tiktoken.Tiktoken, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e, ok := w.encs[name]; ok {
		return e, nil
	}
	e, err := tiktoken.GetEncoding(name)
	if err != nil {
		return nil, err
	}
	w.encs[name] = e
	return e, nil
}

func encodingForModel(model string) string {
	lm := toLower(model)
	// o200k for gpt-4o, o1, o3
	if containsFold(lm, "gpt-4o") || containsFold(lm, "o1") || containsFold(lm, "o3") || containsFold(lm, "o200") {
		return "o200k_base"
	}
	// cl100k for gpt-3.5, gpt-4
	return "cl100k_base"
}
