package models

import "time"

// ChatJob — DB fallback untuk antrean chat saat Redis down.
// Ditulis controller via services.StoreChatRequest / EnqueueResilient,
// diklaim worker via services.ClaimDbJob, dibersihkan scheduler tiap 10 menit.
// DDL manual: migrations/001_create_chat_jobs.sql (no AutoMigrate).
type ChatJobStatus string

const (
	ChatJobQueued     ChatJobStatus = "queued"
	ChatJobProcessing ChatJobStatus = "processing"
	ChatJobCompleted  ChatJobStatus = "completed"
	ChatJobFailed     ChatJobStatus = "failed"
)

type ChatJob struct {
	UUID       string        `gorm:"primaryKey;type:char(36)" json:"uuid"`
	Payload    string        `gorm:"type:json;not null" json:"payload"`
	Status     ChatJobStatus `gorm:"type:varchar(20);not null;default:queued;index:idx_status_created,priority:1" json:"status"`
	Response   *string       `gorm:"type:json" json:"response,omitempty"`
	StatusCode int           `gorm:"not null;default:0" json:"status_code"`
	LockedBy   *string       `gorm:"type:varchar(64)" json:"locked_by,omitempty"`
	LockedAt   *time.Time    `json:"locked_at,omitempty"`
	CreatedAt  time.Time     `gorm:"index:idx_status_created,priority:2" json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

func (ChatJob) TableName() string {
	return "chat_jobs"
}
