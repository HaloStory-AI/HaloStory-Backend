package routes

import (
	"halostory_go_backend/internal/app/http/controllers/auth"
	apiControllers "halostory_go_backend/internal/app/http/controllers/api"
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

	// Public API routes
	api.Get("/image", apiControllers.ImageProxy)

	// Authentication routes (Public)
	api.Post("/login", auth.Login)
	api.Post("/register", auth.Register)

	// Protected routes (membutuhkan JWT dan tidak dibanned)
	protected := api.Group("/", middlewares.Protected(), middlewares.CheckBanned())
	
	protected.Get("/user", func(c *fiber.Ctx) error {
		// Ambil data user dari context Locals yang diset oleh middleware
		user := c.Locals("user").(models.User)
		return c.JSON(user)
	})

	// Email Verification Routes
	protected.Post("/email/send-verification-otp", auth.SendVerificationOtp)
	protected.Post("/email/verify", auth.VerifyEmail)

	// Categories Routes
	protected.Get("/v1/categories", apiControllers.CategoryIndex)
	protected.Get("/v1/category/:slug", apiControllers.CategoryBots)

	// Bot & Bond Routes
	protected.Get("/bots", apiControllers.BotIndex)
	protected.Get("/v1/bot-bio/:bot_id", apiControllers.ShowBio)
	protected.Get("/v1/bot-tab", apiControllers.BotTabIndex)
	protected.Get("/v1/recruited-bots", apiControllers.RecruitedBotsIndex)

	protected.Post("/bots/recruit", apiControllers.BondRecruit)
	protected.Get("/bots/bond/:bot_id", apiControllers.BondShow)
	protected.Post("/bots/bond/:bot_id/choice", apiControllers.BondHandleChoice)
	protected.Post("/bots/bond/:bot_id/debug", apiControllers.BondDebugUpdate)
}
