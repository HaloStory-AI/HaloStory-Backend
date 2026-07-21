package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
)

func main() {
	config.LoadConfig()
	database.Connect()

	log.Println("Scheduler service starting...")

	scheduler := services.StartScheduler()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Scheduler service shutting down...")
	scheduler.Stop()
	log.Println("Scheduler service stopped.")
}
