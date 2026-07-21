package api

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

// ───────────────────── Admin: Users ─────────────────────

type adminUserItem struct {
	ID           uint       `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	IsBanned     bool       `json:"is_banned"`
	BanReason    *string    `json:"ban_reason,omitempty"`
	VipExpiresAt *time.Time `json:"vip_expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func AdminGetUsers(c *gin.Context) {
	var users []models.User
	database.DB.Order("created_at desc").Find(&users)

	response := make([]adminUserItem, 0)
	for _, u := range users {
		response = append(response, adminUserItem{
			ID:           u.ID,
			Name:         u.Name,
			Email:        u.Email,
			Role:         u.Role,
			IsBanned:     u.IsBanned,
			BanReason:    u.BanReason,
			VipExpiresAt: u.VipExpiresAt,
			CreatedAt:    u.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

func AdminChangeUserRole(c *gin.Context) {
	userID := c.Param("id")

	userVal, _ := c.Get("user")
	currentUser := userVal.(models.User)

	var req struct {
		Role string `json:"role" form:"role"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	req.Role = strings.TrimSpace(req.Role)
	if req.Role != "admin" && req.Role != "Sensei" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Role harus 'admin' atau 'Sensei'.",
		})
		return
	}

	// Prevent self-demotion to avoid locking out the only admin
	if fmt.Sprint(currentUser.ID) == userID && req.Role != "admin" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Cannot demote yourself.",
		})
		return
	}

	result := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("role", req.Role)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User tidak ditemukan.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Role updated successfully",
	})
}

func AdminDeleteUser(c *gin.Context) {
	userID := c.Param("id")

	userVal, _ := c.Get("user")
	currentUser := userVal.(models.User)

	if fmt.Sprint(currentUser.ID) == userID {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Cannot delete yourself.",
		})
		return
	}

	result := database.DB.Delete(&models.User{}, userID)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User tidak ditemukan.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
	})
}

// ───────────────────── Admin: Bots ─────────────────────

type adminBotItem struct {
	BotID        string     `json:"id"`
	Name         string     `json:"name"`
	CategorySlug *string    `json:"category_slug"`
	Description  *string    `json:"description"`
	Avatar       *string    `json:"avatar"`
	Jailbreak    *string    `json:"background"`
	Personality  *string    `json:"personality"`
	ChatExamples *string    `json:"chat_examples"`
	Birthday     *string    `json:"birthday"`
	Sbr          *string    `json:"sbr"`
	Color        *string    `json:"color"`
	Child        bool       `json:"child"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func AdminGetBots(c *gin.Context) {
	var bots []models.Bot
	database.DB.Order("created_at desc").Find(&bots)

	response := make([]adminBotItem, 0)
	for _, bot := range bots {
		item := adminBotItem{
			BotID:        bot.BotID,
			Name:         bot.Name,
			CategorySlug: bot.CategorySlug,
			Description:  bot.Description,
			Avatar:       bot.Avatar,
			Jailbreak:    bot.Jailbreak,
			Personality:  bot.Personality,
			ChatExamples: bot.ChatExamples,
			Sbr:          bot.Sbr,
			Color:        bot.Color,
			Child:        bot.Child,
			CreatedAt:    bot.CreatedAt,
			UpdatedAt:    bot.UpdatedAt,
		}

		if bot.Birthday != nil {
			str := bot.Birthday.Format("2006-01-02")
			item.Birthday = &str
		}

		response = append(response, item)
	}

	c.JSON(http.StatusOK, response)
}

func AdminStoreBot(c *gin.Context) {
	var req struct {
		BotID        string  `json:"bot_id" form:"bot_id"`
		Name         string  `json:"name" form:"name"`
		CategorySlug *string `json:"category_slug" form:"category_slug"`
		Description  *string `json:"description" form:"description"`
		Avatar       *string `json:"avatar" form:"avatar"`
		Jailbreak    *string `json:"background" form:"background"`
		Personality  *string `json:"personality" form:"personality"`
		ChatExamples *string `json:"chat_examples" form:"chat_examples"`
		Birthday     *string `json:"birthday" form:"birthday"`
		Sbr          *string `json:"sbr" form:"sbr"`
		Color        *string `json:"color" form:"color"`
		Child        *bool   `json:"child" form:"child"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	if strings.TrimSpace(req.BotID) == "" || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "bot_id dan name wajib diisi.",
		})
		return
	}

	var count int64
	database.DB.Model(&models.Bot{}).Where("bot_id = ?", req.BotID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"message": "Bot ID sudah ada.",
		})
		return
	}

	var birthday *time.Time
	if req.Birthday != nil && *req.Birthday != "" {
		t, err := time.Parse("2006-01-02", *req.Birthday)
		if err == nil {
			birthday = &t
		}
	}

	child := false
	if req.Child != nil {
		child = *req.Child
	}

	bot := models.Bot{
		BotID:        req.BotID,
		Name:         req.Name,
		CategorySlug: req.CategorySlug,
		Description:  req.Description,
		Avatar:       req.Avatar,
		Jailbreak:    req.Jailbreak,
		Personality:  req.Personality,
		ChatExamples: req.ChatExamples,
		Birthday:     birthday,
		Sbr:          req.Sbr,
		Color:        req.Color,
		Child:        child,
	}

	if err := database.DB.Create(&bot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal membuat bot.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Bot created successfully",
		"bot":     bot,
	})
}

func AdminUpdateBot(c *gin.Context) {
	botID := c.Param("id")

	var bot models.Bot
	if err := database.DB.Where("bot_id = ?", botID).First(&bot).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Bot tidak ditemukan.",
		})
		return
	}

	var req struct {
		BotID        *string `json:"bot_id" form:"bot_id"`
		Name         *string `json:"name" form:"name"`
		CategorySlug *string `json:"category_slug" form:"category_slug"`
		Description  *string `json:"description" form:"description"`
		Avatar       *string `json:"avatar" form:"avatar"`
		Jailbreak    *string `json:"background" form:"background"`
		Personality  *string `json:"personality" form:"personality"`
		ChatExamples *string `json:"chat_examples" form:"chat_examples"`
		Birthday     *string `json:"birthday" form:"birthday"`
		Sbr          *string `json:"sbr" form:"sbr"`
		Color        *string `json:"color" form:"color"`
		Child        *bool   `json:"child" form:"child"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	updates := map[string]interface{}{}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"message": "Nama tidak boleh kosong.",
			})
			return
		}
		updates["name"] = *req.Name
	}

	if req.BotID != nil {
		newID := strings.TrimSpace(*req.BotID)
		if newID != botID {
			var count int64
			database.DB.Model(&models.Bot{}).Where("bot_id = ?", newID).Count(&count)
			if count > 0 {
				c.JSON(http.StatusConflict, gin.H{
					"message": "Bot ID sudah ada.",
				})
				return
			}
			updates["bot_id"] = newID
		}
	}

	if req.CategorySlug != nil {
		updates["category_slug"] = *req.CategorySlug
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}
	if req.Jailbreak != nil {
		updates["jailbreak"] = *req.Jailbreak
	}
	if req.Personality != nil {
		updates["personality"] = *req.Personality
	}
	if req.ChatExamples != nil {
		updates["chat_examples"] = *req.ChatExamples
	}
	if req.Sbr != nil {
		updates["sbr"] = *req.Sbr
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}
	if req.Child != nil {
		updates["child"] = *req.Child
	}
	if req.Birthday != nil {
		if *req.Birthday == "" {
			updates["birthday"] = nil
		} else {
			t, err := time.Parse("2006-01-02", *req.Birthday)
			if err == nil {
				updates["birthday"] = t
			}
		}
	}

	if len(updates) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Tidak ada field yang diupdate.",
		})
		return
	}

	database.DB.Model(&bot).Updates(updates)

	c.JSON(http.StatusOK, gin.H{
		"message": "Bot updated successfully",
	})
}

