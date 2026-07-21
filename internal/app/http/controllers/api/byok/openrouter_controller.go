package byok

import (
	"net/http"

	byokSvc "halostory_go_backend/internal/services/byok"

	"github.com/gin-gonic/gin"
)

func OpenRouterChat(c *gin.Context) {
	var req OpenAIProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":   "Validation failed",
			"message": err.Error(),
		})
		return
	}

	enqueueProviderChat(c, req, "https://openrouter.ai/api/v1", "openrouter", byokSvc.DefaultOpenAIModel)
}
