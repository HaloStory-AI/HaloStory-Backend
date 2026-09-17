package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/observability"
	"halostory_go_backend/internal/utils"
	"halostory_go_backend/internal/worker"
)

var httpClient = utils.NewAIHttpClient()

func workerID() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "worker"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

func dispatchJob(data []byte, sem chan struct{}, goroutineName string) {
	sem <- struct{}{}
	go func(d []byte) {
		var jobUUID string
		var jobTraceID string
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Worker: RECOVERED panic in %s goroutine: %v", goroutineName, r)
				observability.Error("worker.panic", "goroutine", goroutineName, "error", fmt.Sprintf("%v", r), "uuid", jobUUID, "trace_id", jobTraceID)
				if jobUUID != "" {
					_ = services.MarkChatCompletedResilient(jobUUID, json.RawMessage(fmt.Sprintf(`{"error":"Internal worker panic: %v","error_status_code":500}`, r)), 500)
				}
			}
			<-sem
		}()
		var rawPayload map[string]interface{}
		if json.Unmarshal(d, &rawPayload) == nil {
			if u, ok := rawPayload["uuid"].(string); ok {
				jobUUID = u
			}
			if tid, ok := rawPayload["trace_id"].(string); ok {
				jobTraceID = tid
			}
		}
		worker.ProcessJob(d)
	}(data)
}

func main() {
	config.LoadConfig()
	observability.InitLogger()
	database.Connect()
	services.InitRedis()

	maxWorkersBYOK := 100
	maxWorkersSystem := 30
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxWorkersBYOK = n
		}
	}
	mainSem := make(chan struct{}, maxWorkersBYOK)
	sysSem := make(chan struct{}, maxWorkersSystem)
	log.Printf("Worker service starting — BYOK concurrency: %d, System AI concurrency: %d (free), VIP routed to BYOK pool", maxWorkersBYOK, maxWorkersSystem)
	_ = httpClient

	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		})
		log.Printf("Worker health check listening on :3001")
		if err := http.ListenAndServe(":3001", nil); err != nil {
			log.Printf("Worker health check server error: %v", err)
		}
	}()

	go func() {
		wid := workerID()
		backoff := 100 * time.Millisecond
		lastDbSweep := time.Now().Add(-1 * time.Hour)
		for {
			_, data, err := services.Dequeue("ai-chat", 5*time.Second)
			if err != nil {
				shouldSweep := services.IsRedisDown(err) || time.Since(lastDbSweep) > 30*time.Second
				if shouldSweep {
					lastDbSweep = time.Now()
					if payload, cerr := services.ClaimDbJob(wid); cerr != nil {
						log.Printf("Worker: ClaimDbJob error: %v", cerr)
					} else if payload != nil {
						backoff = 100 * time.Millisecond
						dispatchJob(payload, mainSem, "main-db")
						continue
					}
				}
				time.Sleep(backoff)
				backoff *= 2
				if backoff > 10*time.Second {
					backoff = 10 * time.Second
				}
				continue
			}
			backoff = 100 * time.Millisecond

			// Route System AI free to sysSem, VIP/priority to mainSem
			var probe struct {
				Provider   string `json:"provider"`
				IsPriority bool   `json:"is_priority"`
			}
			_ = json.Unmarshal(data, &probe)
			if probe.Provider == "system-ai" {
				if probe.IsPriority {
					dispatchJob(data, mainSem, "main")
				} else {
					dispatchJob(data, sysSem, "sys")
				}
			} else {
				dispatchJob(data, mainSem, "main")
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Worker service shutting down...")
}
