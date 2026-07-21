package services

import (
	"log"
	"time"
)

func RateLimit(key string, maxAttempts int64, decay time.Duration) (bool, int64, error) {
	count, err := RedisClient.Incr(RedisCtx, key).Result()
	if err != nil {
		return false, 0, err
	}

	if count == 1 {
		if err := RedisClient.Expire(RedisCtx, key, decay).Err(); err != nil {
			log.Printf("RateLimit: failed to set expiry on %s: %v", key, err)
		}
	}

	if count > maxAttempts {
		ttl, _ := RedisClient.TTL(RedisCtx, key).Result()
		return false, int64(ttl.Seconds()), nil
	}

	return true, 0, nil
}
