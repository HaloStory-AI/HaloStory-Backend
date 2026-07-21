package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

func Login(c *gin.Context) {
	allowed, retryAfter, err := services.RateLimit("auth.login:"+c.ClientIP(), 5, 15*time.Minute)
	if err == nil && !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"message":     "Too many attempts. Please try again later.",
			"retry_after": retryAfter,
		})
		return
	}

	var req LoginRequest

	// Parsing body JSON
	if err = c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	var user models.User

	// Cari user berdasarkan email
	result := database.DB.Where("email = ?", req.Email).First(&user)
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Email atau password salah",
		})
		return
	}

	// Cek kecocokan password menggunakan bcrypt (karena Laravel menggunakan bcrypt)
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Email atau password salah",
		})
		return
	}

	// Cek apakah user sedang dibanned
	if user.HasActiveBan() {
		var bannedUntilStr *string
		if user.BannedUntil != nil {
			str := user.BannedUntil.Format("2006-01-02T15:04:05Z07:00") // Format RFC3339
			bannedUntilStr = &str
		}

		c.JSON(http.StatusForbidden, gin.H{
			"message":      "Akun Anda telah dibanned.",
			"reason":       user.BanReason,
			"banned_until": bannedUntilStr,
		})
		return
	}

	// Generate JWT Token menggunakan APP_KEY dari .env
	tokenString, err := utils.GenerateJWT(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal membuat token autentikasi",
		})
		return
	}

	// Return JSON sesuai format yang diminta
	c.JSON(http.StatusOK, gin.H{
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
	DeviceID             string `json:"device_id" form:"device_id"`
}

func Register(c *gin.Context) {
	allowed, retryAfter, err := services.RateLimit("auth.register:"+c.ClientIP(), 5, 15*time.Minute)
	if err == nil && !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"message":     "Too many attempts. Please try again later.",
			"retry_after": retryAfter,
		})
		return
	}

	var req RegisterRequest

	if err = c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	if req.Password != req.PasswordConfirmation {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Password confirmation tidak cocok",
		})
		return
	}

	// Cek apakah device_id dibanned
	if req.DeviceID != "" {
		var deviceBan models.DeviceBan
		if err := database.DB.Where("device_id = ?", req.DeviceID).First(&deviceBan).Error; err == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"message": "Perangkat ini telah dibanned. Tidak dapat mendaftar.",
			})
			return
		}
	}

	// Cek apakah domain email adalah temporary/fake domain
	parts := strings.SplitN(req.Email, "@", 2)
	if len(parts) == 2 {
		domain := parts[1]
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get("https://fakefilter.net/api/is/fakedomain/" + domain)
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			var result struct {
				IsFakeDomain bool `json:"isFakeDomain"`
			}
			if json.Unmarshal(body, &result) == nil && result.IsFakeDomain {
				c.JSON(http.StatusUnprocessableEntity, gin.H{
					"success": false,
					"message": "Email sementara (temporary) tidak diizinkan. Gunakan email permanen.",
				})
				return
			}
		}
	}

	// Cek apakah email sudah ada
	var count int64
	database.DB.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"message": "Email sudah terdaftar",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal mengenkripsi password",
		})
		return
	}

	// Buat user baru (default role 'Sensei' seperti di Laravel)
	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     "Sensei",
	}

	// Simpan ke database
	if err = database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal mendaftarkan user",
		})
		return
	}

	// Kirim OTP verifikasi ke email user yang baru mendaftar (dijalankan di background)
	SendOtp(user.Email)

	// Generate JWT Token
	tokenString, err := utils.GenerateJWT(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "User berhasil dibuat, tapi gagal generate token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": tokenString,
		"token_type":   "Bearer",
		"user":         user,
	})
}
