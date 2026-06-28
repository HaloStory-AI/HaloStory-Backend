package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db     *gorm.DB
	cfg    *config.Config
	rdb    *redis.Client
	email  *services.EmailService
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *AuthHandler {
	return &AuthHandler{
		db:    db,
		cfg:   cfg,
		rdb:   rdb,
		email: services.NewEmailService(cfg),
	}
}

func (h *AuthHandler) enforceRateLimit(c *fiber.Ctx, action string) *fiber.Map {
	if h.rdb == nil {
		return nil
	}
	key := fmt.Sprintf("halostory:rate:%s:%s", action, c.IP())
	ctx := c.Context()

	count, err := h.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		h.rdb.Set(ctx, key, 1, 15*time.Minute)
		return nil
	}
	if err != nil {
		return nil
	}

	if count >= 5 {
		ttl, _ := h.rdb.TTL(ctx, key).Result()
		return &fiber.Map{
			"message":     "Too many attempts. Please try again later.",
			"retry_after": int(ttl.Seconds()),
		}
	}

	h.rdb.Incr(ctx, key)
	return nil
}

func generateOTP() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(999999))
	return fmt.Sprintf("%06d", n.Int64()+1)
}

// POST /register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	if resp := h.enforceRateLimit(c, "auth.register"); resp != nil {
		return c.Status(fiber.StatusTooManyRequests).JSON(resp)
	}

	var req struct {
		Name     string  `json:"name"`
		Email    string  `json:"email"`
		Password string  `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
		DeviceID *string `json:"device_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Name == "" || len(req.Name) > 255 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Name is required and must be under 255 characters"})
	}
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Valid email is required"})
	}
	if len(req.Password) < 8 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Password must be at least 8 characters"})
	}
	if req.Password != req.PasswordConfirmation {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Passwords do not match"})
	}

	var existing models.User
	if h.db.Where("email = ?", req.Email).First(&existing).Error == nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Email already taken"})
	}

	if req.DeviceID != nil && *req.DeviceID != "" {
		var banned models.DeviceBan
		if h.db.Where("device_id = ?", *req.DeviceID).First(&banned).Error == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Perangkat ini telah dibanned. Tidak dapat mendaftar."})
		}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Registration failed"})
	}

	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashed),
	}
	if err := h.db.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Registration failed"})
	}

	h.sendOTP(user.Email, "Email Verification OTP")

	token, err := middleware.GenerateJWT(h.cfg, user.ID, "user", user.IsVip(), false)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Token generation failed"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"access_token": token,
		"token_type":   "Bearer",
		"user":         user,
	})
}

// POST /login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	if resp := h.enforceRateLimit(c, "auth.login"); resp != nil {
		return c.Status(fiber.StatusTooManyRequests).JSON(resp)
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Invalid login details"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Invalid login details"})
	}

	if user.IsBanned() {
		var bannedUntil *string
		if user.BannedUntil != nil {
			s := user.BannedUntil.Format(time.RFC3339)
			bannedUntil = &s
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"message":      "Akun Anda telah dibanned.",
			"reason":       user.BanReason,
			"banned_until": bannedUntil,
		})
	}

	role := "user"
	if user.Role != nil {
		role = *user.Role
	}

	token, err := middleware.GenerateJWT(h.cfg, user.ID, role, user.IsVip(), user.EmailVerifiedAt != nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Token generation failed"})
	}

	return c.JSON(fiber.Map{
		"access_token": token,
		"token_type":   "Bearer",
		"user":         user,
	})
}

// GET /user
func (h *AuthHandler) GetUser(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "User not found"})
	}
	return c.JSON(user)
}

// POST /user/update-email
func (h *AuthHandler) UpdateEmail(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	userID := middleware.GetUserID(c)
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "User not found"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid password"})
	}

	user.Email = strings.TrimSpace(strings.ToLower(req.Email))
	h.db.Save(&user)

	return c.JSON(fiber.Map{"message": "Email updated successfully"})
}

// POST /user/update-password
func (h *AuthHandler) UpdatePassword(c *fiber.Ctx) error {
	var req struct {
		CurrentPassword         string `json:"current_password"`
		NewPassword             string `json:"new_password"`
		NewPasswordConfirmation string `json:"new_password_confirmation"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	userID := middleware.GetUserID(c)
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "User not found"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "Invalid current password"})
	}

	if len(req.NewPassword) < 8 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Password must be at least 8 characters"})
	}
	if req.NewPassword != req.NewPasswordConfirmation {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Passwords do not match"})
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	user.Password = string(hashed)
	h.db.Save(&user)

	return c.JSON(fiber.Map{"message": "Password updated successfully"})
}

// POST /user/update-name
func (h *AuthHandler) UpdateName(c *fiber.Ctx) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	userID := middleware.GetUserID(c)
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "User not found"})
	}

	user.Name = strings.TrimSpace(req.Name)
	h.db.Save(&user)

	return c.JSON(fiber.Map{"message": "Username updated successfully", "user": user})
}

// POST /user/update-persona
func (h *AuthHandler) UpdatePersona(c *fiber.Ctx) error {
	var req struct {
		Persona string `json:"persona"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	if len(req.Persona) > 500 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Persona must be under 500 characters"})
	}

	userID := middleware.GetUserID(c)
	h.db.Model(&models.User{}).Where("id = ?", userID).Update("persona", req.Persona)

	return c.JSON(fiber.Map{"message": "Persona updated successfully"})
}

