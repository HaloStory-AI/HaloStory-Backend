package models

import (
	"time"
)

type UserOAuth struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `gorm:"not null" json:"user_id"`
	Provider         string    `gorm:"size:50;not null;default:'google'" json:"provider"`
	ProviderUserID   string    `gorm:"size:255;not null" json:"provider_user_id"`
	RefreshToken     string    `gorm:"type:text" json:"refresh_token"`
	HasLocalPassword bool      `gorm:"type:boolean;not null;default:false" json:"has_local_password"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (UserOAuth) TableName() string {
	return "user_oauths"
}
