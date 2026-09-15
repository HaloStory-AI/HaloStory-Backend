package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

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

	// If already completed, return immediately (Redis → DB fallback)
	if data, err := services.GetChatResultResilient(uuid); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil {
			if m["status"] == "completed" {
				// Return done event
				c.SSEvent("done", string(data))
				return
			}
		}
	}

	// Redis down → degradasi ke polling 1 detik (tanpa delta real-time).
	// Hasil akhir tetap terkirim sebagai event "done".
	if !services.IsRedisUp() {
		c.Header("X-Degraded", "polling")
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-ticker.C:
				if data, err := services.GetChatResultResilient(uuid); err == nil {
					var m map[string]interface{}
					if json.Unmarshal(data, &m) == nil && m["status"] == "completed" {
						c.SSEvent("done", string(data))
						return
					}
				}
			}
		}
	}

	// Replay buffer first
	for _, delta := range services.GetStreamBuffer(uuid) {
		c.SSEvent("delta", gin.H{"content": delta})
		c.Writer.Flush()
	}

	// Check if already completed after replay
	if data, err := services.GetChatResultResilient(uuid); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil && m["status"] == "completed" {
			c.SSEvent("done", string(data))
			return
		}
	}

	ch := services.SubscribeStream(c.Request.Context(), uuid)
	watchdog := time.NewTicker(5 * time.Second)
	defer watchdog.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-watchdog.C:
			// Redis mati di tengah stream → cek hasil via fallback
			if data, err := services.GetChatResultResilient(uuid); err == nil {
				var m map[string]interface{}
				if json.Unmarshal(data, &m) == nil && m["status"] == "completed" {
					c.SSEvent("done", string(data))
					return
				}
			}
		case delta, ok := <-ch:
			if !ok {
				return
			}
			if delta == "[DONE]" {
				// fetch final result
				if data, err := services.GetChatResultResilient(uuid); err == nil {
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
