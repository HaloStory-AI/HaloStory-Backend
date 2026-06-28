package cron

import (
	"log"
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/models"

	"gorm.io/gorm"
)

func AnalyticsCleanup(db *gorm.DB) {
	result := db.Where("created_at < ?", time.Now().Add(-365*24*time.Hour)).Delete(&models.UserActivity{})
	if result.Error != nil {
		log.Printf("analytics:cleanup error: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("analytics:cleanup deleted %d old activities", result.RowsAffected)
	}
}
