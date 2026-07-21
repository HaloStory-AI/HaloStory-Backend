package services

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoRedis(t *testing.T) {
	if os.Getenv("TEST_REDIS") != "true" {
		t.Skip("Skipping Redis test: set TEST_REDIS=true to run")
	}
}

func TestQueue_EnqueueDequeue(t *testing.T) {
	skipIfNoRedis(t)

	payload := []byte(`{"test":"data"}`)
	err := Enqueue("test-queue", payload)
	require.NoError(t, err)

	queue, data, err := Dequeue("test-queue", 0)
	require.NoError(t, err)
	assert.Equal(t, "queue:test-queue", queue)
	assert.Equal(t, payload, data)
}

func TestQueue_ChatResult(t *testing.T) {
	skipIfNoRedis(t)

	payload := []byte(`{"status":"completed","response":"ok"}`)
	err := StoreChatResult("test-uuid", payload, 0)
	require.NoError(t, err)

	data, err := GetChatResult("test-uuid")
	require.NoError(t, err)
	assert.Equal(t, payload, data)

	err = DeleteChatResult("test-uuid")
	require.NoError(t, err)

	_, err = GetChatResult("test-uuid")
	assert.Error(t, err)
}

func TestQueue_ChatPayload(t *testing.T) {
	skipIfNoRedis(t)

	payload := ChatPayload{
		Status:     ChatStatusQueued,
		UserID:     1,
		BotID:      "test-bot",
		IsGroup:    false,
		IsPriority: false,
		StatusCode: 200,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	err = StoreChatResult("test-payload", data, 0)
	require.NoError(t, err)

	raw, err := GetChatResult("test-payload")
	require.NoError(t, err)

	var decoded ChatPayload
	err = json.Unmarshal(raw, &decoded)
	require.NoError(t, err)
	assert.Equal(t, ChatStatusQueued, decoded.Status)
	assert.Equal(t, uint(1), decoded.UserID)
	assert.Equal(t, "test-bot", decoded.BotID)

	err = DeleteChatResult("test-payload")
	require.NoError(t, err)
}

func TestQueue_UpdateChatStatus(t *testing.T) {
	skipIfNoRedis(t)

	payload := ChatPayload{Status: ChatStatusQueued, UserID: 1}
	data, _ := json.Marshal(payload)
	StoreChatResult("test-status", data, 0)

	err := UpdateChatStatus("test-status", ChatStatusProcessing)
	require.NoError(t, err)

	raw, _ := GetChatResult("test-status")
	var decoded ChatPayload
	json.Unmarshal(raw, &decoded)
	assert.Equal(t, ChatStatusProcessing, decoded.Status)

	err = DeleteChatResult("test-status")
	require.NoError(t, err)
}

func TestQueue_Cooldown(t *testing.T) {
	skipIfNoRedis(t)

	active, err := CheckCooldown(999)
	require.NoError(t, err)
	assert.False(t, active)

	err = SetCooldown(999, 0)
	require.NoError(t, err)

	active, err = CheckCooldown(999)
	require.NoError(t, err)
	assert.True(t, active)
}

func TestQueue_Pending(t *testing.T) {
	skipIfNoRedis(t)

	err := PushPending(999, "uuid-1")
	require.NoError(t, err)

	err = PushPending(999, "uuid-2")
	require.NoError(t, err)

	uuid, err := PopPending(999)
	require.NoError(t, err)
	assert.Equal(t, "uuid-1", uuid)

	uuid, err = PopPending(999)
	require.NoError(t, err)
	assert.Equal(t, "uuid-2", uuid)
}
