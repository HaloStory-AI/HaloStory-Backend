package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

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

	// Setup Google OAUTH2
	services.InitGoogleAuthorization()

	// Set Gin mode (default release — set GIN_MODE=debug di .env untuk dev)
	gin.SetMode(config.Env("GIN_MODE", "release"))

	// Inisialisasi instance Gin baru dengan middleware global
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middlewares.SetupCORS())

	// Daftarkan routing (mirip seperti register di Laravel)
	routes.SetupAPI(r)

	// Route dasar untuk memastikan server berjalan
	r.GET("/", func(c *gin.Context) {
		c.File("internal/templates/index.html")
	})

	// Handle 404 Not Found
	r.NoRoute(func(c *gin.Context) {
		c.File("internal/templates/404.html")
	})

	r.GET("/up", func(c *gin.Context) {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get("https://icy-recipe-89f8.r8kstudio.workers.dev")
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			var data struct {
				Status             string `json:"status"`
				MaintenanceMessage string `json:"maintenance_message"`
			}
			if json.Unmarshal(body, &data) == nil && data.Status == "active" {
				c.String(200, "OK")
				return
			}
			if data.MaintenanceMessage != "" {
				c.String(503, data.MaintenanceMessage)
				return
			}
			c.String(503, "Maintenance")
			return
		}
		c.String(200, "OK")
	})

	// Jalankan server di port 3000
	log.Fatal(r.Run(":3000"))
}
