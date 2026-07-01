package models

import (
	"time"
)

// User mewakili tabel users dari Laravel
type User struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Name            string     `json:"name"`
	Persona         *string    `json:"persona"`
	Email           string     `json:"email"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	Password        string     `json:"-"` // disembunyikan dari JSON response
	Role            string     `json:"role"`
	VipStartedAt    *time.Time `json:"vip_started_at"`
	VipExpiresAt    *time.Time `json:"vip_expires_at"`
	IsBanned        bool       `json:"is_banned"`
	BanReason       *string    `json:"ban_reason"`
	BannedUntil     *time.Time `json:"banned_until"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
