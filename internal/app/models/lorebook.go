package models

import "time"

// Lorebook STAFF-BASED — per karakter, created_by staff/admin, bukan user.
// Guardrail: mencegah user inject jailbreak via lorebook.
type Lorebook struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	BotID          string    `gorm:"type:varchar(255);index:idx_bot_enabled,priority:1;not null" json:"bot_id"`
	Keys           string    `gorm:"type:json;not null" json:"keys"` // JSON array string e.g. ["vampire","blood moon"]
	Content        string    `gorm:"type:text;not null" json:"content"`
	Enabled        bool      `gorm:"default:1;index:idx_bot_enabled,priority:2" json:"enabled"`
	IsGlobal       bool      `gorm:"default:0" json:"is_global"`
	InsertionOrder int       `gorm:"default:0" json:"insertion_order"`
	CreatedBy      uint      `gorm:"not null;index:idx_created_by" json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (Lorebook) TableName() string {
	return "lorebooks"
}

// SQL DDL (jalankan manual, no AutoMigrate):
// CREATE TABLE lorebooks (
//   id BIGINT AUTO_INCREMENT PRIMARY KEY,
//   bot_id VARCHAR(255) NOT NULL,
//   keys JSON NOT NULL,
//   content TEXT NOT NULL,
//   enabled TINYINT DEFAULT 1,
//   is_global TINYINT DEFAULT 0,
//   insertion_order INT DEFAULT 0,
//   created_by BIGINT NOT NULL,
//   created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//   updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
//   FOREIGN KEY (bot_id) REFERENCES bots(bot_id) ON DELETE CASCADE,
//   FOREIGN KEY (created_by) REFERENCES users(id),
//   INDEX idx_bot_enabled (bot_id, enabled),
//   INDEX idx_created_by (created_by)
// );
