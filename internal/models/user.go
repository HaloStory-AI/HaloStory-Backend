package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID              uint       `gorm:"primaryKey;autoIncrement"`
	Name            string     `gorm:"type:varchar(255);not null"`
	Email           string     `gorm:"type:varchar(255);uniqueIndex;not null"`
	EmailVerifiedAt *time.Time `gorm:"type:datetime"`
	Password        string     `gorm:"type:varchar(255);not null"`
	Role            *string    `gorm:"type:varchar(255)"`
	Persona         *string    `gorm:"type:varchar(500)"`
	VipStartedAt    *time.Time `gorm:"type:datetime"`
	VipExpiresAt    *time.Time `gorm:"type:datetime"`
	Banned          bool       `gorm:"column:is_banned;type:tinyint(1);default:0"`
	BanReason       *string    `gorm:"type:text"`
	BannedUntil     *time.Time `gorm:"type:datetime"`
	RememberToken   *string    `gorm:"type:varchar(100)"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (u *User) IsVip() bool {
	if u.Role != nil && *u.Role == "admin" {
		return true
	}
	if u.Role != nil && *u.Role == "vip" && u.VipExpiresAt != nil {
		return u.VipExpiresAt.After(time.Now())
	}
	return false
}

func (u *User) IsBanned() bool {
	if !u.Banned {
		return false
	}
	if u.BannedUntil != nil && u.BannedUntil.Before(time.Now()) {
		return false
	}
	return true
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Role == nil {
		role := "user"
		u.Role = &role
	}
	return nil
}
