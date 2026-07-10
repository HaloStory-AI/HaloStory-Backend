package models

import "time"

type CharacterStory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BotID     string    `json:"bot_id"`
	BotName   string    `json:"bot_name"`
	ImagePath string    `json:"image_path"`
	Rating    string    `json:"rating"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (CharacterStory) TableName() string {
	return "character_stories"
}
