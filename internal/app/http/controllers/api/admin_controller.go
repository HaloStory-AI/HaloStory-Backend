package api

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
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
	page, perPage := getPagination(c, 25)
	q := strings.TrimSpace(c.Query("q"))

	base := database.DB.Model(&models.User{})
	if q != "" {
		like := "%" + q + "%"
		base = base.Where("name LIKE ? OR email LIKE ?", like, like)
	}

	var total int64
	base.Count(&total)

	usersQuery := database.DB.Model(&models.User{})
	if q != "" {
		like := "%" + q + "%"
		usersQuery = usersQuery.Where("name LIKE ? OR email LIKE ?", like, like)
	}

	var users []models.User
	usersQuery.Order("created_at desc").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&users)

	response := make([]adminUserItem, 0, len(users))
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

	c.JSON(http.StatusOK, adminPage[adminUserItem]{
		Items:      response,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: pageCount(total, perPage),
	})
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
	database.DB.Where("user_id = ?", userID).Delete(&models.UserActivity{})

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
	ImageURL     string     `json:"image_url"`
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
	page, perPage := getPagination(c, 25)
	q := strings.TrimSpace(c.Query("q"))

	base := database.DB.Model(&models.Bot{})
	if q != "" {
		like := "%" + q + "%"
		base = base.Where("bot_id LIKE ? OR name LIKE ?", like, like)
	}

	var total int64
	base.Count(&total)

	botsQuery := database.DB.Model(&models.Bot{})
	if q != "" {
		like := "%" + q + "%"
		botsQuery = botsQuery.Where("bot_id LIKE ? OR name LIKE ?", like, like)
	}

	var bots []models.Bot
	botsQuery.Order("category_slug asc, name asc").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&bots)

	response := make([]adminBotItem, 0, len(bots))
	for _, bot := range bots {
		item := adminBotItem{
			BotID:        bot.BotID,
			Name:         bot.Name,
			CategorySlug: bot.CategorySlug,
			Description:  bot.Description,
			Avatar:       bot.Avatar,
			ImageURL:     services.ImageURL(strPtrValue(bot.Avatar)),
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

	c.JSON(http.StatusOK, adminPage[adminBotItem]{
		Items:      response,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: pageCount(total, perPage),
	})
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
	botID := c.Param("bot_id")

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
	botID := c.Param("bot_id")

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
	page, perPage := getPagination(c, 25)
	q := strings.TrimSpace(c.Query("q"))

	var pendingCount int64
	database.DB.Model(&models.StudentRequest{}).Where("status = ?", "pending").Count(&pendingCount)

	base := database.DB.Model(&models.StudentRequest{}).
		Joins("LEFT JOIN users ON users.id = student_requests.user_id")
	if q != "" {
		like := "%" + q + "%"
		base = base.Where("student_requests.student_name LIKE ? OR users.name LIKE ?", like, like)
	}

	var total int64
	base.Count(&total)

	var requests []models.StudentRequest
	base.Order("FIELD(student_requests.status, 'pending') DESC, student_requests.created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&requests)

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

	response := make([]adminStudentRequestItem, 0, len(requests))
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

	c.JSON(http.StatusOK, struct {
		adminPage[adminStudentRequestItem]
		PendingCount int64 `json:"pending_count"`
	}{
		adminPage[adminStudentRequestItem]{
			Items:      response,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: pageCount(total, perPage),
		},
		pendingCount,
	})
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
	if req.Status != "pending" && req.Status != "approved" && req.Status != "rejected" && req.Status != "silent_approved" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Status harus 'pending', 'approved', 'rejected', atau 'silent_approved'.",
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

	actualStatus := req.Status
	if req.Status == "silent_approved" {
		actualStatus = "approved"
	}

	database.DB.Model(&reqRecord).Update("status", actualStatus)

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

// ───────────────────── Admin: VIP ─────────────────────

func AdminMakeVip(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Duration int `json:"duration" form:"duration"`
	}
	if err := c.ShouldBind(&req); err != nil || req.Duration < 1 || req.Duration > 3650 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Duration wajib diisi (1-3650 hari).",
		})
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User tidak ditemukan.",
		})
		return
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, req.Duration)

	// Extend if already VIP
	if user.Role == "vip" && user.VipExpiresAt != nil && user.VipExpiresAt.After(now) {
		expiresAt = user.VipExpiresAt.AddDate(0, 0, req.Duration)
	}

	database.DB.Model(&user).Updates(map[string]interface{}{
		"role":           "vip",
		"vip_started_at": now,
		"vip_expires_at": expiresAt,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":       "VIP granted successfully",
		"vip_expires_at": expiresAt,
	})
}

