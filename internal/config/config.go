package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Env mirip dengan helper env() di Laravel. Mengambil value dari environment variable,
// jika tidak ada akan mengembalikan nilai default.
func Env(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

// LoadConfig membaca file .env dan memasukkannya ke environment variable OS
func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: File .env tidak ditemukan atau gagal dibaca. Menggunakan env bawaan OS.")
	} else {
		log.Println("Berhasil memuat konfigurasi dari .env")
	}
}
