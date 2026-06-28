package cron

import (
	"fmt"
	"log"
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/models"
	"rafiq8k-moga/halostory-go-backend/internal/services"

	"gorm.io/gorm"
)

func sendAnalyticsReport(tg *services.TelegramService, db *gorm.DB, title string, since time.Time) {
	var count int64
	db.Model(&models.UserActivity{}).Where("created_at >= ?", since).Count(&count)

	var distinctUsers int64
	db.Model(&models.UserActivity{}).Where("created_at >= ?", since).Distinct("user_id").Count(&distinctUsers)

	content := fmt.Sprintf(
		"Total aktivitas: %d\nUser aktif: %d\nPeriode: %s — %s",
		count, distinctUsers,
		since.Format("2006-01-02 15:04"),
		time.Now().Format("2006-01-02 15:04"),
	)

	if err := tg.SendAnalyticsReport(title, content); err != nil {
		log.Printf("analytics report error: %v", err)
	}
}

func AnalyticsHourly(tg *services.TelegramService, db *gorm.DB) {
	sendAnalyticsReport(tg, db, "Analytics Hourly", time.Now().Add(-1*time.Hour))
}

func AnalyticsDaily(tg *services.TelegramService, db *gorm.DB) {
	sendAnalyticsReport(tg, db, "Analytics Daily", time.Now().Add(-7*24*time.Hour))
}

func AnalyticsWeekly(tg *services.TelegramService, db *gorm.DB) {
	sendAnalyticsReport(tg, db, "Analytics Weekly", time.Now().Add(-4*7*24*time.Hour))
}

func AnalyticsMonthly(tg *services.TelegramService, db *gorm.DB) {
	sendAnalyticsReport(tg, db, "Analytics Monthly", time.Now().Add(-365*24*time.Hour))
}
