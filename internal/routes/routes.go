package routes

import (
	"rafiq8k-moga/halostory-go-backend/internal/config"
	byokHandler "rafiq8k-moga/halostory-go-backend/internal/handlers/byok"
	"rafiq8k-moga/halostory-go-backend/internal/handlers"
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

	// ── Static Auth ──
	staticGroup := api.Group("", middleware.StaticTokenAuth(cfg))
	staticGroup.Post("/login", auth.Login)
	staticGroup.Get("/v1/get-call", handlers.GetCall(cfg))

	// ── JWT Auth (allow banned users for device reporting) ──
	jwtGroup := api.Group("", middleware.JWTAuth(cfg))
	jwtGroup.Post("/user/report-banned-device", auth.ReportBannedDevice)

	// ── JWT Auth + Banned Check ──
	jwtBanned := api.Group("", middleware.JWTAuth(cfg), middleware.BannedCheck(db))
	jwtBanned.Get("/user", auth.GetUser)
	jwtBanned.Post("/email/verify/send", auth.SendVerificationOTP)
	jwtBanned.Post("/email/verify", auth.VerifyEmail)
	jwtBanned.Post("/user/update-email", auth.UpdateEmail)
	jwtBanned.Post("/user/update-name", auth.UpdateName)
	jwtBanned.Post("/user/update-password", auth.UpdatePassword)
	jwtBanned.Post("/user/update-persona", auth.UpdatePersona)

	// ── AI Chat ──
	jwtBanned.Post("/ai/chat/system", ai.SystemChat)
	jwtBanned.Post("/ai/summarize", ai.Summarize)
	jwtBanned.Get("/chat/status/:uuid", ai.ChatStatus)
	jwtBanned.Get("/ai/system-prompt/:bot_id", ai.GetSystemPrompt)

	// ── BYOK — 16 individual routes + models ──
	providers := []string{
		"gemini", "claude", "deepseek", "groq", "mistral",
		"openrouter", "arli", "mancer", "featherless", "chutes",
		"nanogpt", "venus", "zai", "novel", "horde", "openai-compatible",
	}
	for _, p := range providers {
		jwtBanned.Post("/byok/"+p+"/chat", byok.BYOKChatHandler())
	}
	jwtBanned.Post("/byok/models", byokHandler.FetchModels)

	// ── Bot routes ──
	bot := handlers.NewBotHandler(db)
	bond := handlers.NewBondHandler(db, cfg)
	jwtBanned.Get("/bots", bot.Index)
	jwtBanned.Post("/bots/recruit", bond.Recruit)
	jwtBanned.Get("/bots/bond/:bot_id", bond.Show)
	jwtBanned.Post("/bots/bond/:bot_id/choice", bond.HandleChoice)
	jwtBanned.Post("/bots/bond/:bot_id/debug", bond.DebugUpdate)

	// ── Story routes ──
	story := handlers.NewStoryHandler(db)
	jwtBanned.Get("/v1/stories", story.GetByBondedBots)

	// ── Calendar ──
	cal := handlers.NewCalendarHandler(db)
	jwtBanned.Post("/v1/calendar", cal.Index)

	// ── Categories ──
	cat := handlers.NewCategoryHandler(db)
	jwtBanned.Get("/v1/categories", cat.Index)
	jwtBanned.Get("/v1/category/:slug", cat.Bots)

	// ── Bot Tab ──
	botTab := handlers.NewBotTabHandler(db)
	jwtBanned.Get("/v1/bot-tab", botTab.Index)

	// ── Bot Bio ──
	botBio := handlers.NewBotBioHandler(db)
	jwtBanned.Get("/v1/bot-bio/:bot_id", botBio.Show)

	// ── Recruited Bots ──
	recruited := handlers.NewRecruitedBotHandler(db)
	jwtBanned.Get("/v1/recruited-bots", recruited.Index)

	// ── Student Requests ──
	sr := handlers.NewStudentRequestHandler(db, cfg)
	jwtBanned.Get("/student-requests", sr.Index)
	jwtBanned.Post("/student-requests", sr.Store)

	// ── Backup ──
	backup := handlers.NewBackupHandler(db)
	jwtBanned.Post("/user/backup", backup.Store)
	jwtBanned.Get("/user/restore", backup.Restore)

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
