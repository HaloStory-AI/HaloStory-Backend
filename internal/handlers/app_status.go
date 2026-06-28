package handlers

import (
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func GetAppStatus(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var settings []models.AppSetting
		db.Find(&settings)

		settingMap := make(map[string]string)
		for _, s := range settings {
			settingMap[s.Key] = s.Value
		}

		version := settingMap["app_version"]
		if version == "" {
			version = "0.0.0"
		}

		status := "active"
		maintenanceMsg := "HaloStory is under maintenance."
		if settingMap["is_maintenance"] == "true" {
			status = "maintenance"
			if m := settingMap["maintenance_message"]; m != "" {
				maintenanceMsg = m
			}
		}

		return c.JSON(fiber.Map{
			"version":            version,
			"api_url":            c.BaseURL() + "/api",
			"status":             status,
			"maintenance_message": maintenanceMsg,
		})
	}
}
