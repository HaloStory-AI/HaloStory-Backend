package routes

import (
	"net/http"

	adminControllers "halostory_go_backend/internal/app/http/controllers/admin"
	apiControllers "halostory_go_backend/internal/app/http/controllers/api"
	byokControllers "halostory_go_backend/internal/app/http/controllers/api/byok"
	"halostory_go_backend/internal/app/http/controllers/auth"
	"halostory_go_backend/internal/app/http/middlewares"
	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/services/observability"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// SetupAPI mendaftarkan semua route untuk aplikasi
func SetupAPI(r *gin.Engine) {
	// Observability: metrics endpoint (public, Prometheus-compatible)
	r.GET("/metrics", observability.MetricsHandler)
	apiMetrics := r.Group("/api")
	apiMetrics.GET("/metrics", observability.MetricsHandler)

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

	// App Status (Public)
	api.GET("/app/status", apiControllers.AppStatus)

	// Authentication routes (Static token required — like bluetalk's static.auth)
	api.POST("/login", middlewares.CheckStaticToken(), auth.Login)
	api.GET("/v1/get-call", middlewares.CheckStaticToken(), apiControllers.GetCall)
	api.POST("/register", auth.Register)

	// Token refresh (public) — menukar refresh token dengan access token baru
	api.POST("/refresh", auth.Refresh)

	// Google OAUTH2 ( Dari Flutter kirim Auth code + device_id )
	api.POST("/auth/google/exchange", middlewares.CheckStaticToken(), auth.AuthorizationCodeExchange)



	// ################################################################
	// Buat tes development aja
	// Gak dipake
	// Gak punya sdk flutter nya rek, android studio berat cuy
	api.GET("/auth/google/callback", func(c *gin.Context) {
		code := c.Query("code")

		c.JSON(200, gin.H{
			"authorization_code": code,
			"device_id": "dummy_id",
		})
	})
	api.GET("/auth/google", func(c *gin.Context) {
		url := services.GOOGLE_OAUTH.AuthCodeURL(
			"random-state",
			oauth2.AccessTypeOffline,
			oauth2.ApprovalForce,
		)

		c.Redirect(302, url)
	})
	// ################################################################




	// Password Reset Routes (Public)
	api.POST("/forgot-password", auth.ForgotPassword)
	api.POST("/reset-password", auth.ResetPassword)

	// Protected routes (membutuhkan JWT)
	protected := api.Group("/", middlewares.Protected())

	protected.GET("/user", func(c *gin.Context) {
		// Ambil data user dari context yang diset oleh middleware
		userVal, _ := c.Get("user")
		user := userVal.(models.User)
		c.JSON(http.StatusOK, user)
	})

	// Sama kayak yang diatas, tapi ini untuk link akun dan wajib kirim JWT yang valid
	// Google OAUTH2 LINK ( Dari Flutter kirim Auth code + device_id ) wajib kirim headers authorization
	protected.POST("/auth/google/link", auth.AuthorizationCodeLink)

	// Logout (revoke refresh token)
	protected.POST("/logout", auth.Logout)

	// Email Verification Routes (excluded from email verification check)
	protected.POST("/email/send-verification-otp", auth.SendVerificationOtp)
	protected.POST("/email/verify", auth.VerifyEmail)

	// Backup Routes (no email verification required — same as Laravel)
	protected.POST("/user/backup", apiControllers.BackupStore)
	protected.GET("/user/restore", apiControllers.BackupRestore)

	// Drive Backup Routes (no email verification required — same as Laravel)
	protected.POST("/user/drive-backup", apiControllers.DriveBackupStore)
	protected.POST("/user/drive-delete", apiControllers.DriveDeleteBackup)
	protected.POST("/user/drive-download", apiControllers.DriveDownloadBackup)
	protected.GET("/user/drive-list", apiControllers.DriveBackupList)

	// Cek Apakah User ini Sudah Set Password Atau Belum ( Untuk Endpoint Ubah Password Nanti )
	// Supaya bisa Hilang / Munculkan Input Current Password yang Opsional Untuk Login Google ( kalo belum punya akun biasa )
	// Atau memeriksa apakah akun user terkait dengan akun google
	protected.GET("/user/has_local_password", auth.CheckHasLocalPassword)
	protected.GET("/user/has_google_auth", auth.CheckIsUserOauth)

	// User Profile Routes (no email verification required — same as Laravel)
	protected.POST("/user/update-email", auth.UpdateEmail)
	protected.POST("/user/update-name", auth.UpdateName)
	protected.POST("/user/update-password", auth.UpdatePassword)
	protected.POST("/user/update-persona", auth.UpdatePersona)

	// Bond System dihapus total (tidak ada lagi recruit/XP/level/relationship).
	// Tabel bond lama dibiarkan di DB (tidak dipakai lagi).

	// Stories & Calendar Routes (no email verification required — same as Laravel)
	protected.GET("/v1/stories", apiControllers.GetByBondedBots)
	protected.POST("/v1/calendar", apiControllers.CalendarIndex)

	// Categories, Bot Tab & Recruited Routes (no email verification required — same as Laravel)
	protected.GET("/v1/categories", apiControllers.CategoryIndex)
	protected.GET("/v1/category/:slug", apiControllers.CategoryBots)
	protected.GET("/v1/bot-tab", apiControllers.BotTabIndex)

	// Bot Bio & Recruited Routes (no email verification required — same as Laravel)
	protected.GET("/v1/bot-bio/:bot_id", apiControllers.ShowBio)
	protected.GET("/v1/recruited-bots", apiControllers.RecruitedBotsIndex)

	// Activity tracking (api_hit logging for protected routes)
	protected.Use(middlewares.ActivityLogger())

	// Verified-only routes (require email verification)
	verified := protected.Group("/", middlewares.EnsureEmailIsVerified())
	{
		// Bot Routes
		verified.GET("/bots", apiControllers.BotIndex)

		// ─── System AI Chat (server-side keys) ───
		verified.POST("/ai/chat/system", apiControllers.SystemChat)
		verified.POST("/ai/summarize", apiControllers.Summarize)
		verified.GET("/ai/system-prompt/:bot_id", apiControllers.GetSystemPrompt)
		// Fase 1: tokenizer debug
		verified.GET("/ai/tokenize", apiControllers.Tokenize)
		verified.POST("/ai/tokenize", apiControllers.Tokenize)

		// ─── Call ( WEBRTC ) - Belum di Implementasi ───
		// verified.GET("")

		// ─── BYOK Chat Status (shared polling) ───
		verified.POST("/chat/upload", apiControllers.ChatUpload)
		verified.GET("/chat/status/:uuid", byokControllers.ChatStatus)
		// Fase 3: SSE streaming (polling fallback tetap)
		verified.GET("/chat/stream/:uuid", apiControllers.ChatStream)

		// ─── BYOK Endpoints ───
		verified.POST("/byok/gemini/chat", byokControllers.GeminiChat)
		verified.POST("/byok/claude/chat", byokControllers.ClaudeChat)
		verified.POST("/byok/mistral/chat", byokControllers.MistralChat)
		verified.POST("/byok/deepseek/chat", byokControllers.DeepSeekChat)
		verified.POST("/byok/openrouter/chat", byokControllers.OpenRouterChat)
		verified.POST("/byok/zai/chat", byokControllers.ZAIChat)
		verified.POST("/byok/mock/chat", byokControllers.MockChat)
		verified.POST("/byok/openai-compatible/chat", byokControllers.OpenAICompatibleChat)
		verified.POST("/byok/models", byokControllers.FetchModels)
		// Fase 2: registry read-only
		verified.GET("/llm/providers", apiControllers.ListProviders)
	}

	// ─── Admin Panel ───
	admin := protected.Group("/admin", middlewares.CheckAdmin())
	{
		admin.GET("/users", apiControllers.AdminGetUsers)
		admin.PUT("/users/:id/role", apiControllers.AdminChangeUserRole)
		admin.POST("/users/:id/make-vip", apiControllers.AdminMakeVip)
		admin.DELETE("/users/:id", apiControllers.AdminDeleteUser)

		admin.GET("/activities", apiControllers.AdminGetActivities)
		admin.GET("/activities/summary", apiControllers.AdminGetActivitiesSummary)
		admin.GET("/bots", apiControllers.AdminGetBots)
		admin.POST("/bots", apiControllers.AdminStoreBot)
		admin.POST("/bots/import", apiControllers.AdminImportBots)
		admin.PUT("/bots/:bot_id", apiControllers.AdminUpdateBot)
		admin.DELETE("/bots/:bot_id", apiControllers.AdminDeleteBot)

		admin.GET("/settings", apiControllers.AdminGetSettings)
		admin.PUT("/settings", apiControllers.AdminUpdateSettings)

		admin.GET("/stats", apiControllers.AdminGetStats)
		admin.GET("/stats/charts", apiControllers.AdminGetCharts)

		admin.GET("/categories", apiControllers.AdminGetCategories)
		admin.POST("/categories", apiControllers.AdminStoreCategory)
		admin.PUT("/categories/:id", apiControllers.AdminUpdateCategory)
		admin.DELETE("/categories/:id", apiControllers.AdminDeleteCategory)

		admin.POST("/bots/bulk-assign-category", apiControllers.AdminBulkAssignCategory)
		admin.POST("/bots/bulk-store", apiControllers.AdminBulkStoreBots)

		admin.POST("/upload", apiControllers.AdminUploadFile)

		// Fase 4: Lorebook STAFF-BASED (admin only, per karakter)
		admin.GET("/bots/:bot_id/lorebooks", adminControllers.ListLorebooks)
		admin.POST("/bots/:bot_id/lorebooks", adminControllers.CreateLorebook)
		admin.PUT("/bots/:bot_id/lorebooks/:id", adminControllers.UpdateLorebook)
		admin.DELETE("/bots/:bot_id/lorebooks/:id", adminControllers.DeleteLorebook)
		admin.POST("/bots/:bot_id/lorebooks/preview", adminControllers.PreviewLorebook)

		// Fase 5: Prompt templating STAFF-BASED (admin only)
		admin.GET("/prompts/templates", adminControllers.ListPromptTemplates)
		admin.POST("/prompts/render", adminControllers.RenderPromptPreview)
		admin.PUT("/prompts/templates", adminControllers.UpdatePromptTemplate)
	}
}
