package models

import "time"

type CharacterStory struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	BotID     string    `gorm:"type:varchar(255);not null;index"`
	BotName   string    `gorm:"type:varchar(255);not null"`
	ImagePath string    `gorm:"type:varchar(255);not null"`
	Rating    *string   `gorm:"type:varchar(10)"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
