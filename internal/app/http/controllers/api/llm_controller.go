package api

import (
	"net/http"

	"halostory_go_backend/internal/services/llm"

	"github.com/gin-gonic/gin"
)

type ProvidersResponse struct {
	Providers []llm.ProviderSpec `json:"providers"`
}

func ListProviders(c *gin.Context) {
	providers := llm.Default.List()
	c.JSON(http.StatusOK, ProvidersResponse{Providers: providers})
}
