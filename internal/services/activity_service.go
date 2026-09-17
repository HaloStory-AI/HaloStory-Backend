package services

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
)

func AnonIP(ip string) string {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return ""
	}
	if parsed.To4() != nil {
		masked := parsed.Mask(net.CIDRMask(24, 32))
		return masked.String()
	}
	masked := parsed.Mask(net.CIDRMask(48, 128))
	return masked.String()
}

func HashIP(ip string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(ip)))
	return hex.EncodeToString(h[:8])
}

// LogActivity fire-and-forget.
func LogActivity(userID uint, activityType, botID, provider, ip, userAgent string, metadata *models.JSON) {
	if userID == 0 {
		return
	}
	go func() {
		var anonIP *string
		var ua *string
		var bID *string
		var prov *string
		if ip != "" {
			v := AnonIP(ip)
			if v != "" {
				anonIP = &v
			}
		}
		if userAgent != "" {
			s := userAgent
			if len(s) > 512 {
				s = s[:512]
			}
			ua = &s
		}
		if botID != "" {
			bID = &botID
		}
		if provider != "" {
			prov = &provider
		}
		rec := models.UserActivity{
			UserID:       userID,
			ActivityType: activityType,
			BotID:        bID,
			Provider:     prov,
			IP:           anonIP,
			UserAgent:    ua,
			Metadata:     metadata,
		}
		if err := database.DB.Create(&rec).Error; err != nil {
			log.Printf("[WARN] LogActivity failed: %v", err)
		}
	}()
}

// RollupDaily mengisi activity_daily_rollups untuk tanggal tertentu.
func RollupDaily(date time.Time) error {
	day := date.Format("2006-01-02")
	// Hapus rollup hari itu dulu agar idempoten, lalu insert agregat.
	database.DB.Exec("DELETE FROM activity_daily_rollups WHERE date = ?", day)
	return database.DB.Exec(`
		INSERT INTO activity_daily_rollups (date, activity_type, bot_id, distinct_users, total_events)
		SELECT DATE(created_at) as date, COALESCE(activity_type,'chat') as activity_type, bot_id,
		       COUNT(DISTINCT user_id), COUNT(*)
		FROM user_activities
		WHERE DATE(created_at) = ?
		GROUP BY DATE(created_at), activity_type, bot_id
	`, day).Error
}

// BackfillRollups mengisi rollup untuk semua data existing (7a 365d penuh, idempoten).
func BackfillRollups() error {
	return database.DB.Exec(`
		INSERT INTO activity_daily_rollups (date, activity_type, bot_id, distinct_users, total_events)
		SELECT DATE(created_at) as date, COALESCE(activity_type,'chat') as activity_type, bot_id,
		       COUNT(DISTINCT user_id), COUNT(*)
		FROM user_activities
		GROUP BY DATE(created_at), activity_type, bot_id
		ON DUPLICATE KEY UPDATE distinct_users=VALUES(distinct_users), total_events=VALUES(total_events)
	`).Error
}
