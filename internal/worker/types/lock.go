package types

import (
	"fmt"
	"log"
	"time"

	"halostory_go_backend/internal/services"
)

// AcquireChatLock tries to acquire the per-user/chat overlapping lock.
// Returns (acquired=true, nil) on success; (false, nil) if already held (caller should drop the job);
// (false, err) only on Redis error where caller still decides to proceed.
func AcquireChatLock(userID uint, botID string, isGroup bool) (bool, error) {
	if userID == 0 {
		return true, nil
	}
	lockKey := fmt.Sprintf("ai-chat-lock:%d:%s", userID, botID)
	if isGroup {
		lockKey = fmt.Sprintf("ai-chat-lock:%d:group", userID)
	}
	locked, err := services.RedisClient.SetNX(services.RedisCtx, lockKey, "1", 120*time.Second).Result()
	if err != nil {
		log.Printf("Worker: Lock check failed for %s: %v", lockKey, err)
		return false, err
	}
	return locked, nil
}

// ReleaseChatLock releases the overlapping lock. Only call if AcquireChatLock returned acquired=true.
func ReleaseChatLock(userID uint, botID string, isGroup bool) {
	if userID == 0 {
		return
	}
	lockKey := fmt.Sprintf("ai-chat-lock:%d:%s", userID, botID)
	if isGroup {
		lockKey = fmt.Sprintf("ai-chat-lock:%d:group", userID)
	}
	_ = services.RedisClient.Del(services.RedisCtx, lockKey).Err()
}
