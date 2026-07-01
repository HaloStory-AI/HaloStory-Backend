package main

import (
	"context"
	"log"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/database"
	"rafiq8k-moga/halostory-go-backend/internal/jobs"
	byokJob "rafiq8k-moga/halostory-go-backend/internal/jobs"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	db := database.NewDB(cfg)

	var rdb *redis.Client
	if cfg.RedisAddr != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		_, err := rdb.Ping(context.Background()).Result()
		if err != nil {
			log.Printf("redis connection failed: %v", err)
			return
		}
		log.Println("redis connected")
	}

	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 30,
		Queues: map[string]int{
			"ai-chat": 10,
			"default": 5,
		},
	})

	mux := asynq.NewServeMux()

	aiProcessor := jobs.NewSystemAIChatProcessor(db, cfg, rdb)
	mux.HandleFunc(jobs.TypeSystemAIChat, aiProcessor.ProcessTask)

	byokProcessor := byokJob.NewBYOKChatProcessor(db, cfg, rdb)
	mux.HandleFunc(byokJob.TypeBYOKChat, byokProcessor.ProcessTask)

	cacheProcessor := jobs.NewCacheInvalidationProcessor(rdb)
	mux.HandleFunc(jobs.TypeCacheInvalidation, cacheProcessor.ProcessTask)

	log.Printf("worker starting, connected to redis at %s", cfg.RedisAddr)
	log.Fatal(server.Run(mux))
}
