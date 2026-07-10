package routes

import (
	"halostory_go_backend/internal/app/http/controllers/auth"
	apiControllers "halostory_go_backend/internal/app/http/controllers/api"
	"halostory_go_backend/internal/app/http/middlewares"
	"halostory_go_backend/internal/app/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SetupAPI mendaftarkan semua route untuk aplikasi
func SetupAPI(r *gin.Engine) {
	// Membuat grup route untuk /api
	api := r.Group("/api")

	// Contoh endpoint API sederhana
	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
			"status":  "ok",
		})
	})

	// Public API routes
	api.GET("/image", apiControllers.ImageProxy)

	// Authentication routes (Public)
	api.POST("/login", auth.Login)
	api.POST("/register", auth.Register)

	// Protected routes (membutuhkan JWT dan tidak dibanned)
	protected := api.Group("/", middlewares.Protected(), middlewares.CheckBanned())

	protected.GET("/user", func(c *gin.Context) {
		// Ambil data user dari context yang diset oleh middleware
		userVal, _ := c.Get("user")
		user := userVal.(models.User)
		c.JSON(http.StatusOK, user)
	})

	// Email Verification Routes
	protected.POST("/email/send-verification-otp", auth.SendVerificationOtp)
	protected.POST("/email/verify", auth.VerifyEmail)

	// Categories Routes
	protected.GET("/v1/categories", apiControllers.CategoryIndex)
	protected.GET("/v1/category/:slug", apiControllers.CategoryBots)

	// Bot & Bond Routes
	protected.GET("/bots", apiControllers.BotIndex)
	protected.GET("/v1/bot-bio/:bot_id", apiControllers.ShowBio)
	protected.GET("/v1/bot-tab", apiControllers.BotTabIndex)
	protected.GET("/v1/recruited-bots", apiControllers.RecruitedBotsIndex)
	protected.GET("/v1/stories", apiControllers.GetByBondedBots)

	protected.POST("/bots/recruit", apiControllers.BondRecruit)
	protected.GET("/bots/bond/:bot_id", apiControllers.BondShow)
	protected.POST("/bots/bond/:bot_id/choice", apiControllers.BondHandleChoice)
	protected.POST("/bots/bond/:bot_id/debug", apiControllers.BondDebugUpdate)
}
