package services

import (
	"context"
	"log"

	"halostory_go_backend/internal/config"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client
var RedisCtx = context.Background()

func InitRedis() {
	host := config.Env("REDIS_HOST", "127.0.0.1")
	port := config.Env("REDIS_PORT", "6379")
	password := config.Env("REDIS_PASSWORD", "")

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: password,
		DB:       0,
	})

	_, err := RedisClient.Ping(RedisCtx).Result()
	if err != nil {
		log.Println("Warning: Gagal terhubung ke Redis:", err)
		return
	}

	log.Println("Berhasil terhubung ke Redis!")
}
