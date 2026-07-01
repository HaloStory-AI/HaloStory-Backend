package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	TypeCacheInvalidation = "cache:invalidate"
)

type CacheInvalidationPayload struct {
	Keys []string `json:"keys"`
}

func NewCacheInvalidationTask(keys ...string) (*asynq.Task, error) {
	payload, err := json.Marshal(CacheInvalidationPayload{Keys: keys})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCacheInvalidation, payload, asynq.MaxRetry(3)), nil
}

type CacheInvalidationProcessor struct {
	rdb *redis.Client
}

func NewCacheInvalidationProcessor(rdb *redis.Client) *CacheInvalidationProcessor {
	return &CacheInvalidationProcessor{rdb: rdb}
}

func (p *CacheInvalidationProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload CacheInvalidationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	if p.rdb == nil {
		return nil
	}

	for _, key := range payload.Keys {
		if err := p.rdb.Del(ctx, key).Err(); err != nil {
			return fmt.Errorf("failed to delete cache key %s: %w", key, err)
		}
	}

	return nil
}
