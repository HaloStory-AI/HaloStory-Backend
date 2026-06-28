package models

import "time"

type DeviceBan struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	UserID      uint      `gorm:"not null;index"`
	DeviceID    string    `gorm:"type:varchar(500);not null;index"`
	DeviceModel *string   `gorm:"type:varchar(255)"`
	DeviceBrand *string   `gorm:"type:varchar(255)"`
	OsVersion   *string   `gorm:"type:varchar(100)"`
	Platform    *string   `gorm:"type:varchar(50)"`
	AppVersion  *string   `gorm:"type:varchar(50)"`
	BannedAt    *time.Time `gorm:"type:datetime"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	User *User `gorm:"foreignKey:UserID"`
}
