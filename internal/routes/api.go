package routes

import (
	"net/http"

	apiControllers "halostory_go_backend/internal/app/http/controllers/api"
	byokControllers "halostory_go_backend/internal/app/http/controllers/api/byok"
	"halostory_go_backend/internal/app/http/controllers/auth"
	"halostory_go_backend/internal/app/http/middlewares"
	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
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

	// Protected routes tanpa banned middleware (biar banned user bisa report device)
	api.POST("/user/report-banned-device", middlewares.Protected(), auth.ReportBannedDevice)

	// Protected routes (membutuhkan JWT dan tidak dibanned)
	protected := api.Group("/", middlewares.Protected(), middlewares.CheckBanned())

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

	// Bond Routes (no email verification required — same as Laravel)
	protected.POST("/bots/recruit", apiControllers.BondRecruit)
	protected.GET("/bots/bond/:bot_id", apiControllers.BondShow)
	protected.POST("/bots/bond/:bot_id/choice", apiControllers.BondHandleChoice)
	protected.POST("/bots/bond/:bot_id/debug", apiControllers.BondDebugUpdate)

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

	// Student Request Routes (no email verification required — same as Laravel)
	protected.GET("/student-requests", apiControllers.StudentRequestIndex)
	protected.POST("/student-requests", apiControllers.StudentRequestStore)
	protected.POST("/student-request/check", apiControllers.StudentRequestCheck)

	// Verified-only routes (require email verification)
	verified := protected.Group("/", middlewares.EnsureEmailIsVerified())
	{
		// Bot Routes
		verified.GET("/bots", apiControllers.BotIndex)

		// ─── System AI Chat (server-side keys) ───
		verified.POST("/ai/chat/system", apiControllers.SystemChat)
		verified.POST("/ai/summarize", apiControllers.Summarize)
		verified.GET("/ai/system-prompt/:bot_id", apiControllers.GetSystemPrompt)

		// ─── Call ( WEBRTC ) - Belum di Implementasi ───
		// verified.GET("")

		// ─── BYOK Chat Status (shared polling) ───
		verified.GET("/chat/status/:uuid", byokControllers.ChatStatus)

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
	}

	// ─── Admin Panel ───
	admin := protected.Group("/admin", middlewares.CheckAdmin())
	{
		admin.GET("/users", apiControllers.AdminGetUsers)
		admin.PUT("/users/:id/role", apiControllers.AdminChangeUserRole)
		admin.POST("/users/:id/make-vip", apiControllers.AdminMakeVip)
		admin.POST("/users/:id/ban", apiControllers.AdminBanUser)
		admin.POST("/users/:id/unban", apiControllers.AdminUnbanUser)
		admin.DELETE("/users/:id", apiControllers.AdminDeleteUser)

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

		admin.GET("/student-requests", apiControllers.AdminGetStudentRequests)
		admin.PUT("/student-requests/:id/status", apiControllers.AdminUpdateStudentRequestStatus)
	}
}
