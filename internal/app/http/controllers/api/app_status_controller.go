package api

import (
	"net/http"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"github.com/gin-gonic/gin"
)

func AppStatus(c *gin.Context) {
	var settings []models.AppSetting
	database.DB.Find(&settings)

	settingMap := make(map[string]string)
	for _, s := range settings {
		if s.Value != nil {
			settingMap[s.Key] = *s.Value
		} else {
			settingMap[s.Key] = ""
		}
	}

	version := settingMap["app_version"]
	if version == "" {
		version = "0.0.0"
	}

	isMaintenance := settingMap["is_maintenance"] == "true"

	status := "active"
	if isMaintenance {
		status = "maintenance"
	}

	maintenanceMsg := settingMap["maintenance_message"]
	if maintenanceMsg == "" {
		maintenanceMsg = "HaloStory is currently under maintenance."
	}

	c.JSON(http.StatusOK, gin.H{
		"version":             version,
		"api_url":             config.Env("APP_URL", "http://127.0.0.1") + "/api",
		"status":              status,
		"maintenance_message": maintenanceMsg,
	})
}
