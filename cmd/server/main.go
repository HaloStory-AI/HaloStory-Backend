package main

import (
	"context"
	"log"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/database"
	"rafiq8k-moga/halostory-go-backend/internal/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
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
			log.Printf("redis connection failed (auth will work without rate limiting): %v", err)
			rdb = nil
		} else {
			log.Println("redis connected")
		}
	}

	app := fiber.New(fiber.Config{
		AppName: cfg.AppName,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "*",
		AllowHeaders: "*",
	}))

	routes.Register(app, cfg, db, rdb)

	log.Printf("server starting on :18000")
	log.Fatal(app.Listen(":18000"))
}
