package models

import "time"

type RefreshToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	TokenHash string     `gorm:"type:char(64);uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
