package models

import "time"

type UserActivity struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	UserID    uint      `gorm:"not null;index"`
	CreatedAt time.Time
	UpdatedAt time.Time

	User *User `gorm:"foreignKey:UserID"`
}
