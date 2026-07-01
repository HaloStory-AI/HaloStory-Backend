package services

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheService struct {
	rdb *redis.Client
}

func NewCacheService(rdb *redis.Client) *CacheService {
	return &CacheService{rdb: rdb}
}

func (s *CacheService) Get(key string) (string, error) {
	return s.rdb.Get(context.Background(), key).Result()
}

func (s *CacheService) Set(key string, value interface{}, ttl time.Duration) error {
	return s.rdb.Set(context.Background(), key, value, ttl).Err()
}

func (s *CacheService) Del(key string) error {
	return s.rdb.Del(context.Background(), key).Err()
}

func (s *CacheService) Incr(key string) (int64, error) {
	return s.rdb.Incr(context.Background(), key).Result()
}

func (s *CacheService) Exists(key string) (bool, error) {
	n, err := s.rdb.Exists(context.Background(), key).Result()
	return n > 0, err
}

func (s *CacheService) IncrCircuitFailure(provider string) (int64, error) {
	key := fmt.Sprintf("halostory:circuit:%s", provider)
	n, err := s.rdb.Incr(context.Background(), key).Result()
	if n == 1 {
		s.rdb.Expire(context.Background(), key, 2*time.Minute)
	}
	return n, err
}

func (s *CacheService) IsCircuitBroken(provider string) bool {
	key := fmt.Sprintf("halostory:circuit:%s", provider)
	n, err := s.rdb.Get(context.Background(), key).Int()
	if err != nil {
		return false
	}
	return n >= 5
}

func (s *CacheService) ResetCircuit(provider string) {
	key := fmt.Sprintf("halostory:circuit:%s", provider)
	s.rdb.Del(context.Background(), key)
}

// Cache response helpers — stores pre-marshaled JSON
const (
	CacheTTLPublic  = 5 * time.Minute
	CacheTTLPrivate = 2 * time.Minute
	CachePrefix     = "halostory:cache:"
)

func CacheKey(name string) string {
	return CachePrefix + name
}

// CacheResponse stores JSON bytes in Redis with a TTL.
// Returns true if stored, false if Redis is unavailable.
func (s *CacheService) CacheResponse(key string, data []byte, ttl time.Duration) bool {
	if s.rdb == nil {
		return false
	}
	return s.rdb.Set(context.Background(), key, data, ttl).Err() == nil
}

// GetCachedResponse retrieves cached JSON bytes from Redis.
// Returns nil if miss or Redis unavailable.
func (s *CacheService) GetCachedResponse(key string) []byte {
	if s.rdb == nil {
		return nil
	}
	data, err := s.rdb.Get(context.Background(), key).Bytes()
	if err != nil {
		return nil
	}
	return data
}

// InvalidateCache removes one or more cache keys.
func (s *CacheService) InvalidateCache(keys ...string) {
	if s.rdb == nil {
		return
	}
	ctx := context.Background()
	for _, key := range keys {
		s.rdb.Del(ctx, key)
	}
}

func (s *CacheService) GetKeyIndex(provider string) (int, error) {
	key := fmt.Sprintf("halostory:vip_key_idx:%s", provider)
	n, err := s.rdb.Get(context.Background(), key).Int()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *CacheService) RotateKeyIndex(provider string) {
	key := fmt.Sprintf("halostory:vip_key_idx:%s", provider)
	s.rdb.Incr(context.Background(), key)
	s.rdb.Expire(context.Background(), key, 1*time.Hour)
}

func (s *CacheService) SetChatResponse(uuid string, data interface{}) error {
	key := fmt.Sprintf("halostory:chat:%s", uuid)
	return s.Set(key, data, 10*time.Minute)
}

func (s *CacheService) GetChatResponse(uuid string) (string, error) {
	key := fmt.Sprintf("halostory:chat:%s", uuid)
	return s.Get(key)
}
