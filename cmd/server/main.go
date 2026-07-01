package main

import (
	"log"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/routes"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Load environment variables dari .env
	config.LoadConfig()

	// Koneksi ke Database
	database.Connect()

	// Inisialisasi instance Fiber baru
	app := fiber.New()

	// Daftarkan routing (mirip seperti register di Laravel)
	routes.SetupAPI(app)

	// Route dasar untuk memastikan server berjalan
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Halo dari Fiber v2! Routing terpisah berhasil.")
	})

	// Jalankan server di port 3000
	log.Fatal(app.Listen(":3000"))
}
