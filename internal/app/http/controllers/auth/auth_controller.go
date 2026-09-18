package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
	"halostory_go_backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type AUTHTYPE = int

const (
	EXCHANGE AUTHTYPE = iota
	LINK
)

type LoginRequest struct {
	Email    string `json:"email" form:"email" binding:"required,email"`
	Password string `json:"password" form:"password" binding:"required"`
}

type RegisterRequest struct {
	Name                 string `json:"name" form:"name" binding:"required,max=255"`
	Email                string `json:"email" form:"email" binding:"required,email,max=255"`
	Password             string `json:"password" form:"password" binding:"required,min=8"`
	PasswordConfirmation string `json:"password_confirmation" form:"password_confirmation"`
	DeviceID             string `json:"device_id" form:"device_id"`
}

type GAuthorizationCodeRequest struct {
	AuthorizationCode string `json:"authorization_code" form:"authorization_code" binding:"required,min=1"`
	// CodeVerifier      string `json:"code_verifier" form:"code_verifier" binding:"required,min=1"`
	DeviceID     string `json:"device_id" form:"device_id" binding:"required,min=1"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
}

func AuthorizationCode(c *gin.Context, authType AUTHTYPE) {
	allowed, retryAfter, err := services.RateLimit("auth.oauth:"+c.ClientIP(), 10, 15*time.Minute)
	if err == nil && !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"message":     "Too many attempts. Please try again later.",
			"retry_after": retryAfter,
		})
		return
	}

	var authRequest GAuthorizationCodeRequest
	if err := c.ShouldBind(&authRequest); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
		return
	}

	var token *oauth2.Token
	if authRequest.RedirectURI != "" && authRequest.ClientID != "" {
		token, err = services.ExchangeCodeCustom(
			c.Request.Context(),
			authRequest.AuthorizationCode,
			authRequest.ClientID,
			authRequest.ClientSecret,
			authRequest.RedirectURI,
		)
	} else {
		token, err = services.ExchangeCode(
			c.Request.Context(),
			authRequest.AuthorizationCode,
		)
	}

	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	identity, err := services.GetGoogleIdentity(c.Request.Context(), token)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Cek apakah device_id dibanned
	if authRequest.DeviceID != "" {
		var deviceBan models.DeviceBan
		if err := database.DB.Where("device_id = ?", authRequest.DeviceID).First(&deviceBan).Error; err == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"message": "Perangkat ini telah dibanned. Tidak dapat mendaftar.",
			})
			return
		}
	}

	if authType == LINK {
		userVal, _ := c.Get("user")
		user := userVal.(models.User)
		if user.Email != identity.Email {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Gunakan akun google yang sama dengan akun anda sekarang.",
			})
			return
		}
	}

	var userOAuth models.UserOAuth
	var user models.User
	var hasLocalPassword bool

	// Apakah user sudah pernah login google?
	err = database.DB.Where("provider = ? AND provider_user_id = ?", "google", identity.ID).First(&userOAuth).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// User belum pernah login pake google oauth
		// log.Println("[DEBUG] User belum pernah login pake google oauth")

		errUser := database.DB.Where("email = ?", identity.Email).First(&user).Error
		if errors.Is(errUser, gorm.ErrRecordNotFound) {
			// User belum pernah login gooogle dan belum pernah register biasa
			// log.Println("[DEBUG] User belum pernah login gooogle dan belum pernah register biasa")

			randomPassword, _ := utils.GenerateRandomPassword(8)
			// Hash password
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Gagal mengenkripsi password",
				})
				return
			}

			// Buat user baru dengan default role 'Sensei'
			user = models.User{
				Name:     identity.Name,
				Username: services.GenerateUsername(database.DB, identity.Name),
				Email:    identity.Email,
				// Pake password random dulu, karena login google langsung
				// Ntar bisa diganti di setting profile
				// Tanpa input password lama ( Karena dianggap belum pernah set Password )
				Password: string(hashedPassword),
				Role:     "Sensei",
				EmailVerifiedAt: func() *time.Time {
					t := time.Now()
					return &t
				}(),
			}

			// Simpan ke database
			if err = database.DB.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Gagal mendaftarkan user",
				})
				return
			}

			// User belum punya password ( Masih acak dari login google )
			// karena belum punya akun biasa
			hasLocalPassword = false
		} else if errUser != nil {
			// Error pada database
			// log.Println("[ERROR] Error: " + errUser.Error())
		} else {
			// Belum login akun google tapi pernah register biasa
			// log.Println("[DEBUG] Belum login akun google tapi pernah register biasa")

			// User sudah punya password, karena sudah punya akun biasa
			hasLocalPassword = true
		}

		storedRefreshToken := token.RefreshToken
		if token.RefreshToken != "" && authRequest.ClientID != "" {
			storedRefreshToken = fmt.Sprintf("%s|%s|%s", token.RefreshToken, authRequest.ClientID, authRequest.ClientSecret)
		}

		userOAuth = models.UserOAuth{
			UserID:           user.ID,
			Provider:         "google",
			ProviderUserID:   identity.ID,
			RefreshToken:     storedRefreshToken,
			HasLocalPassword: hasLocalPassword,
		}

		if err := database.DB.Create(&userOAuth).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Gagal mendaftarkan user",
			})
			return
		}
	} else if err != nil {
		// Error database
		log.Println("[ERROR] Error: " + err.Error())
	} else {
		// User sudah pernah login pake google oauth
		// log.Println("[DEBUG] User sudah pernah login pake google oauth")

		if token.RefreshToken != "" {
			storedRefreshToken := token.RefreshToken
			if authRequest.ClientID != "" {
				storedRefreshToken = fmt.Sprintf("%s|%s|%s", token.RefreshToken, authRequest.ClientID, authRequest.ClientSecret)
			}
			database.DB.Model(&models.UserOAuth{}).Where("provider_user_id = ?", identity.ID).Update("refresh_token", storedRefreshToken)
		}

		// Cari user berdasarkan email dan simpan di `user`
		result := database.DB.Where("email = ?", identity.Email).First(&user)
		if result.Error != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"message": "Email belum terdaftar",
			})
			return
		}
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

	tokenString, refreshToken, err := issueTokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal membuat token autentikasi",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":       tokenString,
		"refresh_token":      refreshToken,
		"token_type":         "Bearer",
		"user":               user,
		"has_local_password": true,
	})
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" form:"refresh_token" binding:"required"`
}

