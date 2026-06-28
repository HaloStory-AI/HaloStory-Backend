package models

import (
	"time"
)

type UserBackup struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	UserID    uint      `gorm:"not null;index;unique"`
	Data      string    `gorm:"type:longtext;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time

	User *User `gorm:"foreignKey:UserID"`
}
