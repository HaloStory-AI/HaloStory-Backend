package cron

import (
	"log"

	"rafiq8k-moga/halostory-go-backend/internal/models"

	"gorm.io/gorm"
)

func BondSync(db *gorm.DB) {
	var bonds []models.UserBotBond
	if err := db.Where("bond_status IS NULL OR bond_status = ''").Find(&bonds).Error; err != nil {
		log.Printf("bond:sync error: %v", err)
		return
	}

	normal := models.BondNormal
	for _, b := range bonds {
		db.Model(&b).Update("bond_status", normal)
	}

	if len(bonds) > 0 {
		log.Printf("bond:sync updated %d bonds", len(bonds))
	}
}
