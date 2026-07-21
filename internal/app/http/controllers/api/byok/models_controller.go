package byok

import (
	"net/http"

	byokSvc "halostory_go_backend/internal/services/byok"

	"github.com/gin-gonic/gin"
)

type FetchModelsRequest struct {
	APIKey  string `json:"api_key" binding:"required"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
}

func FetchModels(c *gin.Context) {
	var req FetchModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
		return
	}

	provider := req.Provider
	if provider == "" {
		provider = "gemini"
	}

	models, err := byokSvc.FetchModels(req.APIKey, provider, req.BaseURL)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message, "details": err.Details})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}
