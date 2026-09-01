package api

import (
	"net/http"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/services/tokenizer"

	"github.com/gin-gonic/gin"
)

type TokenizeRequest struct {
	Text  string `form:"text" json:"text" binding:"required"`
	Model string `form:"model" json:"model"`
}

type TokenizeResponse struct {
	Text   string `json:"text"`
	Model  string `json:"model"`
	Tokens int    `json:"tokens"`
	Method string `json:"method"`
}

func Tokenize(c *gin.Context) {
	if config.Env("APP_DEBUG", "false") != "true" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	var req TokenizeRequest
	if err := c.ShouldBind(&req); err != nil {
		// try query
		req.Text = c.Query("text")
		req.Model = c.Query("model")
		if req.Text == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "text required"})
			return
		}
	}
	if req.Model == "" {
		req.Model = "gpt-3.5-turbo"
	}
	tokens := tokenizer.Default.CountTokens(req.Text, req.Model)
	method := "tiktoken"
	if !tokenizer.Default.IsTiktoken(req.Model) {
		method = "heuristic"
	}
	c.JSON(http.StatusOK, TokenizeResponse{
		Text:   req.Text,
		Model:  req.Model,
		Tokens: tokens,
		Method: method,
	})
}
