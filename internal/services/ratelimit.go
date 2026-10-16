package services

// ratelimit.go — rate limiter berbasis Redis dengan in-memory fallback.
// Path normal (Redis sehat): counter & TTL disimpan di Redis, tersinkron
// antar instance.
// Saat Redis down: counter disimpan di memRateLimit (per-proses). Pada
// single-VPS ini cukup; multi-instance tanpa Redis tidak berbagi counter —
// rate limit tetap aktif namun tidak global.

import (
	"log"
	"sync"
	"time"
)

// RateLimit mencoba Redis. Jika Redis error, fallback ke in-memory counter.
// Kembalikan (allowed, retryAfter, error).
// error selalu nil setelah fallback agar caller tidak mengembalikan 500.
func RateLimit(key string, maxAttempts int64, decay time.Duration) (bool, int64, error) {
	count, err := RedisClient.Incr(RedisCtx, key).Result()
	if err != nil {
		log.Printf("RateLimit: Redis error, using in-memory fallback for %s: %v", key, err)
		return memRateLimit.Hit(key, maxAttempts, decay)
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

// ─── In-memory rate limit store ─────────────────────────────────────────────

type rateLimitEntry struct {
	count  int64
	expiry time.Time
	decay  time.Duration
}

type memRateLimitStore struct {
	mu    sync.Mutex
	items map[string]*rateLimitEntry
}

// Hit menambah counter; kembalikan (allowed, retryAfterSeconds, nil).
func (s *memRateLimitStore) Hit(key string, maxAttempts int64, decay time.Duration) (bool, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	e, ok := s.items[key]
	if !ok || now.After(e.expiry) {
		// Entry baru atau sudah expired
		s.items[key] = &rateLimitEntry{
			count:  1,
			expiry: now.Add(decay),
			decay:  decay,
		}
		return true, 0, nil
	}

	e.count++
	if e.count > maxAttempts {
		remaining := int64(e.expiry.Sub(now).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		return false, remaining, nil
	}
	return true, 0, nil
}

var memRateLimit = &memRateLimitStore{items: make(map[string]*rateLimitEntry)}
