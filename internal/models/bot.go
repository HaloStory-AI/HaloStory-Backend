package models

import (
	"time"
)

type Bot struct {
	BotID        string     `gorm:"type:varchar(255);primaryKey"`
	Name         string     `gorm:"type:varchar(255);not null"`
	CategorySlug *string    `gorm:"type:varchar(255)"`
	Description  *string    `gorm:"type:text"`
	Avatar       *string    `gorm:"type:varchar(255)"`
	Jailbreak    *string    `gorm:"type:text"`
	Personality  *string    `gorm:"type:text"`
	ChatExamples *string    `gorm:"type:text"`
	Birthday     *time.Time `gorm:"type:date"`
	Sbr          *string    `gorm:"type:varchar(255)"`
	Color        *string    `gorm:"type:varchar(255)"`
	Child        bool       `gorm:"type:tinyint(1);default:0"`
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Category *Category `gorm:"foreignKey:CategorySlug;references:Slug"`
}
