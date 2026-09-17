package main

import (
	"log"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
)

func main() {
	config.LoadConfig()
	database.Connect()
	log.Println("Backfill activities: 365d rollup start")
	if err := services.BackfillRollups(); err != nil {
		log.Fatalf("Backfill failed: %v", err)
	}
	log.Println("Backfill done")
}
