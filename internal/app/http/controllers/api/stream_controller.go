package api

import (
	"encoding/json"
	"io"
	"net/http"

	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

func ChatStream(c *gin.Context) {
	uuid := c.Param("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid required"})
		return
	}
	// headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.Header().Set("X-Trace-Id", uuid)

	// If already completed, return immediately
	if data, err := services.GetChatResult(uuid); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil {
			if m["status"] == "completed" {
				// Return done event
				c.SSEvent("done", string(data))
				return
			}
		}
	}

	// Replay buffer first
	for _, delta := range services.GetStreamBuffer(uuid) {
		c.SSEvent("delta", gin.H{"content": delta})
		c.Writer.Flush()
	}

	// Check if already completed after replay
	if data, err := services.GetChatResult(uuid); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil && m["status"] == "completed" {
			c.SSEvent("done", string(data))
			return
		}
	}

	ch := services.SubscribeStream(c.Request.Context(), uuid)
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case delta, ok := <-ch:
			if !ok {
				return
			}
			if delta == "[DONE]" {
				// fetch final result
				if data, err := services.GetChatResult(uuid); err == nil {
					c.SSEvent("done", string(data))
				} else {
					c.SSEvent("done", gin.H{"uuid": uuid})
				}
				return
			}
			c.SSEvent("delta", gin.H{"content": delta})
			c.Writer.Flush()
		}
	}
}

func ChatStreamHealth(c *gin.Context) {
	// simple check that upstream not buffering
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.SSEvent("ping", gin.H{"status": "ok"})
}

// Proxy helper for Cloudflare compatibility
var _ = io.Copy
