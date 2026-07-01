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

type BotListResponse struct {
	BotID        string     `json:"bot_id"`
	Name         string     `json:"name"`
	CategorySlug *string    `json:"category_slug"`
	Description  *string    `json:"description"`
	Avatar       *string    `json:"avatar"`
	Birthday     *time.Time `json:"birthday"`
	Sbr          *string    `json:"sbr"`
	Color        *string    `json:"color"`
	Child        bool       `json:"child"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Category     *Category  `json:"category,omitempty"`
}

func BotToListResponse(bot *Bot) BotListResponse {
	return BotListResponse{
		BotID:        bot.BotID,
		Name:         bot.Name,
		CategorySlug: bot.CategorySlug,
		Description:  bot.Description,
		Avatar:       bot.Avatar,
		Birthday:     bot.Birthday,
		Sbr:          bot.Sbr,
		Color:        bot.Color,
		Child:        bot.Child,
		CreatedAt:    bot.CreatedAt,
		UpdatedAt:    bot.UpdatedAt,
		Category:     bot.Category,
	}
}

func BotsToListResponse(bots []Bot) []BotListResponse {
	result := make([]BotListResponse, len(bots))
	for i, bot := range bots {
		result[i] = BotToListResponse(&bot)
	}
	return result
}
