package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscord_SendRequestApproved(t *testing.T) {
	var receivedPayload discordPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("DISCORD_WEBHOOK_URL", server.URL)
	defer os.Unsetenv("DISCORD_WEBHOOK_URL")

	discordHTTPClient = &http.Client{}

	SendRequestApproved("TestUser", "TestCharacter")

	assert.Equal(t, "Character Request Approved @everyone", receivedPayload.Content)
	require.Len(t, receivedPayload.Embeds, 1)
	assert.Equal(t, 3066993, receivedPayload.Embeds[0].Color)
}

func TestDiscord_SendRequestRejected(t *testing.T) {
	var receivedPayload discordPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv("DISCORD_WEBHOOK_URL", server.URL)
	defer os.Unsetenv("DISCORD_WEBHOOK_URL")

	discordHTTPClient = &http.Client{}

	SendRequestRejected("TestUser", "TestCharacter")

	assert.Equal(t, "Character Request Rejected", receivedPayload.Content)
	require.Len(t, receivedPayload.Embeds, 1)
	assert.Equal(t, 15277667, receivedPayload.Embeds[0].Color)
}

func TestDiscord_EmptyURL(t *testing.T) {
	os.Unsetenv("DISCORD_WEBHOOK_URL")

	discordHTTPClient = &http.Client{}

	// Should not panic or error
	SendRequestApproved("TestUser", "TestCharacter")
	SendRequestRejected("TestUser", "TestCharacter")
}