// POST /forgot-password
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	if resp := h.enforceRateLimit(c, "auth.forgot-password"); resp != nil {
		return c.Status(fiber.StatusTooManyRequests).JSON(resp)
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var user models.User
	if err := h.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Email not found"})
	}

	otp := generateOTP()
	hashed, _ := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)

	if h.rdb != nil {
		ctx := c.Context()
		h.rdb.Set(ctx, fmt.Sprintf("halostory:otp:reset:%s", req.Email), string(hashed), 15*time.Minute)
	}

	h.email.SendOTP(req.Email, otp, "HaloStory AI - Password Reset OTP")

	return c.JSON(fiber.Map{"message": "OTP sent to your email successfully."})
}

// POST /reset-password
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req struct {
		Email           string `json:"email"`
		OTP             string `json:"otp"`
		Password        string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if len(req.OTP) != 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid OTP."})
	}
	if len(req.Password) < 8 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Password must be at least 8 characters"})
	}
	if req.Password != req.PasswordConfirmation {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Passwords do not match"})
	}

	if h.rdb == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "OTP verification unavailable"})
	}

	ctx := c.Context()
	key := fmt.Sprintf("halostory:otp:reset:%s", req.Email)
	stored, err := h.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid or expired OTP."})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "OTP verification failed"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(req.OTP)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid OTP."})
	}

	h.rdb.Del(ctx, key)

	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	h.db.Model(&models.User{}).Where("email = ?", req.Email).Update("password", string(hashed))

	return c.JSON(fiber.Map{"message": "Password has been reset successfully."})
}

// POST /email/verify/send
func (h *AuthHandler) SendVerificationOTP(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "User not found"})
	}

	if user.EmailVerifiedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Email already verified."})
	}

	h.sendOTP(user.Email, "Email Verification OTP")

	return c.JSON(fiber.Map{"message": "Verification OTP sent to your email."})
}

// POST /email/verify
func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error {
	var req struct {
		OTP string `json:"otp"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	userID := middleware.GetUserID(c)
	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "User not found"})
	}

	if user.EmailVerifiedAt != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Email already verified."})
	}

	if h.rdb == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "OTP verification unavailable"})
	}

	ctx := c.Context()
	key := fmt.Sprintf("halostory:otp:verify:%s", user.Email)
	stored, err := h.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid or expired OTP."})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "OTP verification failed"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(req.OTP)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid OTP."})
	}

	h.rdb.Del(ctx, key)

	now := time.Now()
	h.db.Model(&user).Update("email_verified_at", &now)

	return c.JSON(fiber.Map{"message": "Email verified successfully.", "user": user})
}

// POST /user/report-banned-device
func (h *AuthHandler) ReportBannedDevice(c *fiber.Ctx) error {
	var req struct {
		DeviceID    string  `json:"device_id"`
		DeviceModel *string `json:"device_model"`
		DeviceBrand *string `json:"device_brand"`
		OsVersion   *string `json:"os_version"`
		Platform    *string `json:"platform"`
		AppVersion  *string `json:"app_version"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request"})
	}

	userID := middleware.GetUserID(c)
	now := time.Now()

	ban := models.DeviceBan{
		UserID:      userID,
		DeviceID:    req.DeviceID,
		DeviceModel: req.DeviceModel,
		DeviceBrand: req.DeviceBrand,
		OsVersion:   req.OsVersion,
		Platform:    req.Platform,
		AppVersion:  req.AppVersion,
		BannedAt:    &now,
	}

	h.db.Where("device_id = ?", req.DeviceID).Assign(ban).FirstOrCreate(&ban)

	return c.JSON(fiber.Map{"message": "Device reported."})
}

func (h *AuthHandler) sendOTP(email, subject string) {
	otp := generateOTP()
	hashed, _ := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)

	if h.rdb != nil {
		h.rdb.Set(context.Background(), fmt.Sprintf("halostory:otp:verify:%s", email), string(hashed), 15*time.Minute)
	}

	h.email.SendOTP(email, otp, subject)
}