// Refresh menukar refresh token yang masih valid dengan pasangan access token
// + refresh token baru (rotasi). Refresh token lama langsung di-revoke.
func Refresh(c *gin.Context) {
	allowed, retryAfter, err := services.RateLimit("auth.refresh:"+c.ClientIP(), 30, 15*time.Minute)
	if err == nil && !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"message":     "Too many attempts. Please try again later.",
			"retry_after": retryAfter,
		})
		return
	}

	var req RefreshTokenRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "Refresh token wajib diisi",
		})
		return
	}

	hash := utils.HashToken(req.RefreshToken)

	var rt models.RefreshToken
	if err := database.DB.Where("token_hash = ?", hash).First(&rt).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Refresh token tidak valid",
		})
		return
	}

	if rt.RevokedAt != nil {
		// Reuse detection: token sudah di-rotate tapi dipakai lagi → kemungkinan theft.
		// Revoke semua refresh token user untuk memaksa re-login di semua device.
		log.Printf("[SECURITY] Refresh token reuse detected for user %d (token %d)", rt.UserID, rt.ID)
		nowReuse := time.Now()
		_ = database.DB.Model(&models.RefreshToken{}).
			Where("user_id = ? AND revoked_at IS NULL", rt.UserID).
			Update("revoked_at", nowReuse).Error
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Refresh token reuse detected, please login again",
		})
		return
	}

	if time.Now().After(rt.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Refresh token sudah kedaluwarsa",
		})
		return
	}

	var user models.User
	if err := database.DB.First(&user, rt.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "User tidak ditemukan",
		})
		return
	}

	if user.HasActiveBan() {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "Akun Anda telah dibanned.",
		})
		return
	}

	// Rotasi: buat pasangan token baru dulu, baru revoke token lama.
	// Kalau pembuatan token baru gagal, token lama tidak di-revoke sehingga
	// sesi user tidak hilang.
	tokenString, refreshToken, err := issueTokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal membuat token autentikasi",
		})
		return
	}

	now := time.Now()
	if err := database.DB.Model(&models.RefreshToken{}).
		Where("id = ?", rt.ID).
		Update("revoked_at", now).Error; err != nil {
		log.Println("[ERROR] Gagal revoke refresh token lama: " + err.Error())
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenString,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"user":          user,
	})
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
}

