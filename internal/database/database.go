package database

import (
	"fmt"
	"log"

	"halostory_go_backend/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// DB adalah instance GORM global yang bisa dipakai di mana saja (mirip Facade DB di Laravel)
var DB *gorm.DB

func Connect() {
	// Format DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Env("DB_USERNAME", "root"),
		config.Env("DB_PASSWORD", ""),
		config.Env("DB_HOST", "127.0.0.1"),
		config.Env("DB_PORT", "3306"),
		config.Env("DB_DATABASE", "halostory"),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		// Menyesuaikan konvensi nama tabel agar mirip Laravel (plural: users, bots)
		NamingStrategy: schema.NamingStrategy{
			SingularTable: false,
		},
	})

	if err != nil {
		log.Fatal("Gagal koneksi ke database: ", err)
	}

	DB = db
	log.Println("Berhasil terhubung ke database MySQL!")
}
