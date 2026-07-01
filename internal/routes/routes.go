package routes

import (
	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/handlers"
	byokHandler "rafiq8k-moga/halostory-go-backend/internal/handlers/byok"
	"rafiq8k-moga/halostory-go-backend/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func Register(app *fiber.App, cfg *config.Config, db *gorm.DB, rdb *redis.Client) {
	api := app.Group("/api")

	// ── Public ──
	api.Get("/app/status", handlers.GetAppStatus(db))

	auth := handlers.NewAuthHandler(db, cfg, rdb)
	ai := handlers.NewAIHandler(db, cfg, rdb)
	byok := byokHandler.NewBYOKHandler(db, cfg, rdb)

	api.Post("/register", auth.Register)
	api.Post("/forgot-password", auth.ForgotPassword)
	api.Post("/reset-password", auth.ResetPassword)

	// ── Static Auth (applied per-route, NOT via group — empty-prefix groups
	//     leak middleware to sibling routes in Fiber v2) ──
	api.Post("/login", middleware.StaticTokenAuth(cfg), auth.Login)
	api.Get("/v1/get-call", middleware.StaticTokenAuth(cfg), handlers.GetCall(cfg))

	// ── JWT Auth (allow banned users for device reporting) ──
	api.Post("/user/report-banned-device", middleware.JWTAuth(cfg), auth.ReportBannedDevice)

	// ── JWT Auth + Banned Check ──
	// ── JWT Auth + Banned Check ──
	api.Get("/user", middleware.JWTAuth(cfg), middleware.BannedCheck(db), auth.GetUser)
	api.Post("/email/verify/send", middleware.JWTAuth(cfg), middleware.BannedCheck(db), auth.SendVerificationOTP)
	api.Post("/email/verify", middleware.JWTAuth(cfg), middleware.BannedCheck(db), auth.VerifyEmail)
	api.Post("/user/update-email", middleware.JWTAuth(cfg), middleware.BannedCheck(db), auth.UpdateEmail)
	api.Post("/user/update-name", middleware.JWTAuth(cfg), middleware.BannedCheck(db), auth.UpdateName)
	api.Post("/user/update-password", middleware.JWTAuth(cfg), middleware.BannedCheck(db), auth.UpdatePassword)
	api.Post("/user/update-persona", middleware.JWTAuth(cfg), middleware.BannedCheck(db), auth.UpdatePersona)

	// ── AI Chat ──
	api.Post("/ai/chat/system", middleware.JWTAuth(cfg), middleware.BannedCheck(db), ai.SystemChat)
	api.Post("/ai/summarize", middleware.JWTAuth(cfg), middleware.BannedCheck(db), ai.Summarize)
	api.Get("/chat/status/:uuid", middleware.JWTAuth(cfg), middleware.BannedCheck(db), ai.ChatStatus)
	api.Get("/ai/system-prompt/:bot_id", middleware.JWTAuth(cfg), middleware.BannedCheck(db), ai.GetSystemPrompt)

	// ── BYOK — 16 individual routes + models ──
	providers := []string{
		"gemini", "claude", "deepseek", "groq", "mistral",
		"openrouter", "arli", "mancer", "featherless", "chutes",
		"nanogpt", "venus", "zai", "novel", "horde", "openai-compatible",
	}
	for _, p := range providers {
		api.Post("/byok/"+p+"/chat", middleware.JWTAuth(cfg), middleware.BannedCheck(db), byok.BYOKChatHandler())
	}
	api.Post("/byok/models", middleware.JWTAuth(cfg), middleware.BannedCheck(db), byokHandler.FetchModels)

	// ── Bot routes ──
	bot := handlers.NewBotHandler(db)
	bond := handlers.NewBondHandler(db, cfg)
	api.Get("/bots", middleware.JWTAuth(cfg), middleware.BannedCheck(db), bot.Index)
	api.Post("/bots/recruit", middleware.JWTAuth(cfg), middleware.BannedCheck(db), bond.Recruit)
	api.Get("/bots/bond/:bot_id", middleware.JWTAuth(cfg), middleware.BannedCheck(db), bond.Show)
	api.Post("/bots/bond/:bot_id/choice", middleware.JWTAuth(cfg), middleware.BannedCheck(db), bond.HandleChoice)
	api.Post("/bots/bond/:bot_id/debug", middleware.JWTAuth(cfg), middleware.BannedCheck(db), bond.DebugUpdate)

	// ── Story routes ──
	story := handlers.NewStoryHandler(db)
	api.Get("/v1/stories", middleware.JWTAuth(cfg), middleware.BannedCheck(db), story.GetByBondedBots)

	// ── Calendar ──
	cal := handlers.NewCalendarHandler(db)
	api.Post("/v1/calendar", middleware.JWTAuth(cfg), middleware.BannedCheck(db), cal.Index)

	// ── Categories ──
	cat := handlers.NewCategoryHandler(db)
	api.Get("/v1/categories", middleware.JWTAuth(cfg), middleware.BannedCheck(db), cat.Index)
	api.Get("/v1/category/:slug", middleware.JWTAuth(cfg), middleware.BannedCheck(db), cat.Bots)

	// ── Bot Tab ──
	botTab := handlers.NewBotTabHandler(db)
	api.Get("/v1/bot-tab", middleware.JWTAuth(cfg), middleware.BannedCheck(db), botTab.Index)

	// ── Bot Bio ──
	botBio := handlers.NewBotBioHandler(db)
	api.Get("/v1/bot-bio/:bot_id", middleware.JWTAuth(cfg), middleware.BannedCheck(db), botBio.Show)

	// ── Recruited Bots ──
	recruited := handlers.NewRecruitedBotHandler(db)
	api.Get("/v1/recruited-bots", middleware.JWTAuth(cfg), middleware.BannedCheck(db), recruited.Index)

	// ── Student Requests ──
	sr := handlers.NewStudentRequestHandler(db, cfg)
	api.Get("/student-requests", middleware.JWTAuth(cfg), middleware.BannedCheck(db), sr.Index)
	api.Post("/student-requests", middleware.JWTAuth(cfg), middleware.BannedCheck(db), sr.Store)

	// ── Backup ──
	backup := handlers.NewBackupHandler(db)
	api.Post("/user/backup", middleware.JWTAuth(cfg), middleware.BannedCheck(db), backup.Store)
	api.Get("/user/restore", middleware.JWTAuth(cfg), middleware.BannedCheck(db), backup.Restore)

	// ── Admin (JWT + role=admin) ──
	admin := handlers.NewAdminHandler(db, cfg)
	adminGroup := api.Group("/admin", middleware.JWTAuth(cfg), middleware.AdminOnly())
	adminGroup.Get("/users", admin.GetUsers)
	adminGroup.Put("/users/:id/role", admin.ChangeUserRole)
	adminGroup.Delete("/users/:id", admin.DeleteUser)
	adminGroup.Get("/student-requests", admin.GetStudentRequests)
	adminGroup.Put("/student-requests/:id/status", admin.UpdateStudentRequestStatus)
	adminGroup.Get("/bots", admin.GetBots)
	adminGroup.Post("/bots", admin.StoreBot)
	adminGroup.Post("/bots/import", admin.ImportBots)
	adminGroup.Put("/bots/:id", admin.UpdateBot)
	adminGroup.Delete("/bots/:id", admin.DeleteBot)
}
