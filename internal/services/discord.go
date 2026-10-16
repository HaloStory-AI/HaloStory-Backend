package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"halostory_go_backend/internal/config"
)

var discordHTTPClient = &http.Client{Timeout: 10 * time.Second}

type discordEmbed struct {
	Author      discordAuthor `json:"author"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Color       int           `json:"color,omitempty"`
}

type discordAuthor struct {
	Name string `json:"name"`
}

type discordPayload struct {
	Content string        `json:"content"`
	Embeds  []discordEmbed `json:"embeds"`
}

func getWebhookURL() string {
	return config.Env("DISCORD_WEBHOOK_URL", "")
}

func sendDiscord(payload discordPayload) {
	webhookURL := getWebhookURL()
	if webhookURL == "" {
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Println("Discord: Gagal marshal payload:", err)
		return
	}

	resp, err := discordHTTPClient.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Println("Discord: Gagal mengirim webhook:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Discord: Webhook returned status %d", resp.StatusCode)
	}
}

func SendRequestApproved(userName, studentName string) {
	sendDiscord(discordPayload{
		Content: "Character Request Approved @everyone",
		Embeds: []discordEmbed{
			{
				Author:      discordAuthor{Name: "User = " + userName},
				Title:       "Request Approved",
				Description: fmt.Sprintf("Hei... Someone request a Character name: %s By: %s Has been approved, yay :)", studentName, userName),
				Color:       3066993,
			},
		},
	})
}

func SendRequestRejected(userName, studentName string) {
	sendDiscord(discordPayload{
		Content: "Character Request Rejected",
		Embeds: []discordEmbed{
			{
				Author:      discordAuthor{Name: "User = " + userName},
				Title:       "Request Rejected",
				Description: fmt.Sprintf("Sorry... Someone request a Character name: %s By: %s Has been Rejected. Sorry, sensei...", studentName, userName),
				Color:       15277667,
			},
		},
	})
}
