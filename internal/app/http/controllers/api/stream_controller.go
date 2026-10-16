package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

func sendDone(c *gin.Context, data string) {
	c.SSEvent("done", data)
	c.Writer.Flush()
}

// fetchCompletedResult membaca hasil completed dengan retry singkat.
// Diperlukan karena worker mem-publish "[DONE]" sebelum write hasil akhir
// (bond-parse + store) selesai — tanpa retry, klien dapat done {uuid} kosong.
func fetchCompletedResult(uuid string) (string, bool) {
	for i := 0; i < 3; i++ {
		if data, err := services.GetChatResultResilient(uuid); err == nil {
			var m map[string]interface{}
			if json.Unmarshal(data, &m) == nil && m["status"] == "completed" {
				return string(data), true
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", false
}

// flushHeaders mengirim header SSE + comment awal agar TTFB < 1 detik.
// Tanpa ini, proxy (nginx/Cloudflare) dan http client menunggu byte pertama
// yang baru tiba saat delta/completed — koneksi terlihat "pending".
func flushHeaders(c *gin.Context, uuid string) {
	c.Status(http.StatusOK)
	_, _ = fmt.Fprint(c.Writer, ": connected\n\n")
	c.Writer.Flush()
}

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
	flushHeaders(c, uuid)

	// If already completed, return immediately (Redis → DB fallback)
	if data, ok := fetchCompletedResult(uuid); ok {
		// Return done event
		sendDone(c, data)
		return
	}

	// Redis down → degradasi ke polling 1 detik (tanpa delta real-time).
	// Hasil akhir tetap terkirim sebagai event "done".
	// Keepalive 15 dtk agar nginx/Cloudflare tidak membunuh koneksi idle.
	if !services.IsRedisUp() {
		c.Header("X-Degraded", "polling")
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		keepalive := time.NewTicker(15 * time.Second)
		defer keepalive.Stop()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case <-keepalive.C:
				_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
				c.Writer.Flush()
			case <-ticker.C:
				if data, ok := fetchCompletedResult(uuid); ok {
					sendDone(c, data)
					return
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
	if data, ok := fetchCompletedResult(uuid); ok {
		sendDone(c, data)
		return
	}

	ch := services.SubscribeStream(c.Request.Context(), uuid)
	watchdog := time.NewTicker(5 * time.Second)
	defer watchdog.Stop()
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-keepalive.C:
			_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
			c.Writer.Flush()
		case <-watchdog.C:
			// Watchdog: Redis mati di tengah stream atau worker lambat →
			// cek hasil via DB fallback agar klien tidak menunggu selamanya.
			if data, ok := fetchCompletedResult(uuid); ok {
				sendDone(c, data)
				return
			}
		case delta, ok := <-ch:
			if !ok {
				// Channel ditutup (Redis mati mid-stream atau ctx selesai).
				// Coba baca hasil akhir dari DB fallback sebelum menyerah.
				if data, ok := fetchCompletedResult(uuid); ok {
					sendDone(c, data)
					return
				}
				// Belum selesai — lanjut polling via watchdog sampai ctx selesai.
				ch = nil // set nil agar select tidak spin pada closed channel
				continue
			}
			if delta == "[DONE]" {
				// fetch final result (dengan retry: DONE dipublish sebelum write)
				if data, ok := fetchCompletedResult(uuid); ok {
					sendDone(c, data)
				} else {
					fb, _ := json.Marshal(gin.H{"uuid": uuid})
					sendDone(c, string(fb))
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