// Logout menandai refresh token sebagai revoked. Membutuhkan access token valid
// (protected), refresh token dikirim di body untuk di-revoke.
func Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Format request tidak valid",
		})
		return
	}

	if req.RefreshToken != "" {
		hash := utils.HashToken(req.RefreshToken)
		database.DB.Model(&models.RefreshToken{}).
			Where("token_hash = ?", hash).
			Update("revoked_at", time.Now())
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Berhasil keluar",
	})
}

func AuthorizationCodeExchange(c *gin.Context) {
	AuthorizationCode(c, EXCHANGE)
}

func AuthorizationCodeLink(c *gin.Context) {
	AuthorizationCode(c, LINK)
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

	if err = c.ShouldBind(&req); err != nil {
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
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
	tokenString, refreshToken, err := issueTokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Gagal membuat token autentikasi",
		})
		return
	}

	// Return JSON sesuai format yang diminta
	c.JSON(http.StatusOK, gin.H{
		"access_token":       tokenString,
		"refresh_token":      refreshToken,
		"token_type":         "Bearer",
		"user":               user,
		"has_local_password": true,
	})
	services.LogActivity(user.ID, "login", "", "", c.ClientIP(), c.GetHeader("User-Agent"), nil)
}

func formatValidationErrors(err error) (int, gin.H) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fieldMap := map[string]string{
			"Name":                 "name",
			"Email":                "email",
			"Password":             "password",
			"PasswordConfirmation": "password_confirmation",
			"DeviceID":             "device_id",
			"Otp":                  "otp",
			"PasswordConfirm":      "password_confirmation",
			"DeviceModel":          "device_model",
			"DeviceBrand":          "device_brand",
			"OsVersion":            "os_version",
			"Platform":             "platform",
			"AppVersion":           "app_version",
			"CurrentPassword":      "current_password",
			"NewPassword":          "new_password",
			"NewPasswordConfirm":   "new_password_confirmation",
			"Persona":              "persona",
			"AuthorizationCode":    "authorization_code",
			"CodeVerifier":         "code_verifier",
		}

		errs := make(map[string][]string)
		var allMsgs []string
		for _, fe := range ve {
			jsonField := fieldMap[fe.Field()]
			if jsonField == "" {
				jsonField = fe.Field()
			}

			var msg string
			switch fe.Tag() {
			case "required":
				msg = fmt.Sprintf("The %s field is required.", jsonField)
			case "email":
				msg = fmt.Sprintf("The %s field must be a valid email address.", jsonField)
			case "min":
				msg = fmt.Sprintf("The %s field must be at least %s characters.", jsonField, fe.Param())
			case "max":
				msg = fmt.Sprintf("The %s field must not exceed %s characters.", jsonField, fe.Param())
			default:
				msg = fmt.Sprintf("The %s field is invalid.", jsonField)
			}

			errs[jsonField] = append(errs[jsonField], msg)
			allMsgs = append(allMsgs, msg)
		}

		var message string
		if len(allMsgs) > 1 {
			message = fmt.Sprintf("%s (and %d more errors)", allMsgs[0], len(allMsgs)-1)
		} else if len(allMsgs) == 1 {
			message = allMsgs[0]
		} else {
			message = "Validation failed."
		}

		return http.StatusUnprocessableEntity, gin.H{
			"message": message,
			"errors":  errs,
		}
	}

	return http.StatusBadRequest, gin.H{
		"message": "Format request tidak valid",
	}
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
		status, resp := formatValidationErrors(err)
		c.JSON(status, resp)
		return
	}

	if req.Password != req.PasswordConfirmation {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The password confirmation does not match.",
			"errors": gin.H{
				"password": []string{"The password confirmation does not match."},
			},
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
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"message": "The email has already been taken.",
			"errors": gin.H{
				"email": []string{"The email has already been taken."},
			},
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
		Username: services.GenerateUsername(database.DB, req.Name),
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
	tokenString, refreshToken, err := issueTokens(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "User berhasil dibuat, tapi gagal generate token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":       tokenString,
		"refresh_token":      refreshToken,
		"token_type":         "Bearer",
		"user":               user,
		"has_local_password": true,
	})
	services.LogActivity(user.ID, "login", "", "", c.ClientIP(), c.GetHeader("User-Agent"), nil)
}
