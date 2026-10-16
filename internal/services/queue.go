package services

import (
	"context"
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

// ─── Streaming SSE: Redis Pub/Sub + buffer, fallback in-process ───
// Saat Redis sehat: delta dikirim via PUBLISH + di-buffer di List Redis.
// Saat Redis down: delta dikirim lewat memStream (in-process channel store).
// Single-instance: SSE tetap berfungsi penuh tanpa Redis.
// Multi-instance: delta tidak diteruskan antar proses; klien mendapat hasil
// akhir via polling watchdog (watchdog 5 dtk di stream_controller).

type StreamChunk struct {
	UUID         string  `json:"uuid"`
	Delta        string  `json:"delta"`
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason,omitempty"`
}

const streamBufTTL = 5 * time.Minute
const streamBufMax = 1000

func streamChannel(uuid string) string {
	return "chat:stream:" + uuid
}

func streamBufKey(uuid string) string {
	return "chat:stream:buf:" + uuid
}

func PublishStream(uuid string, delta string) error {
	if delta == "" {
		return nil
	}
	if IsRedisUp() {
		// PUBLISH untuk live subscriber + buffer untuk replay
		_ = RedisClient.Publish(RedisCtx, streamChannel(uuid), delta).Err()
		pipe := RedisClient.Pipeline()
		pipe.LPush(RedisCtx, streamBufKey(uuid), delta)
		pipe.LTrim(RedisCtx, streamBufKey(uuid), 0, streamBufMax-1)
		pipe.Expire(RedisCtx, streamBufKey(uuid), streamBufTTL)
		_, _ = pipe.Exec(RedisCtx)
		return nil
	}
	// Redis down → in-process pub/sub
	memStream.Publish(uuid, delta)
	return nil
}

func PublishStreamDone(uuid string) error {
	if IsRedisUp() {
		_ = RedisClient.Publish(RedisCtx, streamChannel(uuid), "[DONE]").Err()
		return nil
	}
	memStream.Publish(uuid, "[DONE]")
	return nil
}

func GetStreamBuffer(uuid string) []string {
	if IsRedisUp() {
		vals, err := RedisClient.LRange(RedisCtx, streamBufKey(uuid), 0, -1).Result()
		if err != nil {
			return nil
		}
		// LPUSH stores newest first, reverse to chronological
		for i, j := 0, len(vals)-1; i < j; i, j = i+1, j-1 {
			vals[i], vals[j] = vals[j], vals[i]
		}
		return vals
	}
	return memStream.GetBuffer(uuid)
}

func SubscribeStream(ctx context.Context, uuid string) <-chan string {
	ch := make(chan string, 100)
	if IsRedisUp() {
		pubsub := RedisClient.Subscribe(ctx, streamChannel(uuid))
		go func() {
			defer close(ch)
			defer pubsub.Close()
			_, _ = pubsub.Receive(ctx)
			msgCh := pubsub.Channel()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-msgCh:
					if !ok {
						return
					}
					select {
					case ch <- msg.Payload:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
		return ch
	}
	// Redis down → in-process subscribe
	return memStream.Subscribe(ctx, uuid, ch)
}
