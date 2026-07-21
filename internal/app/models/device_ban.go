package models

import "time"

type DeviceBan struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      *uint      `gorm:"index" json:"user_id"`
	DeviceID    string     `gorm:"uniqueIndex;type:varchar(255)" json:"device_id"`
	DeviceModel *string    `json:"device_model"`
	DeviceBrand *string    `json:"device_brand"`
	OsVersion   *string    `json:"os_version"`
	Platform    *string    `json:"platform"`
	AppVersion  *string    `json:"app_version"`
	BannedAt    *time.Time `json:"banned_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (DeviceBan) TableName() string {
	return "device_bans"
}
