package auth

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ───────────────────── Forgot Password ─────────────────────

func sendPasswordResetOtp(email string) {
	rand.Seed(time.Now().UnixNano())
	otp := fmt.Sprintf("%06d", rand.Intn(1000000))

	hashedOtp, _ := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)

	now := time.Now()
	token := models.PasswordResetToken{
		Email:     email,
		Token:     string(hashedOtp),
		CreatedAt: &now,
	}
	database.DB.Where("email = ?", email).Delete(&models.PasswordResetToken{})
	database.DB.Create(&token)

	go utils.SendPasswordResetOtpEmail(email, otp)
}

func ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" form:"email"`
	}
	if err := c.ShouldBind(&req); err != nil || strings.TrimSpace(req.Email) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The email field is required.",
		})
		return
	}

	var user models.User
	if database.DB.Where("email = ?", req.Email).First(&user).Error == nil {
		sendPasswordResetOtp(req.Email)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "If the email is registered, you will receive a password reset OTP.",
	})
}

// ───────────────────── Reset Password ─────────────────────

func ResetPassword(c *gin.Context) {
	var req struct {
		Email           string `json:"email" form:"email"`
		Otp             string `json:"otp" form:"otp"`
		Password        string `json:"password" form:"password"`
		PasswordConfirm string `json:"password_confirmation" form:"password_confirmation"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Otp = strings.TrimSpace(req.Otp)

	if req.Email == "" || len(req.Otp) != 6 || req.Password == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Email, OTP (6 digit), dan password wajib diisi.",
		})
		return
	}

	if len(req.Password) < 8 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Password minimal 8 karakter.",
		})
		return
	}

	if req.Password != req.PasswordConfirm {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Password confirmation tidak cocok.",
		})
		return
	}

	var token models.PasswordResetToken
	if err := database.DB.Where("email = ?", req.Email).First(&token).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid or expired OTP.",
		})
		return
	}

	if token.CreatedAt != nil && time.Since(*token.CreatedAt) > 15*time.Minute {
		database.DB.Where("email = ?", req.Email).Delete(&models.PasswordResetToken{})
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "OTP has expired. Please request a new one.",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(token.Token), []byte(req.Otp)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid OTP.",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal mengenkripsi password.",
		})
		return
	}

	database.DB.Model(&models.User{}).Where("email = ?", req.Email).Update("password", string(hashedPassword))
	database.DB.Where("email = ?", req.Email).Delete(&models.PasswordResetToken{})

	c.JSON(http.StatusOK, gin.H{
		"message": "Password has been reset successfully.",
	})
}

// ───────────────────── Update Email ─────────────────────

func UpdateEmail(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email"`
		Password string `json:"password" form:"password"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Email dan password wajib diisi.",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Invalid password",
		})
		return
	}

	var count int64
	database.DB.Model(&models.User{}).Where("email = ? AND id != ?", req.Email, user.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"message": "Email sudah terdaftar.",
		})
		return
	}

	database.DB.Model(&user).Update("email", req.Email)

	c.JSON(http.StatusOK, gin.H{
		"message": "Email updated successfully",
	})
}

// ───────────────────── Update Name ─────────────────────

func UpdateName(c *gin.Context) {
	var req struct {
		Name string `json:"name" form:"name"`
	}
	if err := c.ShouldBind(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Nama wajib diisi.",
		})
		return
	}

	if len(req.Name) > 255 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Nama maksimal 255 karakter.",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	database.DB.Model(&user).Update("name", req.Name)

	c.JSON(http.StatusOK, gin.H{
		"message": "Name updated successfully",
	})
}

// ───────────────────── Update Password ─────────────────────

func UpdatePassword(c *gin.Context) {
	var req struct {
		CurrentPassword    string `json:"current_password" form:"current_password"`
		Password           string `json:"password" form:"password"`
		PasswordConfirm    string `json:"password_confirmation" form:"password_confirmation"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	if req.CurrentPassword == "" || req.Password == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Current password dan password baru wajib diisi.",
		})
		return
	}

	if len(req.Password) < 8 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Password minimal 8 karakter.",
		})
		return
	}

	if req.Password != req.PasswordConfirm {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Password confirmation tidak cocok.",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Current password is incorrect.",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal mengenkripsi password.",
		})
		return
	}

	database.DB.Model(&user).Update("password", string(hashedPassword))

	c.JSON(http.StatusOK, gin.H{
		"message": "Password updated successfully",
	})
}

// ───────────────────── Update Persona ─────────────────────

func UpdatePersona(c *gin.Context) {
	var req struct {
		Persona string `json:"persona" form:"persona"`
	}
	if err := c.ShouldBind(&req); err != nil || strings.TrimSpace(req.Persona) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Persona wajib diisi.",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	database.DB.Model(&user).Update("persona", req.Persona)

	c.JSON(http.StatusOK, gin.H{
		"message": "Persona updated successfully",
	})
}

// ───────────────────── Report Banned Device ─────────────────────

func ReportBannedDevice(c *gin.Context) {
	var req struct {
		DeviceID    string  `json:"device_id" form:"device_id"`
		DeviceModel *string `json:"device_model" form:"device_model"`
		DeviceBrand *string `json:"device_brand" form:"device_brand"`
		OsVersion   *string `json:"os_version" form:"os_version"`
		Platform    *string `json:"platform" form:"platform"`
		AppVersion  *string `json:"app_version" form:"app_version"`
	}
	if err := c.ShouldBind(&req); err != nil || strings.TrimSpace(req.DeviceID) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Device ID wajib diisi.",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)
	now := time.Now()

	ban := models.DeviceBan{
		UserID:      &user.ID,
		DeviceID:    req.DeviceID,
		DeviceModel: req.DeviceModel,
		DeviceBrand: req.DeviceBrand,
		OsVersion:   req.OsVersion,
		Platform:    req.Platform,
		AppVersion:  req.AppVersion,
		BannedAt:    &now,
	}

	var existing models.DeviceBan
	result := database.DB.Where("device_id = ?", req.DeviceID).First(&existing)
	if result.Error == nil {
		ban.ID = existing.ID
		ban.CreatedAt = existing.CreatedAt
		database.DB.Save(&ban)
	} else {
		database.DB.Create(&ban)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Device reported.",
	})
}
