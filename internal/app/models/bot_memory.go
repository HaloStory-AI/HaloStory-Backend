package models

import "time"

type BotMemory struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	BotID     string    `gorm:"type:varchar(255);index;not null" json:"bot_id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Score     float64   `gorm:"default:0" json:"score"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BotMemory) TableName() string {
	return "bot_memories"
}

// CREATE TABLE bot_memories (
//   id BIGINT AUTO_INCREMENT PRIMARY KEY,
//   bot_id VARCHAR(255) NOT NULL,
//   user_id BIGINT NOT NULL,
//   content TEXT NOT NULL,
//   score FLOAT DEFAULT 0,
//   created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//   updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
//   UNIQUE KEY uniq_bot_user_content (bot_id, user_id, content(255)),
//   INDEX idx_bot_user (bot_id, user_id)
// );
