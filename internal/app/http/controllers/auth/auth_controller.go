package auth

import (
	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/utils"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

func Login(c *fiber.Ctx) error {
	var req LoginRequest

	// Parsing body JSON
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Format request tidak valid",
		})
	}

	var user models.User

	// Cari user berdasarkan email
	result := database.DB.Where("email = ?", req.Email).First(&user)
	if result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Email atau password salah",
		})
	}

	// Cek kecocokan password menggunakan bcrypt (karena Laravel menggunakan bcrypt)
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Email atau password salah",
		})
	}

	// Cek apakah user sedang dibanned
	if user.HasActiveBan() {
		var bannedUntilStr *string
		if user.BannedUntil != nil {
			str := user.BannedUntil.Format("2006-01-02T15:04:05Z07:00") // Format RFC3339
			bannedUntilStr = &str
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"message":      "Akun Anda telah dibanned.",
			"reason":       user.BanReason,
			"banned_until": bannedUntilStr,
		})
	}

	// Generate JWT Token menggunakan APP_KEY dari .env
	tokenString, err := utils.GenerateJWT(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Gagal membuat token autentikasi",
		})
	}

	// Return JSON sesuai format yang diminta
	return c.JSON(fiber.Map{
		"access_token": tokenString,
		"token_type":   "Bearer",
		"user":         user,
	})
}

type RegisterRequest struct {
	Name                 string `json:"name" form:"name"`
	Email                string `json:"email" form:"email"`
	Password             string `json:"password" form:"password"`
	PasswordConfirmation string `json:"password_confirmation" form:"password_confirmation"`
}

func Register(c *fiber.Ctx) error {
	var req RegisterRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Format request tidak valid",
		})
	}

	if req.Password != req.PasswordConfirmation {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Password confirmation tidak cocok",
		})
	}

	// Cek apakah email sudah ada
	var count int64
	database.DB.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "Email sudah terdaftar",
		})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Gagal mengenkripsi password",
		})
	}

	// Buat user baru (default role 'Sensei' seperti di Laravel)
	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     "Sensei",
	}

	// Simpan ke database
	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Gagal mendaftarkan user",
		})
	}

	// Kirim OTP verifikasi ke email user yang baru mendaftar (dijalankan di background)
	SendOtp(user.Email)

	// Generate JWT Token
	tokenString, err := utils.GenerateJWT(user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "User berhasil dibuat, tapi gagal generate token",
		})
	}

	return c.JSON(fiber.Map{
		"access_token": tokenString,
		"token_type":   "Bearer",
		"user":         user,
	})
}
