package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	*j = append((*j)[:0], bytes...)
	return nil
}

type UserBackup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex" json:"user_id"`
	Data      *JSON     `gorm:"type:longtext" json:"-"`
	ObjectKey *string   `gorm:"type:varchar(512)" json:"object_key"`
	SizeBytes *int64    `json:"size_bytes"`
	ETag      *string   `gorm:"type:varchar(64)" json:"etag"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UserBackup) TableName() string {
	return "user_backups"
}
