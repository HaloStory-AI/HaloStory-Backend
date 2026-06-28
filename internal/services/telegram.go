package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"rafiq8k-moga/halostory-go-backend/internal/config"
)

type TelegramService struct {
	botToken string
	chatID   string
	client   *http.Client
}

func NewTelegramService(cfg *config.Config) *TelegramService {
	return &TelegramService{
		botToken: cfg.TelegramBotToken,
		chatID:   cfg.TelegramUserID,
		client:   &http.Client{},
	}
}

type tgMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

func (s *TelegramService) send(text string) error {
	if s.botToken == "" || s.chatID == "" {
		log.Printf("[telegram dry-run] %s", text)
		return nil
	}

	payload := tgMessage{
		ChatID:    s.chatID,
		Text:      text,
		ParseMode: "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram marshal: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	resp, err := s.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram post: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (s *TelegramService) SendChildSafetyAlert(botName string, userID uint, userInput, aiResponse string) error {
	text := fmt.Sprintf(
		"<b>Child Safety Alert</b>\nBot: <code>%s</code>\nUser: #%d\n\n<b>Input:</b>\n%s\n\n<b>Response:</b>\n%s",
		botName, userID, userInput, aiResponse,
	)
	return s.send(text)
}

func (s *TelegramService) SendAnalyticsReport(title, content string) error {
	text := fmt.Sprintf("<b>%s</b>\n\n%s", title, content)
	return s.send(text)
}
