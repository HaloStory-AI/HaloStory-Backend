package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const chatTTL = 10 * time.Minute

type ChatStatus string

const (
	ChatStatusQueued     ChatStatus = "queued"
	ChatStatusProcessing ChatStatus = "processing"
	ChatStatusCompleted  ChatStatus = "completed"
	ChatStatusNotFound   ChatStatus = "not_found"
)

type ChatPayload struct {
	Status     ChatStatus      `json:"status"`
	UserID     uint            `json:"user_id,omitempty"`
	BotID      string          `json:"bot_id,omitempty"`
	IsGroup    bool            `json:"is_group,omitempty"`
	IsPriority bool            `json:"is_priority,omitempty"`
	Response   json.RawMessage `json:"response,omitempty"`
	StatusCode int             `json:"status_code,omitempty"`
	TraceID    string          `json:"trace_id,omitempty"`
}

func Enqueue(queueName string, payload []byte) error {
	key := fmt.Sprintf("queue:%s", queueName)
	return RedisClient.RPush(RedisCtx, key, payload).Err()
}

func Dequeue(queueName string, timeout time.Duration) (string, []byte, error) {
	key := fmt.Sprintf("queue:%s", queueName)
	result, err := RedisClient.BLPop(RedisCtx, timeout, key).Result()
	if err != nil {
		return "", nil, err
	}
	return result[0], []byte(result[1]), nil
}

func StoreChatResult(uuid string, result []byte, ttl time.Duration) error {
	key := fmt.Sprintf("chat:%s", uuid)
	// return RedisClient.SetEx(RedisCtx, key, string(result), ttl).Err()
	return RedisClient.Set(RedisCtx, key, string(result), ttl).Err()
}

func GetChatResult(uuid string) ([]byte, error) {
	key := fmt.Sprintf("chat:%s", uuid)
	return RedisClient.Get(RedisCtx, key).Bytes()
}

func DeleteChatResult(uuid string) error {
	key := fmt.Sprintf("chat:%s", uuid)
	return RedisClient.Del(RedisCtx, key).Err()
}

// redis.call('SETEX', KEYS[1], ARGV[4], cjson.encode(payload))
const updateStatusScript = `
local data = redis.call('GET', KEYS[1])
if not data then return nil end
local payload = cjson.decode(data)
payload['status'] = ARGV[1]
if ARGV[2] ~= '' then
	payload['response'] = cjson.decode(ARGV[2])
	payload['status_code'] = tonumber(ARGV[3])
end
redis.call('SET', KEYS[1], cjson.encode(payload), 'EX', ARGV[4])
return "OK"
`

func UpdateChatStatus(uuid string, status ChatStatus) error {
	key := fmt.Sprintf("chat:%s", uuid)
	err := RedisClient.Eval(RedisCtx, updateStatusScript, []string{key},
		string(status), "", "0", int(chatTTL.Seconds())).Err()
	if err == redis.Nil {
		return nil
	}
	return err
}

func MarkChatCompleted(uuid string, response json.RawMessage, statusCode int) error {
	key := fmt.Sprintf("chat:%s", uuid)
	err := RedisClient.Eval(RedisCtx, updateStatusScript, []string{key},
		string(ChatStatusCompleted), string(response), statusCode, int(chatTTL.Seconds())).Err()
	if err == redis.Nil {
		log.Printf("Queue: chat:%s not found for markCompleted", uuid)
		return nil
	}
	return err
}

func SetCooldown(userID uint, ttl time.Duration) error {
	key := fmt.Sprintf("ai-chat-cooldown:%d", userID)
	return RedisClient.SetEx(RedisCtx, key, "1", ttl).Err()
}

func CheckCooldown(userID uint) (bool, error) {
	key := fmt.Sprintf("ai-chat-cooldown:%d", userID)
	_, err := RedisClient.Get(RedisCtx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func PushPending(userID uint, uuid string) error {
	key := fmt.Sprintf("ai-chat-pending:%d", userID)
	return RedisClient.RPush(RedisCtx, key, uuid).Err()
}

func PopPending(userID uint) (string, error) {
	key := fmt.Sprintf("ai-chat-pending:%d", userID)
	return RedisClient.LPop(RedisCtx, key).Result()
}
