package models

import "time"

type AppSetting struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Key       string    `gorm:"type:varchar(255);not null;uniqueIndex"`
	Value     string    `gorm:"type:text"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
