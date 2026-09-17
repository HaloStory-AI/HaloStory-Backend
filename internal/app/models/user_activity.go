package models

import "time"

type UserActivity struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"index" json:"user_id"`
	ActivityType string     `gorm:"type:varchar(32);not null;default:chat;index" json:"activity_type"`
	BotID        *string    `gorm:"type:varchar(255);index" json:"bot_id"`
	Provider     *string    `gorm:"type:varchar(32)" json:"provider"`
	IP           *string    `gorm:"type:varchar(45)" json:"ip"`
	UserAgent    *string    `gorm:"type:varchar(512)" json:"user_agent"`
	Metadata     *JSON      `gorm:"type:json" json:"metadata"`
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (UserActivity) TableName() string {
	return "user_activities"
}

// ActivityDailyRollup menyimpan agregat harian per (date, activity_type, bot_id).
type ActivityDailyRollup struct {
	Date          time.Time `gorm:"type:date;primaryKey" json:"date"`
	ActivityType  string    `gorm:"type:varchar(32);primaryKey" json:"activity_type"`
	BotID         *string   `gorm:"type:varchar(255);primaryKey" json:"bot_id"`
	DistinctUsers int       `json:"distinct_users"`
	TotalEvents   int       `json:"total_events"`
}

func (ActivityDailyRollup) TableName() string {
	return "activity_daily_rollups"
}
