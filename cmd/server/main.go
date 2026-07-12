package main

import (
	"log"

	"halostory_go_backend/internal/app/http/middlewares"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/routes"
	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load environment variables dari .env
	config.LoadConfig()

	// Koneksi ke Database
	database.Connect()

	// Koneksi ke S3
	services.InitS3()

	// Koneksi ke Redis
	services.InitRedis()

	// Inisialisasi instance Gin baru dengan middleware global
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middlewares.SetupCORS())

	// Daftarkan routing (mirip seperti register di Laravel)
	routes.SetupAPI(r)

	// Route dasar untuk memastikan server berjalan
	r.GET("/", func(c *gin.Context) {
		c.String(200, "Halo dari Gin! Routing terpisah berhasil.")
	})

	// Jalankan server di port 3000
	log.Fatal(r.Run(":3000"))
}
