package admin

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services/memory"

	"github.com/gin-gonic/gin"
)

type LorebookRequest struct {
	Keys           []string `json:"keys" binding:"required"`
	Content        string   `json:"content" binding:"required"`
	Enabled        *bool    `json:"enabled"`
	IsGlobal       *bool    `json:"is_global"`
	InsertionOrder *int     `json:"insertion_order"`
}

var blockSystem = regexp.MustCompile(`(?i)\{\{\s*system`)

func validateLorebook(req LorebookRequest) (string, bool) {
	if len(req.Keys) == 0 || len(req.Keys) > 20 {
		return "keys must be 1-20 items", false
	}
	for _, k := range req.Keys {
		if strings.TrimSpace(k) == "" || len(k) > 100 {
			return "each key must be 1-100 chars", false
		}
	}
	if len(req.Content) == 0 || len(req.Content) > 2000 {
		return "content must be 1-2000 chars", false
	}
	if blockSystem.MatchString(req.Content) {
		return "content contains blocked pattern {{system}}", false
	}
	if strings.Contains(strings.ToLower(req.Content), "<script") {
		return "content contains blocked html", false
	}
	if req.InsertionOrder != nil && (*req.InsertionOrder < 0 || *req.InsertionOrder > 100) {
		return "insertion_order must be 0-100", false
	}
	return "", true
}

func ListLorebooks(c *gin.Context) {
	botID := c.Param("bot_id")
	var list []models.Lorebook
	if err := database.DB.Where("bot_id = ?", botID).Order("insertion_order ASC").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = make([]models.Lorebook, 0)
	}
	c.JSON(http.StatusOK, gin.H{"lorebooks": list})
}

func CreateLorebook(c *gin.Context) {
	botID := c.Param("bot_id")
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var req LorebookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if msg, ok := validateLorebook(req); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	keysJSON, _ := json.Marshal(req.Keys)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	isGlobal := false
	if req.IsGlobal != nil {
		isGlobal = *req.IsGlobal
	}
	order := 0
	if req.InsertionOrder != nil {
		order = *req.InsertionOrder
	}
	lb := models.Lorebook{
		BotID: botID, Keys: string(keysJSON), Content: req.Content,
		Enabled: enabled, IsGlobal: isGlobal, InsertionOrder: order, CreatedBy: user.ID,
	}
	if err := database.DB.Create(&lb).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	memory.InvalidateLoreCache(botID)
	c.JSON(http.StatusCreated, lb)
}

func UpdateLorebook(c *gin.Context) {
	botID := c.Param("bot_id")
	id := c.Param("id")
	var lb models.Lorebook
	if err := database.DB.Where("id = ? AND bot_id = ?", id, botID).First(&lb).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "lorebook not found"})
		return
	}
	var req LorebookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if msg, ok := validateLorebook(req); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	keysJSON, _ := json.Marshal(req.Keys)
	lb.Keys = string(keysJSON)
	lb.Content = req.Content
	if req.Enabled != nil {
		lb.Enabled = *req.Enabled
	}
	if req.IsGlobal != nil {
		lb.IsGlobal = *req.IsGlobal
	}
	if req.InsertionOrder != nil {
		lb.InsertionOrder = *req.InsertionOrder
	}
	database.DB.Save(&lb)
	memory.InvalidateLoreCache(botID)
	c.JSON(http.StatusOK, lb)
}

func DeleteLorebook(c *gin.Context) {
	botID := c.Param("bot_id")
	id := c.Param("id")
	if err := database.DB.Where("id = ? AND bot_id = ?", id, botID).Delete(&models.Lorebook{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	memory.InvalidateLoreCache(botID)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func PreviewLorebook(c *gin.Context) {
	botID := c.Param("bot_id")
	var body struct {
		Messages []string `json:"messages"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	lbs := memory.GetLorebooks(botID)
	matched := memory.Scan(body.Messages, lbs)
	c.JSON(http.StatusOK, gin.H{"matched": matched, "injection": memory.BuildLorebookInjection(matched)})
}