func AdminDeleteBot(c *gin.Context) {
	botID := c.Param("id")

	result := database.DB.Where("bot_id = ?", botID).Delete(&models.Bot{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Bot tidak ditemukan.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Bot deleted successfully",
	})
}

// ───────────────────── Admin: Bot Import ─────────────────────

func AdminImportBots(c *gin.Context) {
	file, header, err := c.Request.FormFile("csv_file")
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "File csv_file wajib diupload.",
		})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "File harus berformat CSV.",
		})
		return
	}

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Gagal membaca header CSV.",
		})
		return
	}

	colMap := make(map[string]int)
	for i, h := range headers {
		colMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"bot_id", "name"}
	for _, r := range required {
		if _, ok := colMap[r]; !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": fmt.Sprintf("Kolom '%s' wajib ada di CSV.", r),
			})
			return
		}
	}

	imported := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		getField := func(key string) string {
			if idx, ok := colMap[key]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		botID := getField("bot_id")
		name := getField("name")
		if botID == "" || name == "" {
			continue
		}

		var birthday *time.Time
		if b := getField("birthday"); b != "" {
			t, err := time.Parse("2006-01-02", b)
			if err == nil {
				birthday = &t
			}
		}

		child := getField("child") == "1" || getField("child") == "true"

		bot := models.Bot{
			BotID:        botID,
			Name:         name,
			CategorySlug: strPtr(getField("category_slug")),
			Description:  strPtr(getField("description")),
			Avatar:       strPtr(getField("avatar")),
			Jailbreak:    strPtr(getField("background")),
			Personality:  strPtr(getField("personality")),
			ChatExamples: strPtr(getField("chat_examples")),
			Birthday:     birthday,
			Sbr:          strPtr(getField("sbr")),
			Color:        strPtr(getField("color")),
			Child:        child,
		}

		var existing models.Bot
		result := database.DB.Where("bot_id = ?", botID).First(&existing)
		if result.Error == nil {
			bot.CreatedAt = existing.CreatedAt
			database.DB.Where("bot_id = ?", botID).Updates(map[string]interface{}{
				"name":          bot.Name,
				"category_slug": bot.CategorySlug,
				"description":   bot.Description,
				"avatar":        bot.Avatar,
				"jailbreak":     bot.Jailbreak,
				"personality":   bot.Personality,
				"chat_examples": bot.ChatExamples,
				"birthday":      bot.Birthday,
				"sbr":           bot.Sbr,
				"color":         bot.Color,
				"child":         bot.Child,
			})
		} else {
			database.DB.Create(&bot)
		}

		imported++
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("%d bots imported successfully", imported),
		"imported": imported,
	})
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ───────────────────── Admin: Student Requests ─────────────────────

