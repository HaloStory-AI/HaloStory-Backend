package api

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"

	"github.com/gin-gonic/gin"
)

type studentRequestItem struct {
	ID           uint   `json:"id"`
	StudentName  string `json:"student_name"`
	Status       string `json:"status"`
	UserName     string `json:"user_name"`
	CreatedAt    string `json:"created_at"`
}

type studentRequestStoreResponse struct {
	Success bool                   `json:"success"`
	Data    *models.StudentRequest `json:"data,omitempty"`
	Message string                 `json:"message,omitempty"`
}

func StudentRequestIndex(c *gin.Context) {
	var requests []models.StudentRequest
	database.DB.Order("created_at desc").Find(&requests)

	type userRow struct {
		ID   uint
		Name string
	}
	var userIDs []uint
	for _, r := range requests {
		userIDs = append(userIDs, r.UserID)
	}

	userMap := make(map[uint]string)
	if len(userIDs) > 0 {
		var users []userRow
		database.DB.Model(&models.User{}).
			Where("id IN ?", userIDs).
			Select("id, name").
			Find(&users)
		for _, u := range users {
			userMap[u.ID] = u.Name
		}
	}

	response := make([]studentRequestItem, 0)
	for _, r := range requests {
		userName := userMap[r.UserID]
		if userName == "" {
			userName = "Unknown"
		}

		response = append(response, studentRequestItem{
			ID:           r.ID,
			StudentName:  r.StudentName,
			Status:       r.Status,
			UserName:     userName,
			CreatedAt:    r.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	c.JSON(http.StatusOK, response)
}

type storeStudentRequestInput struct {
	StudentName string `json:"student_name" form:"student_name" binding:"required,max=100"`
}

func StudentRequestStore(c *gin.Context) {
	if config.Env("STUDENT_REQUESTS_OPEN", "true") != "true" {
		c.JSON(http.StatusForbidden, studentRequestStoreResponse{
			Success: false,
			Message: "Student requests are currently closed.",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var input storeStudentRequestInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The student name field is required and must not exceed 100 characters.",
			"errors": gin.H{
				"student_name": []string{"The student name field is required and must not exceed 100 characters."},
			},
		})
		return
	}

	input.StudentName = strings.TrimSpace(input.StudentName)

	allowed, retryAfter, err := services.RateLimit("student-request:user:"+fmt.Sprint(user.ID), 5, 12*time.Hour)
	if err == nil && !allowed {
		hours := int64(math.Ceil(float64(retryAfter) / 3600))
		c.JSON(http.StatusTooManyRequests, studentRequestStoreResponse{
			Success: false,
			Message: fmt.Sprintf("Kamu sudah mencapai batas maksimal 5 request. Silakan coba lagi dalam %d jam.", hours),
		})
		return
	}

	studentRequest := models.StudentRequest{
		UserID:      user.ID,
		StudentName: input.StudentName,
		Status:      "pending",
	}

	if err := database.DB.Create(&studentRequest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create student request.",
		})
		return
	}

	now := time.Now()
	studentRequest.CreatedAt = now
	studentRequest.UpdatedAt = now

	go services.SendStudentRequest(user.Name, input.StudentName)

	c.JSON(http.StatusOK, studentRequestStoreResponse{
		Success: true,
		Data:    &studentRequest,
	})
}
