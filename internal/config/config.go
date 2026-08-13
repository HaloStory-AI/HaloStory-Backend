package config

import (
	"log"
	"os"
	"strconv"

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

// EnvInt mengambil value dari environment variable sebagai int.
// Jika kosong atau gagal diparse, mengembalikan fallback.
func EnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
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
