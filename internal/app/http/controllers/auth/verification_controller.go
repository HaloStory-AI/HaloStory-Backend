package auth

import (
	"fmt"
	"math/rand"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/utils"

	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// SendOtp logic internal untuk generate & simpan otp lalu kirim email
func SendOtp(email string) {
	// Generate 6 digit angka
	rand.Seed(time.Now().UnixNano())
	otp := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Hash OTP
	hashedOtp, _ := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)

	// Hapus record lama agar tidak menumpuk
	database.DB.Where("email = ?", email).Delete(&models.EmailVerificationOtp{})

	now := time.Now()
	otpRecord := models.EmailVerificationOtp{
		Email:     email,
		Otp:       string(hashedOtp),
		CreatedAt: &now,
	}

	database.DB.Create(&otpRecord)

	// Kirim email (dijalankan sebagai Goroutine agar tidak memblokir respon HTTP)
	go utils.SendOtpEmail(email, otp)
}

type VerifyOtpRequest struct {
	Otp string `json:"otp" form:"otp"`
}

// SendVerificationOtp untuk diakses via endpoint (minta ulang OTP)
func SendVerificationOtp(c *gin.Context) {
	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	if user.EmailVerifiedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Email already verified.",
		})
		return
	}

	SendOtp(user.Email)

	c.JSON(http.StatusOK, gin.H{
		"message": "Verification OTP sent to your email.",
	})
}

// VerifyEmail untuk memvalidasi OTP yang dimasukkan
func VerifyEmail(c *gin.Context) {
	var req VerifyOtpRequest
	if err := c.ShouldBind(&req); err != nil || len(req.Otp) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Input tidak valid. OTP harus 6 digit.",
		})
		return
	}

	userVal, _ := c.Get("user")
	user := userVal.(models.User)

	if user.EmailVerifiedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Email already verified.",
		})
		return
	}

	var otpRecord models.EmailVerificationOtp
	if err := database.DB.Where("email = ?", user.Email).First(&otpRecord).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid or expired OTP.",
		})
		return
	}

	// Cek apakah lebih dari 15 menit
	if otpRecord.CreatedAt != nil && time.Since(*otpRecord.CreatedAt) > 15*time.Minute {
		database.DB.Where("email = ?", user.Email).Delete(&models.EmailVerificationOtp{})
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "OTP has expired. Please request a new one.",
		})
		return
	}

	// Cek hash
	if err := bcrypt.CompareHashAndPassword([]byte(otpRecord.Otp), []byte(req.Otp)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid OTP.",
		})
		return
	}

	// Sukses, update user
	now := time.Now()
	user.EmailVerifiedAt = &now
	database.DB.Save(&user)

	// Hapus OTP dari tabel
	database.DB.Where("email = ?", user.Email).Delete(&models.EmailVerificationOtp{})

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified successfully.",
		"user":    user,
	})
}
