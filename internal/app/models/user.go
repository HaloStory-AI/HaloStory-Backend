package models

import (
	"time"
)

// User mewakili tabel users dari Laravel
type User struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Name            string     `json:"name"`
	Persona         *string    `json:"persona"`
	// Picture         string     `json:"picture"`
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

// HasActiveBan mengecek apakah user sedang dibanned secara aktif.
// Dinamakan HasActiveBan agar tidak bentrok dengan field IsBanned.
func (u *User) HasActiveBan() bool {
	if !u.IsBanned {
		return false
	}

	// Jika ada batas waktu dan sudah lewat, berarti ban sudah selesai
	if u.BannedUntil != nil && u.BannedUntil.Before(time.Now()) {
		return false
	}

	return true
}
