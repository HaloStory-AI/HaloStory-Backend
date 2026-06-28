package services

import (
	"rafiq8k-moga/halostory-go-backend/internal/config"
	"rafiq8k-moga/halostory-go-backend/internal/models"
)

type ChildSafetyService struct {
	discord  *DiscordService
	telegram *TelegramService
}

func NewChildSafetyService(cfg *config.Config) *ChildSafetyService {
	return &ChildSafetyService{
		discord:  NewDiscordService(cfg),
		telegram: NewTelegramService(cfg),
	}
}

func (s *ChildSafetyService) Check(bot *models.Bot, userID uint, userInput, aiResponse string, tags []string) {
	if bot == nil || !bot.Child {
		return
	}

	for _, tag := range tags {
		switch tag {
		case "failed_nsfw":
			s.discord.SendChildSafetyAlert(bot.Name, userID, userInput, aiResponse, "blocked")
			s.telegram.SendChildSafetyAlert(bot.Name, userID, userInput, aiResponse)
		case "nsfw_bypassed":
			s.discord.SendChildSafetyAlert(bot.Name, userID, userInput, aiResponse, "bypassed")
			s.telegram.SendChildSafetyAlert(bot.Name, userID, userInput, aiResponse)
		}
	}
}
