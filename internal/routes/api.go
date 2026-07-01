package routes

import (
	"halostory_go_backend/internal/app/http/controllers/auth"
	"halostory_go_backend/internal/app/http/middlewares"
	"halostory_go_backend/internal/app/models"

	"github.com/gofiber/fiber/v2"
)

// SetupAPI mendaftarkan semua route untuk aplikasi
func SetupAPI(app *fiber.App) {
	// Membuat grup route untuk /api (mirip routes/api.php di Laravel)
	api := app.Group("/api")

	// Contoh endpoint API sederhana
	api.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "pong",
			"status":  "ok",
		})
	})

	// Authentication routes (Public)
	api.Post("/login", auth.Login)
	api.Post("/register", auth.Register)

	// Protected routes (membutuhkan JWT)
	protected := api.Group("/", middlewares.Protected())
	
	protected.Get("/user", func(c *fiber.Ctx) error {
		// Ambil data user dari context Locals yang diset oleh middleware
		user := c.Locals("user").(models.User)
		return c.JSON(user)
	})

	// Email Verification Routes
	protected.Post("/email/send-verification-otp", auth.SendVerificationOtp)
	protected.Post("/email/verify", auth.VerifyEmail)
}
