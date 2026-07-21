package models

import (
	"time"
)

type EmailVerificationOtp struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Email     string     `gorm:"index" json:"email"`
	Otp       string     `json:"-"` // Hashed OTP
	CreatedAt *time.Time `json:"created_at"`
}
