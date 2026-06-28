package cron

import (
	"log"
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/services"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

func Start(cfg *config.Config, db *gorm.DB, _ interface{}) {
	loc, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	c := cron.New(cron.WithLocation(loc))

	tg := services.NewTelegramService(cfg)

	c.AddFunc("@every 1m", func() {
		CronCheck(db)
	})

	c.AddFunc("@every 1h", func() {
		StoryFetcher(db)
	})

	c.AddFunc("@every 24h", func() {
		AnalyticsHourly(tg, db)
	})

	c.AddFunc("@weekly", func() {
		AnalyticsDaily(tg, db)
	})

	c.AddFunc("0 0 1 * *", func() {
		AnalyticsWeekly(tg, db)
	})

	c.AddFunc("0 0 1 1 *", func() {
		AnalyticsMonthly(tg, db)
	})

	c.AddFunc("@weekly", func() {
		AnalyticsCleanup(db)
	})

	c.AddFunc("@every 6h", func() {
		BondSync(db)
	})

	c.Start()
	log.Println("cron scheduler started")
	select {}
}
