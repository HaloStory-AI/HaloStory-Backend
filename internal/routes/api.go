package routes

import (
	"net/http"

	apiControllers "halostory_go_backend/internal/app/http/controllers/api"
	byokControllers "halostory_go_backend/internal/app/http/controllers/api/byok"
	"halostory_go_backend/internal/services/observability"

	"github.com/gin-gonic/gin"
)

// SetupAPI mendaftarkan semua route — semua PUBLIC tanpa auth (BYOK relay tetap via queue+SSE).
func SetupAPI(r *gin.Engine) {
	r.GET("/metrics", observability.MetricsHandler)
	apiMetrics := r.Group("/api")
	apiMetrics.GET("/metrics", observability.MetricsHandler)

	api := r.Group("/api")

	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong", "status": "ok"})
	})

	// Public content
	api.GET("/image", apiControllers.ImageProxy)
	api.GET("/app/status", apiControllers.AppStatus)

	// ── Public content (dulunya protected) ──
	api.GET("/bots", apiControllers.BotIndex)
	api.GET("/v1/categories", apiControllers.CategoryIndex)
	api.GET("/v1/category/:slug", apiControllers.CategoryBots)
	api.GET("/v1/bot-tab", apiControllers.BotTabIndex)
	api.GET("/v1/bot-bio/:bot_id", apiControllers.ShowBio)
	api.GET("/v1/recruited-bots", apiControllers.RecruitedBotsIndex)
	api.GET("/v1/stories", apiControllers.GetByBondedBots)
	api.POST("/v1/calendar", apiControllers.CalendarIndex)
	api.GET("/v1/get-call", apiControllers.GetCall)

	// ── Chat upload / status / stream (public) ──
	api.POST("/chat/upload", apiControllers.ChatUpload)
	api.GET("/chat/status/:uuid", byokControllers.ChatStatus)
	api.GET("/chat/stream/:uuid", apiControllers.ChatStream)

	// ── BYOK relay (public) ──
	api.POST("/byok/gemini/chat", byokControllers.GeminiChat)
	api.POST("/byok/claude/chat", byokControllers.ClaudeChat)
	api.POST("/byok/mistral/chat", byokControllers.MistralChat)
	api.POST("/byok/deepseek/chat", byokControllers.DeepSeekChat)
	api.POST("/byok/openrouter/chat", byokControllers.OpenRouterChat)
	api.POST("/byok/zai/chat", byokControllers.ZAIChat)
	api.POST("/byok/mock/chat", byokControllers.MockChat)
	api.POST("/byok/openai-compatible/chat", byokControllers.OpenAICompatibleChat)
	api.POST("/byok/models", byokControllers.FetchModels)
	api.GET("/llm/providers", apiControllers.ListProviders)
}
