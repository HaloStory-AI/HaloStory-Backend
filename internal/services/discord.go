package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"rafiq8k-moga/halostory-go-backend/internal/config"
)

type DiscordService struct {
	webhookURL       string
	childSafetyURL   string
	client           *http.Client
}

func NewDiscordService(cfg *config.Config) *DiscordService {
	return &DiscordService{
		webhookURL:     cfg.DiscordWebhookURL,
		childSafetyURL: cfg.DiscordChildSafetyWebhook,
		client:         &http.Client{},
	}
}

type discordEmbed struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Color       int               `json:"color,omitempty"`
	Fields      []discordField    `json:"fields,omitempty"`
	Footer      *discordFooter    `json:"footer,omitempty"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type discordFooter struct {
	Text string `json:"text"`
}

type discordPayload struct {
	Content string         `json:"content,omitempty"`
	Embeds  []discordEmbed `json:"embeds,omitempty"`
}

func (s *DiscordService) sendWebhook(url string, payload discordPayload) error {
	if url == "" {
		log.Printf("[discord dry-run] %+v", payload)
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("discord marshal: %w", err)
	}

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord post: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *DiscordService) SendStudentRequest(userName, studentName string) error {
	return s.sendWebhook(s.webhookURL, discordPayload{
		Embeds: []discordEmbed{{
			Title:       "New Student Request",
			Description: fmt.Sprintf("**%s** requested to adopt **%s**", userName, studentName),
			Color:       0x3498db,
		}},
	})
}

func (s *DiscordService) SendRequestApproved(userName, studentName string) error {
	return s.sendWebhook(s.webhookURL, discordPayload{
		Content: "@everyone",
		Embeds: []discordEmbed{{
			Title:       "Request Approved",
			Description: fmt.Sprintf("**%s**'s request for **%s** has been approved!", userName, studentName),
			Color:       0x2ecc71,
		}},
	})
}

func (s *DiscordService) SendRequestRejected(userName, studentName string) error {
	return s.sendWebhook(s.webhookURL, discordPayload{
		Embeds: []discordEmbed{{
			Title:       "Request Rejected",
			Description: fmt.Sprintf("**%s**'s request for **%s** was rejected.", userName, studentName),
			Color:       0xe74c3c,
		}},
	})
}

func (s *DiscordService) SendChildSafetyAlert(botName string, userID uint, userInput, aiResponse, status string) error {
	url := s.childSafetyURL
	if url == "" {
		url = s.webhookURL
	}

	truncate := func(s string, n int) string {
		if len(s) > n {
			return s[:n] + "..."
		}
		return s
	}

	color := 0xe67e22
	if status == "bypassed" {
		color = 0xe74c3c
	}

	return s.sendWebhook(url, discordPayload{
		Embeds: []discordEmbed{{
			Title: fmt.Sprintf("Child Safety Alert (%s)", status),
			Description: fmt.Sprintf("Bot **%s** triggered child safety with user **#%d**", botName, userID),
			Color: color,
			Fields: []discordField{
				{Name: "User Input", Value: truncate(userInput, 500), Inline: false},
				{Name: "AI Response", Value: truncate(aiResponse, 500), Inline: false},
			},
			Footer: &discordFooter{Text: strings.ToUpper(status)},
		}},
	})
}
