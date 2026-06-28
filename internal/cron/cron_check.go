package cron

import (
	"log"
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/models"

	"gorm.io/gorm"
)

func CronCheck(db *gorm.DB) {
	result := db.Where("created_at < ?", time.Now().Add(-24*time.Hour)).Delete(&models.CharacterStory{})
	if result.Error != nil {
		log.Printf("cron:check error: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("cron:check deleted %d old stories", result.RowsAffected)
	}
}
