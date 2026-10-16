package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ───────────────────── Forgot Password ─────────────────────

func sendPasswordResetOtp(email string) {
	otp := generateOTP()

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
	allowed, retryAfter, err := services.RateLimit("auth.forgot-password:"+c.ClientIP(), 5, 15*time.Minute)
	if err == nil && !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"message":     "Too many attempts. Please try again later.",
			"retry_after": retryAfter,
		})
		return
	}

	var req struct {
		Email string `json:"email" form:"email" binding:"required,email,max=255"`
	}
	if err := c.ShouldBind(&req); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
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
		Email           string `json:"email" form:"email" binding:"required,email,max=255"`
		Otp             string `json:"otp" form:"otp" binding:"required,len=6"`
		Password        string `json:"password" form:"password" binding:"required,min=8"`
		PasswordConfirm string `json:"password_confirmation" form:"password_confirmation"`
	}
	if err := c.ShouldBind(&req); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
		return
	}

	req.Otp = strings.TrimSpace(req.Otp)

	if req.Password != req.PasswordConfirm {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The password confirmation does not match.",
			"errors": gin.H{
				"password": []string{"The password confirmation does not match."},
			},
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

// ───────────────────── Update Picture ─────────────────────

func UpdatePicture(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email" binding:"required,email,max=255"`
		Password string `json:"password" form:"password" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The email field is required.",
			"errors": gin.H{
				"email": []string{"The email field is required."},
			},
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
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The email has already been taken.",
			"errors": gin.H{
				"email": []string{"The email has already been taken."},
			},
		})
		return
	}

	database.DB.Model(&user).Update("email", req.Email)

	c.JSON(http.StatusOK, gin.H{
		"message": "Email updated successfully",
	})
}

// ───────────────────── Update Email ─────────────────────

func UpdateEmail(c *gin.Context) {
	var req struct {
		Email    string `json:"email" form:"email" binding:"required,email,max=255"`
		Password string `json:"password" form:"password"`
	}
	if err := c.ShouldBind(&req); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The email field is required.",
			"errors": gin.H{
				"email": []string{"The email field is required."},
			},
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	var count int64
	database.DB.Model(&models.User{}).Where("email = ? AND id != ?", req.Email, user.ID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The email has already been taken.",
			"errors": gin.H{
				"email": []string{"The email has already been taken."},
			},
		})
		return
	}

	var userOAuth models.UserOAuth

	// Ambil data `has_local_password` default false
	err := database.DB.Where("provider = ? AND user_id = ?", "google", user.ID).Select("has_local_password", "user_id").First(&userOAuth).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// User belum pernah login google
		// log.Println("[DEBUG] User belum pernah login google")
		// Berarti update password harus ada current passwordnya, alias has_local_password true
		userOAuth.HasLocalPassword = true
	} else if err != nil {
		// Error
		log.Println("[ERROR] Error database: " + err.Error())
	} else {
		// User sudah pernah login google, maka hapus kaitannya.
		// Kalo hapus, user harus kaitkan ulang ke akun google di pengaturan aplikasi.
		// Kalo dikatikannya di screen login, maka harus login dengan email yang baru itu, jangan yang lama.
		// Karena kalo yang lama, system akan tetap login sebagai email lama.
		// Harus pilih akun google sesuai dengan akun yang ada saat ini
		database.DB.Where("user_id = ?", user.ID).Delete(&models.UserOAuth{})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil && userOAuth.HasLocalPassword {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Invalid password",
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
		Name string `json:"name" form:"name" binding:"required,max=255"`
	}
	if err := c.ShouldBind(&req); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The name field is required.",
			"errors": gin.H{
				"name": []string{"The name field is required."},
			},
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	database.DB.Model(&user).Update("name", req.Name)

	c.JSON(http.StatusOK, gin.H{
		"message": "Username updated successfully",
		"user":    user,
	})
}

// ───────────────────── Update Password ─────────────────────

func UpdatePassword(c *gin.Context) {
	var req struct {
		CurrentPassword    string `json:"current_password"`
		NewPassword        string `json:"new_password" binding:"required,min=8"`
		NewPasswordConfirm string `json:"new_password_confirmation"`
	}
	if err := c.ShouldBind(&req); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
		return
	}

	if req.NewPassword != req.NewPasswordConfirm {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The new password confirmation does not match.",
			"errors": gin.H{
				"new_password": []string{"The new password confirmation does not match."},
			},
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)
	var userOAuth models.UserOAuth

	// Ambil data `has_local_password` default false
	err := database.DB.Where("provider = ? AND user_id = ?", "google", user.ID).Select("has_local_password", "user_id").First(&userOAuth).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// User belum pernah login google
		// log.Println("[DEBUG] User belum pernah login google")
		// Berarti update password harus ada current passwordnya, alias has_local_password true
		userOAuth.HasLocalPassword = true
	} else if err != nil {
		// Error
		log.Println("[ERROR] Error database: " + err.Error())
	} else {
		// User sudah pernah login google
		// log.Println("[DEBUG] User sudah pernah login google")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil && userOAuth.HasLocalPassword {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Invalid current password",
		})
		return
	}

	if !userOAuth.HasLocalPassword {
		database.DB.Model(&models.UserOAuth{}).Where("user_id = ?", userOAuth.UserID).Update("has_local_password", true)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
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
		Persona *string `json:"persona" form:"persona" binding:"omitempty,max=500"`
	}
	if err := c.ShouldBind(&req); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	database.DB.Model(&user).Update("persona", req.Persona)

	c.JSON(http.StatusOK, gin.H{
		"message": "Persona updated successfully",
		"user":    user,
	})
}

func CheckHasLocalPassword(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)
	var userOAuth models.UserOAuth

	// Ambil data `has_local_password` default false
	err := database.DB.Where("provider = ? AND user_id = ?", "google", user.ID).Select("has_local_password", "user_id").First(&userOAuth).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// User belum pernah login google
		// Berarti update password harus ada current passwordnya, alias has_local_password true
		userOAuth.HasLocalPassword = true
	} else if err != nil {
		// Error
		log.Println("[ERROR] Error database: " + err.Error())
	} else {
		// User sudah pernah login google
		// log.Println("[DEBUG] User sudah pernah login google")
	}

	c.JSON(http.StatusOK, gin.H{
		"has_local_password": userOAuth.HasLocalPassword,
	})
}

func CheckIsUserOauth(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)
	var userOAuth models.UserOAuth
	var hasGoogleAuth bool;

	// Ambil data `has_local_password` default false
	err := database.DB.Where("provider = ? AND user_id = ?", "google", user.ID).Select("has_local_password", "user_id").First(&userOAuth).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// User belum pernah login google
		hasGoogleAuth = false
	} else if err != nil {
		// Error
		log.Println("[ERROR] Error database: " + err.Error())
	} else {
		// User sudah pernah login google
		hasGoogleAuth = true
	}

	c.JSON(http.StatusOK, gin.H{
		"has_google_auth": hasGoogleAuth,
	})
}