// ───────────────────── Admin: Ban / Unban ─────────────────────

func AdminBanUser(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Reason   string `json:"reason" form:"reason"`
		Duration string `json:"duration" form:"duration"` // "permanent", "1", "7", "30"
	}
	if err := c.ShouldBind(&req); err != nil || strings.TrimSpace(req.Reason) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Reason wajib diisi.",
		})
		return
	}

	req.Duration = strings.TrimSpace(req.Duration)

	var bannedUntil *time.Time
	switch req.Duration {
	case "1":
		t := time.Now().AddDate(0, 0, 1)
		bannedUntil = &t
	case "7":
		t := time.Now().AddDate(0, 0, 7)
		bannedUntil = &t
	case "30":
		t := time.Now().AddDate(0, 0, 30)
		bannedUntil = &t
	case "permanent", "":
		bannedUntil = nil
	default:
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Duration harus 'permanent', '1', '7', atau '30'.",
		})
		return
	}

	reason := req.Reason
	result := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"is_banned":    true,
		"ban_reason":   reason,
		"banned_until": bannedUntil,
	})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User tidak ditemukan.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User banned successfully",
	})
}

func AdminUnbanUser(c *gin.Context) {
	userID := c.Param("id")

	result := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"is_banned":    false,
		"ban_reason":   nil,
		"banned_until": nil,
	})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "User tidak ditemukan.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User unbanned successfully",
	})
}

// ───────────────────── Admin: Settings ─────────────────────

func AdminGetSettings(c *gin.Context) {
	var settings []models.AppSetting
	database.DB.Find(&settings)

	result := make(map[string]*string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}

	c.JSON(http.StatusOK, result)
}

func AdminUpdateSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	for key, val := range req {
		var strVal string
		switch v := val.(type) {
		case string:
			strVal = v
		case bool:
			strVal = fmt.Sprintf("%t", v)
		default:
			continue
		}

		var existing models.AppSetting
		result := database.DB.Where("`key` = ?", key).First(&existing)
		if result.Error == nil {
			database.DB.Model(&existing).Update("value", strVal)
		} else {
			database.DB.Create(&models.AppSetting{Key: key, Value: &strVal})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Settings updated successfully",
	})
}

// ───────────────────── Admin: Stats ─────────────────────

type adminStatsResponse struct {
	HourlyUsers int64 `json:"hourly_users"`
	DailyUsers  int64 `json:"daily_users"`
	TotalUsers  int64 `json:"total_users"`
	TotalBots   int64 `json:"total_bots"`
}

func AdminGetStats(c *gin.Context) {
	var stats adminStatsResponse

	database.DB.Model(&models.UserActivity{}).
		Where("created_at >= ?", time.Now().Add(-time.Hour)).
		Select("COUNT(DISTINCT user_id)").Scan(&stats.HourlyUsers)

	database.DB.Model(&models.UserActivity{}).
		Where("created_at >= ?", time.Now().AddDate(0, 0, -1)).
		Select("COUNT(DISTINCT user_id)").Scan(&stats.DailyUsers)

	database.DB.Model(&models.User{}).Select("COUNT(*)").Scan(&stats.TotalUsers)
	database.DB.Model(&models.Bot{}).Select("COUNT(*)").Scan(&stats.TotalBots)

	c.JSON(http.StatusOK, stats)
}

// ───────────────────── Admin: Categories ─────────────────────

