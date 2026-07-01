package auth

import (
	"fmt"
	"math/rand"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/utils"

	"github.com/gofiber/fiber/v2"
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
func SendVerificationOtp(c *fiber.Ctx) error {
	user := c.Locals("user").(models.User)

	if user.EmailVerifiedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Email already verified.",
		})
	}

	SendOtp(user.Email)

	return c.JSON(fiber.Map{
		"message": "Verification OTP sent to your email.",
	})
}

// VerifyEmail untuk memvalidasi OTP yang dimasukkan
func VerifyEmail(c *fiber.Ctx) error {
	var req VerifyOtpRequest
	if err := c.BodyParser(&req); err != nil || len(req.Otp) != 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Input tidak valid. OTP harus 6 digit.",
		})
	}

	user := c.Locals("user").(models.User)

	if user.EmailVerifiedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Email already verified.",
		})
	}

	var otpRecord models.EmailVerificationOtp
	if err := database.DB.Where("email = ?", user.Email).First(&otpRecord).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid or expired OTP.",
		})
	}

	// Cek apakah lebih dari 15 menit
	if otpRecord.CreatedAt != nil && time.Since(*otpRecord.CreatedAt) > 15*time.Minute {
		database.DB.Where("email = ?", user.Email).Delete(&models.EmailVerificationOtp{})
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "OTP has expired. Please request a new one.",
		})
	}

	// Cek hash
	if err := bcrypt.CompareHashAndPassword([]byte(otpRecord.Otp), []byte(req.Otp)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid OTP.",
		})
	}

	// Sukses, update user
	now := time.Now()
	user.EmailVerifiedAt = &now
	database.DB.Save(&user)

	// Hapus OTP dari tabel
	database.DB.Where("email = ?", user.Email).Delete(&models.EmailVerificationOtp{})

	return c.JSON(fiber.Map{
		"message": "Email verified successfully.",
		"user":    user,
	})
}
