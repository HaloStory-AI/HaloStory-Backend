package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var usernameSanitizeRe = regexp.MustCompile(`[^a-z0-9]+`)

// SlugifyName mengubah name menjadi slug lower_snake untuk username.
// Contoh: "Rafiq Azizi" -> "rafiq_azizi", "Ahmad Fauzi-123" -> "ahmad_fauzi_123"
func SlugifyName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = usernameSanitizeRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	// collapse multiple __
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	if s == "" {
		s = "user"
	}
	// batas panjang slug biar username + # + 4 digit <= 255
	if len(s) > 240 {
		s = s[:240]
		s = strings.TrimRight(s, "_")
	}
	return s
}

func random4Digit() string {
	n, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		// fallback very unlikely
		return "0000"
	}
	return fmt.Sprintf("%04d", n.Int64()+1000)
}

// GenerateUsername membuat username unik format slug#1234 via DB check.
// Retry sampai 20x, fallback tambah suffix nano jika collision tinggi.
func GenerateUsername(db *gorm.DB, name string) string {
	slug := SlugifyName(name)
	for i := 0; i < 20; i++ {
		candidate := fmt.Sprintf("%s#%s", slug, random4Digit())
		var count int64
		if err := db.Model(&struct {
			Username string `gorm:"column:username"`
		}{}).Table("users").Where("username = ?", candidate).Count(&count).Error; err == nil && count == 0 {
			return candidate
		}
	}
	// fallback: append extra random digit block jika 20x collision (slug sangat duplikat)
	return fmt.Sprintf("%s#%s%s", slug, random4Digit(), random4Digit()[:2])
}