type adminStudentRequestItem struct {
	ID          uint   `json:"id"`
	StudentName string `json:"student_name"`
	Status      string `json:"status"`
	UserName    string `json:"user_name"`
	UserID      uint   `json:"user_id"`
	CreatedAt   string `json:"created_at"`
}

func AdminGetStudentRequests(c *gin.Context) {
	var requests []models.StudentRequest
	database.DB.Order("FIELD(status, 'pending') DESC, created_at DESC").Find(&requests)

	var userIDs []uint
	for _, r := range requests {
		userIDs = append(userIDs, r.UserID)
	}

	userMap := make(map[uint]string)
	if len(userIDs) > 0 {
		type userRow struct {
			ID   uint
			Name string
		}
		var users []userRow
		database.DB.Model(&models.User{}).
			Where("id IN ?", userIDs).
			Select("id, name").
			Find(&users)
		for _, u := range users {
			userMap[u.ID] = u.Name
		}
	}

	response := make([]adminStudentRequestItem, 0)
	for _, r := range requests {
		userName := userMap[r.UserID]
		if userName == "" {
			userName = "Unknown"
		}

		response = append(response, adminStudentRequestItem{
			ID:          r.ID,
			StudentName: r.StudentName,
			Status:      r.Status,
			UserName:    userName,
			UserID:      r.UserID,
			CreatedAt:   r.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.JSON(http.StatusOK, response)
}

func AdminUpdateStudentRequestStatus(c *gin.Context) {
	requestID := c.Param("id")

	var req struct {
		Status string `json:"status" form:"status"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	req.Status = strings.TrimSpace(req.Status)
	if req.Status != "pending" && req.Status != "approved" && req.Status != "rejected" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Status harus 'pending', 'approved', atau 'rejected'.",
		})
		return
	}

	var reqRecord models.StudentRequest
	if err := database.DB.Where("id = ?", requestID).First(&reqRecord).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Student request tidak ditemukan.",
		})
		return
	}

	oldStatus := reqRecord.Status

	database.DB.Model(&reqRecord).Update("status", req.Status)

	if oldStatus == "pending" && req.Status == "approved" {
		var user models.User
		if err := database.DB.First(&user, reqRecord.UserID).Error; err == nil {
			go services.SendRequestApproved(user.Name, reqRecord.StudentName)
		}
	} else if oldStatus == "pending" && req.Status == "rejected" {
		var user models.User
		if err := database.DB.First(&user, reqRecord.UserID).Error; err == nil {
			go services.SendRequestRejected(user.Name, reqRecord.StudentName)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Status updated successfully",
	})
}
