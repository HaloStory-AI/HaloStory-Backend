package main

import (
	"log"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/cron"
	"rafiq8k-moga/halostory-go-backend/internal/database"
)

func main() {
	cfg := config.Load()
	db := database.NewDB(cfg)

	log.Println("scheduler starting...")
	cron.Start(cfg, db, nil)
}
