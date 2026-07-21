package models

import "time"

type Bot struct {
	BotID         string     `gorm:"primaryKey;type:varchar(255)" json:"id"` // Menggunakan "id" pada json agar sesuai dengan format frontend
	Name          string     `json:"name"`
	CategorySlug  *string    `json:"category_slug"`
	Description   *string    `json:"description"`
	Avatar        *string    `json:"avatar"`
	Jailbreak     *string    `json:"background"` // Frontend map jailbreak ke background
	Personality   *string    `json:"personality"`
	ChatExamples  *string    `json:"chat_examples"`
	Birthday      *time.Time `json:"birthday"`
	Sbr           *string    `json:"sbr"`
	Color         *string    `json:"color"`
	Child         bool       `json:"child"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName overrides the table name used by User to `bots`
func (Bot) TableName() string {
	return "bots"
}