type adminCategoryItem struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	LogoUrl   *string   `json:"logo_url"`
	ImageURL  string    `json:"image_url"`
	Pinned    bool      `json:"pinned"`
	Featured  bool      `json:"featured"`
	BotCount  int64     `json:"bot_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func AdminGetCategories(c *gin.Context) {
	var categories []models.Category
	database.DB.Order("name asc").Find(&categories)

	type slugCount struct {
		CategorySlug string
		Count        int64
	}
	var counts []slugCount
	database.DB.Model(&models.Bot{}).
		Select("category_slug, COUNT(*) AS count").
		Group("category_slug").
		Scan(&counts)

	countMap := make(map[string]int64, len(counts))
	for _, sc := range counts {
		countMap[sc.CategorySlug] = sc.Count
	}

	response := make([]adminCategoryItem, 0, len(categories))
	for _, cat := range categories {
		response = append(response, adminCategoryItem{
			ID:        cat.ID,
			Name:      cat.Name,
			Slug:      cat.Slug,
			LogoUrl:   cat.LogoUrl,
			ImageURL:  services.ImageURL(strPtrValue(cat.LogoUrl)),
			Pinned:    cat.Pinned,
			Featured:  cat.Featured,
			BotCount:  countMap[cat.Slug],
			CreatedAt: cat.CreatedAt,
			UpdatedAt: cat.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

func AdminStoreCategory(c *gin.Context) {
	var req struct {
		Name     string  `json:"name" form:"name"`
		Slug     string  `json:"slug" form:"slug"`
		LogoUrl  *string `json:"logo_url" form:"logo_url"`
		Pinned   *bool   `json:"pinned" form:"pinned"`
		Featured *bool   `json:"featured" form:"featured"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "Format request tidak valid"})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "Nama kategori wajib diisi."})
		return
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}

	pinned := false
	if req.Pinned != nil {
		pinned = *req.Pinned
	}
	featured := false
	if req.Featured != nil {
		featured = *req.Featured
	}

	if pinned && featured {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "Pinned dan Featured tidak bisa aktif bersamaan."})
		return
	}

	var count int64
	database.DB.Model(&models.Category{}).Where("slug = ?", slug).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"message": "Slug sudah ada."})
		return
	}

	cat := models.Category{
		Name:     req.Name,
		Slug:     slug,
		LogoUrl:  req.LogoUrl,
		Pinned:   pinned,
		Featured: featured,
	}
	if err := database.DB.Create(&cat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membuat kategori."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Category created successfully", "category": cat})
}

func AdminUpdateCategory(c *gin.Context) {
	id := c.Param("id")

	var cat models.Category
	if err := database.DB.First(&cat, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Kategori tidak ditemukan."})
		return
	}

	var req struct {
		Name     *string `json:"name" form:"name"`
		Slug     *string `json:"slug" form:"slug"`
		LogoUrl  *string `json:"logo_url" form:"logo_url"`
		Pinned   *bool   `json:"pinned" form:"pinned"`
		Featured *bool   `json:"featured" form:"featured"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "Format request tidak valid"})
		return
	}

	pinned := cat.Pinned
	featured := cat.Featured
	if req.Pinned != nil {
		pinned = *req.Pinned
	}
	if req.Featured != nil {
		featured = *req.Featured
	}
	if pinned && featured {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "Pinned dan Featured tidak bisa aktif bersamaan."})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Slug != nil {
		slug := *req.Slug
		if slug == "" {
			slug = strings.ToLower(strings.ReplaceAll(cat.Name, " ", "-"))
		}

		var count int64
		database.DB.Model(&models.Category{}).Where("slug = ? AND id != ?", slug, cat.ID).Count(&count)
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"message": "Slug sudah digunakan kategori lain."})
			return
		}
		updates["slug"] = slug
	}
	if req.LogoUrl != nil {
		updates["logo_url"] = req.LogoUrl
	}
	if req.Pinned != nil {
		updates["pinned"] = pinned
	}
	if req.Featured != nil {
		updates["featured"] = featured
	}

	if len(updates) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "Tidak ada field yang diupdate."})
		return
	}

	database.DB.Model(&cat).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"message": "Category updated successfully"})
}

func AdminDeleteCategory(c *gin.Context) {
	id := c.Param("id")

	var cat models.Category
	if err := database.DB.First(&cat, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Kategori tidak ditemukan."})
		return
	}

	database.DB.Model(&models.Bot{}).Where("category_slug = ?", cat.Slug).Update("category_slug", nil)
	database.DB.Delete(&cat)

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted successfully"})
}

// ───────────────────── Admin: Bulk Assign Category ─────────────────────

func AdminBulkAssignCategory(c *gin.Context) {
	var req struct {
		BotIDs       []string `json:"bot_ids" form:"bot_ids"`
		CategorySlug string   `json:"category_slug" form:"category_slug"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "Format request tidak valid"})
		return
	}

	if len(req.BotIDs) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "bot_ids wajib diisi."})
		return
	}

	// category_slug kosong/omit = hapus kategori dari bot terpilih (seperti panel lama).
	var categorySlug *string
	if strings.TrimSpace(req.CategorySlug) != "" {
		var cat models.Category
		if err := database.DB.Where("slug = ?", req.CategorySlug).First(&cat).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Kategori tidak ditemukan."})
			return
		}
		slug := req.CategorySlug
		categorySlug = &slug
	}

	result := database.DB.Model(&models.Bot{}).Where("bot_id IN ?", req.BotIDs).
		Update("category_slug", categorySlug)

	message := fmt.Sprintf("%d bots assigned to category", result.RowsAffected)
	if categorySlug == nil {
		message = fmt.Sprintf("%d bots removed from category", result.RowsAffected)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"updated": result.RowsAffected,
	})
}

