package cron

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/models"

	"gorm.io/gorm"
)

type safebooruPost struct {
	ID      int    `json:"id"`
	Tags    string `json:"tags"`
	FileURL string `json:"file_url"`
	Rating  string `json:"rating"`
}

func StoryFetcher(db *gorm.DB) {
	var bots []models.Bot
	if err := db.Where("sbr IS NOT NULL AND sbr != ''").Find(&bots).Error; err != nil {
		log.Printf("make:stories error fetching bots: %v", err)
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, bot := range bots {
		if bot.Sbr == nil || *bot.Sbr == "" {
			continue
		}

		for attempt := 0; attempt < 3; attempt++ {
			imageURL, rating, err := fetchSafebooruImage(client, *bot.Sbr)
			if err != nil {
				log.Printf("make:stories safebooru fetch error for %s: %v", bot.BotID, err)
				time.Sleep(2 * time.Second)
				continue
			}

			litterboxURL, err := uploadToLitterbox(client, imageURL)
			if err != nil {
				log.Printf("make:stories litterbox upload error for %s: %v", bot.BotID, err)
				time.Sleep(2 * time.Second)
				continue
			}

			story := models.CharacterStory{
				BotID:     bot.BotID,
				BotName:   bot.Name,
				ImagePath: litterboxURL,
				Rating:    &rating,
			}

			if err := db.Create(&story).Error; err != nil {
				log.Printf("make:stories save error for %s: %v", bot.BotID, err)
			} else {
				log.Printf("make:stories created story for %s (%s)", bot.BotID, litterboxURL)
			}
			break
		}

		time.Sleep(2 * time.Second)
	}
}

func fetchSafebooruImage(client *http.Client, tags string) (string, string, error) {
	apiURL := fmt.Sprintf("https://safebooru.org/index.php?page=dapi&s=post&q=index&tags=%s&limit=100&json=1", url.QueryEscape(tags))

	resp, err := client.Get(apiURL)
	if err != nil {
		return "", "", fmt.Errorf("safebooru request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("safebooru read: %w", err)
	}

	var posts []safebooruPost
	if err := json.Unmarshal(body, &posts); err != nil {
		return "", "", fmt.Errorf("safebooru parse: %w", err)
	}

	if len(posts) == 0 {
		return "", "", fmt.Errorf("no posts found for tags: %s", tags)
	}

	post := posts[rand.Intn(len(posts))]
	return post.FileURL, post.Rating, nil
}

func uploadToLitterbox(client *http.Client, imageURL string) (string, error) {
	form := url.Values{
		"reqtype": {"urlupload"},
		"url":     {imageURL},
		"time":    {"72h"},
	}

	resp, err := client.Post("https://litterbox.catbox.moe/resources/internals/api.php",
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("litterbox request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("litterbox read: %w", err)
	}

	result := strings.TrimSpace(string(body))
	if !strings.HasPrefix(result, "https://") {
		return "", fmt.Errorf("litterbox unexpected response: %s", result)
	}

	return result, nil
}
