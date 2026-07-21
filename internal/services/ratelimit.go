package services

import (
	"context"
	"time"
)

func RateLimit(key string, maxAttempts int64, decay time.Duration) (bool, int64, error) {
	ctx := context.Background()

	count, err := RedisClient.Incr(ctx, key).Result()
	if err != nil {
		return false, 0, err
	}

	if count == 1 {
		RedisClient.Expire(ctx, key, decay)
	}

	if count > maxAttempts {
		ttl, _ := RedisClient.TTL(ctx, key).Result()
		return false, int64(ttl.Seconds()), nil
	}

	return true, 0, nil
}
