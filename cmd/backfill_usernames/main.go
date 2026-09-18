package main

import (
	"log"

	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	config.LoadConfig()

	database.Connect()
	if database.DB == nil {
		log.Fatal("DB not connected")
	}

	// Cek kolom ada
	if !database.DB.Migrator().HasColumn("users", "username") {
		log.Fatal("kolom users.username belum ada — jalankan dulu migrations/003_add_username_avatar_to_users.sql secara MANUAL")
	}

	var users []struct {
		ID       uint   `gorm:"column:id"`
		Name     string `gorm:"column:name"`
		Username *string `gorm:"column:username"`
	}
	if err := database.DB.Table("users").Select("id, name, username").Where("username IS NULL OR username = ''").Find(&users).Error; err != nil {
		log.Fatalf("select users: %v", err)
	}

	if len(users) == 0 {
		log.Println("Tidak ada user yang perlu backfill — semua sudah punya username.")
		return
	}

	log.Printf("Backfill %d user...", len(users))
	updated, skipped := 0, 0
	for _, u := range users {
		candidate := services.GenerateUsername(database.DB, u.Name)
		if err := database.DB.Table("users").Where("id = ?", u.ID).Update("username", candidate).Error; err != nil {
			log.Printf("WARN id=%d name=%q -> %q error: %v", u.ID, u.Name, candidate, err)
			skipped++
			continue
		}
		updated++
		if updated%500 == 0 {
			log.Printf("... %d/%d", updated, len(users))
		}
	}

	var nullCount int64
	database.DB.Table("users").Where("username IS NULL OR username = ''").Count(&nullCount)
	var dupCount int64
	database.DB.Raw("SELECT COUNT(*) FROM (SELECT username, COUNT(*) c FROM users GROUP BY username HAVING c>1) t").Scan(&dupCount)

	log.Printf("Selesai. updated=%d skipped=%d remaining_null=%d duplicate_groups=%d", updated, skipped, nullCount, dupCount)
	if nullCount > 0 {
		log.Printf("Masih ada %d user tanpa username — cek manual", nullCount)
	}
	if dupCount > 0 {
		log.Printf("Ada %d grup duplicate username — perlu cek manual", dupCount)
	}
	log.Println("Jika nullCount=0 & duplicate=0, lanjutkan MANUAL: migrations/003b_make_username_notnull_unique.sql")
}
