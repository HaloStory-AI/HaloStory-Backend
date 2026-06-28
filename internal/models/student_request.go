package models

import "time"

type StudentRequest struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	UserID      uint      `gorm:"not null;index"`
	StudentName string    `gorm:"type:varchar(255);not null"`
	Status      *string   `gorm:"type:varchar(255)"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	User *User `gorm:"foreignKey:UserID"`
}
