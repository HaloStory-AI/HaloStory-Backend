package models

import (
	"time"
)

// User mewakili tabel users dari Laravel
type User struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Name            string     `json:"name"`
	Username        string     `gorm:"size:255;uniqueIndex" json:"username"`
	Avatar          *string    `gorm:"size:255" json:"avatar"`
	Persona         *string    `json:"persona"`
	// Picture         string     `json:"picture"`
	Email           string     `json:"email"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	Password        string     `json:"-"` // disembunyikan dari JSON response
	Role            string     `json:"role"`
	VipStartedAt    *time.Time `json:"vip_started_at"`
	VipExpiresAt    *time.Time `json:"vip_expires_at"`
	// Kolom ban dibiarkan di DB (tidak dipakai lagi, tidak ada gate yang menolak request).
	IsBanned    bool       `json:"is_banned"`
	BanReason   *string    `json:"ban_reason"`
	BannedUntil *time.Time `json:"banned_until"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