// ───────────────────── Admin: Bulk Store Bots ─────────────────────

func AdminBulkStoreBots(c *gin.Context) {
	var req struct {
		Entries []botBulkEntry `json:"entries" form:"entries"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "Format request tidak valid"})
		return
	}

	if len(req.Entries) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "entries wajib diisi."})
		return
	}

	imported := 0
	skipped := 0
	for _, entry := range req.Entries {
		botID := strings.TrimSpace(entry.BotID)
		name := strings.TrimSpace(entry.Name)
		if botID == "" || name == "" {
			skipped++
			continue
		}

		slug := strings.TrimSpace(entry.CategorySlug)
		var birthday *time.Time
		if entry.Birthday != "" {
			t, err := time.Parse("2006-01-02", entry.Birthday)
			if err == nil {
				birthday = &t
			}
		}

		bot := models.Bot{
			BotID:        botID,
			Name:         name,
			CategorySlug: strPtrOrNil(slug),
			Description:  entry.Description,
			Avatar:       entry.Avatar,
			Jailbreak:    entry.Jailbreak,
			Personality:  entry.Personality,
			ChatExamples: entry.ChatExamples,
			Birthday:     birthday,
			Sbr:          entry.Sbr,
			Color:        entry.Color,
			Child:        entry.Child != nil && *entry.Child,
		}

		if err := database.DB.Create(&bot).Error; err != nil {
			skipped++
			continue
		}
		imported++
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("%d bots imported successfully", imported),
		"imported": imported,
		"skipped":  skipped,
	})
}

type botBulkEntry struct {
	BotID        string  `json:"bot_id"`
	Name         string  `json:"name"`
	CategorySlug string  `json:"category_slug"`
	Description  *string `json:"description"`
	Avatar       *string `json:"avatar"`
	Jailbreak    *string `json:"background"`
	Personality  *string `json:"personality"`
	ChatExamples *string `json:"chat_examples"`
	Birthday     string  `json:"birthday"`
	Sbr          *string `json:"sbr"`
	Color        *string `json:"color"`
	Child        *bool   `json:"child"`
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ───────────────────── Admin: File Upload ─────────────────────

// AdminUploadFile menyimpan file (avatar bot / logo kategori) ke S3/MinIO dan
// mengembalikan path yang bisa disimpan langsung ke kolom avatar/logo_url.
// Form: file, type ("bot"|"category"), key (bot_id / slug).
// Object key mengikuti skema lama: "Bot/{key}/{filename}" / "Category/{key}/{filename}".
func AdminUploadFile(c *gin.Context) {
	uploadType := c.PostForm("type")
	key := c.PostForm("key")

	if uploadType != "bot" && uploadType != "category" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "type harus 'bot' atau 'category'.",
		})
		return
	}
	if strings.TrimSpace(key) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "key wajib diisi.",
		})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "File 'file' wajib diupload.",
		})
		return
	}
	defer file.Close()

	filename := sanitizeFileName(header.Filename)
	if filename == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Nama file tidak valid.",
		})
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Gagal membaca file.",
		})
		return
	}
	if len(data) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "File kosong.",
		})
		return
	}

	folder := "Bot"
	if uploadType == "category" {
		folder = "Category"
	}
	objectKey := fmt.Sprintf("%s/%s/%s", folder, key, filename)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	if err := services.UploadObject(objectKey, data, contentType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal mengupload file.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path": objectKey,
		"url":  services.ImageURL(objectKey),
	})
}

// sanitizeFileName membersihkan nama file yang diupload: hanya huruf, angka,
// titik, strip, dan underscore; spasi diganti strip; dibatasi panjangnya.
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))

	var b strings.Builder
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}

	result := strings.Trim(b.String(), ".-_")
	if len(result) > 80 {
		ext := filepath.Ext(result)
		base := strings.TrimSuffix(result, ext)
		if len(base) > 40 {
			base = base[:40]
		}
		result = base + ext
	}
	return result
}

// ───────────────────── Admin: Charts ─────────────────────

type adminChartsResponse struct {
	Hourly  []chartDataPoint `json:"hourly"`
	Daily   []chartDataPoint `json:"daily"`
	Weekly  []chartDataPoint `json:"weekly"`
	Monthly []chartDataPoint `json:"monthly"`
}

type chartDataPoint struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

func AdminGetCharts(c *gin.Context) {
	now := time.Now()
	var resp adminChartsResponse

	// Format label pakai specifier MySQL (bukan pola Java/JS): "yyyy-MM-dd HH:00"
	// dulu menghasilkan string literal karena DATE_FORMAT hanya mengenali "%...".
	resp.Hourly = getChartData("hour", now.Add(-24*time.Hour), now, "%Y-%m-%d %H:00", 24)
	resp.Daily = getChartData("day", now.AddDate(0, 0, -7), now, "%Y-%m-%d", 7)
	resp.Weekly = getChartData("week", now.AddDate(0, 0, -28), now, "%x-%v", 4)
	resp.Monthly = getChartData("month", now.AddDate(0, -12, 0), now, "%Y-%m", 12)

	c.JSON(http.StatusOK, resp)
}

// getChartData mengambil data user_activities per bucket lalu mengisi seluruh
// bucket (termasuk yang kosong dengan nilai 0) agar chart selalu berbentuk
// deret penuh — sama seperti panel lama.
func getChartData(dateFmt string, since, until time.Time, labelFmt string, buckets int) []chartDataPoint {
	rows, err := database.DB.Raw(`
		SELECT DATE_FORMAT(created_at, '`+labelFmt+`') AS label,
		       COUNT(DISTINCT user_id) AS value
		FROM user_activities
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY label
		ORDER BY label ASC
	`, since, until).Rows()
	if err != nil {
		return make([]chartDataPoint, 0)
	}
	defer rows.Close()

	valueMap := make(map[string]int64)
	for rows.Next() {
		var dp chartDataPoint
		rows.Scan(&dp.Label, &dp.Value)
		valueMap[dp.Label] = dp.Value
	}

	return fillChartSeries(dateFmt, until, buckets, valueMap)
}

// fillChartSeries membangun deret bucket berurutan dari yang paling lama ke
// yang terbaru, mengisi nilai 0 untuk bucket tanpa data.
func fillChartSeries(dateFmt string, until time.Time, buckets int, valueMap map[string]int64) []chartDataPoint {
	result := make([]chartDataPoint, 0, buckets)
	for i := buckets - 1; i >= 0; i-- {
		t := until
		switch dateFmt {
		case "hour":
			t = until.Add(-time.Duration(i) * time.Hour).Truncate(time.Hour)
		case "day":
			t = until.AddDate(0, 0, -i)
		case "week":
			t = until.AddDate(0, 0, -7*i)
		case "month":
			t = until.AddDate(0, -i, 0)
		}
		label := chartLabel(dateFmt, t)
		result = append(result, chartDataPoint{Label: label, Value: valueMap[label]})
	}
	return result
}

func chartLabel(dateFmt string, t time.Time) string {
	switch dateFmt {
	case "hour":
		return t.Format("2006-01-02 15:00")
	case "day":
		return t.Format("2006-01-02")
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-%02d", y, w)
	case "month":
		return t.Format("2006-01")
	}
	return ""
}

// getPagination membaca parameter page/per_page dengan default dan batas aman.
func getPagination(c *gin.Context, defaultPerPage int) (page, perPage int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	perPage, _ = strconv.Atoi(c.DefaultQuery("per_page", strconv.Itoa(defaultPerPage)))
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > 100 {
		perPage = 100
	}
	return
}

func pageCount(total int64, perPage int) int {
	pages := int(total) / perPage
	if int(total)%perPage != 0 {
		pages++
	}
	return pages
}

func strPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// adminPage adalah envelope pagination standar untuk list admin.
type adminPage[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalPages int   `json:"total_pages"`
}
