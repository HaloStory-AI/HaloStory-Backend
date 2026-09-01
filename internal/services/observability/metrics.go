package observability

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// Simple in-memory metrics without external prometheus dependency
// Exposes Prometheus-compatible text at GET /metrics (guarded by APP_DEBUG or token)

type counterKey struct {
	Provider string
	Model    string
	Status   string
}

var (
	counters   sync.Map // map[counterKey]*atomic.Int64
	histograms sync.Map // map[string][]float64 (naive, protected by mutex)
	histoMu    sync.Mutex
	queueDepth atomic.Int64
)

func IncRequest(provider, model, status string) {
	k := counterKey{Provider: provider, Model: model, Status: status}
	v, _ := counters.LoadOrStore(k, &atomic.Int64{})
	v.(*atomic.Int64).Add(1)
}

func ObserveLatency(provider string, d time.Duration) {
	ms := float64(d.Milliseconds())
	histoMu.Lock()
	defer histoMu.Unlock()
	key := "ai_latency_ms_" + provider
	val, _ := histograms.Load(key)
	var arr []float64
	if val != nil {
		arr = val.([]float64)
	}
	arr = append(arr, ms)
	// keep last 1000
	if len(arr) > 1000 {
		arr = arr[len(arr)-1000:]
	}
	histograms.Store(key, arr)
}

func SetQueueDepth(n int64) {
	queueDepth.Store(n)
}

func MetricsHandler(c *gin.Context) {
	c.Header("Content-Type", "text/plain; version=0.0.4")
	// counters
	var lines []string
	counters.Range(func(k, v any) bool {
		ck := k.(counterKey)
		cnt := v.(*atomic.Int64).Load()
		lines = append(lines, fmt.Sprintf(`ai_requests_total{provider="%s",model="%s",status="%s"} %d`, ck.Provider, ck.Model, ck.Status, cnt))
		return true
	})
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Fprintln(c.Writer, l)
	}
	fmt.Fprintf(c.Writer, "queue_depth %d\n", queueDepth.Load())

	histoMu.Lock()
	defer histoMu.Unlock()
	histograms.Range(func(k, v any) bool {
		arr := v.([]float64)
		if len(arr) == 0 {
			return true
		}
		var sum float64
		for _, x := range arr {
			sum += x
		}
		avg := sum / float64(len(arr))
		sort.Float64s(arr)
		p50 := arr[len(arr)/2]
		p95 := arr[int(float64(len(arr))*0.95)]
		if p95 >= float64(len(arr)) {
			p95 = arr[len(arr)-1]
		}
		fmt.Fprintf(c.Writer, "ai_latency_ms_avg{provider=\"%s\"} %.2f\n", k, avg)
		fmt.Fprintf(c.Writer, "ai_latency_ms_p50{provider=\"%s\"} %.2f\n", k, p50)
		fmt.Fprintf(c.Writer, "ai_latency_ms_p95{provider=\"%s\"} %.2f\n", k, p95)
		return true
	})
}

// Gin middleware for request ID + latency logging
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		traceID := c.GetHeader("X-Request-Id")
		if traceID == "" {
			traceID = c.GetHeader("X-Trace-Id")
		}
		if traceID == "" {
			// generate lightweight id if missing (fallback for non-AI routes)
			traceID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		c.Set("trace_id", traceID)
		c.Header("X-Request-Id", traceID)
		c.Header("X-Trace-Id", traceID)

		c.Next()

		latency := time.Since(start)
		// sample 10% or always if debug
		if ShouldSample() || c.Writer.Status() >= 400 {
			Info("http.request",
				"trace_id", traceID,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"status", c.Writer.Status(),
				"latency_ms", latency.Milliseconds(),
				"client_ip", c.ClientIP(),
			)
		}
		// queue depth placeholder (could query redis LLen async)
		_ = latency
	}
}

// Helper for handlers to get trace id
func GetTraceID(c *gin.Context) string {
	if v, ok := c.Get("trace_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	h := c.GetHeader("X-Request-Id")
	if h != "" {
		return h
	}
	return ""
}

// Ensure import for gin used in MetricsHandler signature
var _ = http.StatusOK
