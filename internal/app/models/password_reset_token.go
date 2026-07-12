package models

import "time"

type PasswordResetToken struct {
	Email     string     `gorm:"primaryKey;type:varchar(255)" json:"email"`
	Token     string     `gorm:"type:varchar(255)" json:"-"`
	CreatedAt *time.Time `json:"created_at"`
}

func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}
