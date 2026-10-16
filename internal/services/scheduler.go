package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"

	"github.com/robfig/cron/v3"
)

var schedulerHTTP = &http.Client{Timeout: 30 * time.Second}

func StartScheduler() *cron.Cron {
	c := cron.New()

	c.AddFunc("@every 1m", cleanExpiredStories)
	c.AddFunc("@every 10m", CleanExpiredChatJobs)
	c.AddFunc("@every 1h", makeStories)
	// Analytics crons disabled (authwall cleanup - user_activities removed)
	c.AddFunc("0 5 * * *", rollupDaily)

	c.Start()
	log.Println("Scheduler: Started with tasks: clean-stories, clean-chat-jobs, make-stories")
	return c
}

func rollupDaily() {
	log.Printf("Scheduler: rollupDaily skipped (analytics disabled)")
}

func cleanExpiredStories() {
	log.Println("Scheduler: Checking for expired stories...")

	result := database.DB.Exec(
		"DELETE FROM character_stories WHERE created_at < NOW() - INTERVAL 24 HOUR",
	)

	if result.Error != nil {
		log.Printf("Scheduler: Error cleaning expired stories: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Scheduler: Successfully deleted %d expired story/stories", result.RowsAffected)
	} else {
		log.Println("Scheduler: No expired stories found")
	}

	log.Println("Scheduler: Cron check complete!")
}

func RunCleanStoriesOnce() {
	log.Println("Running clean-stories once...")
	cleanExpiredStories()
}

func Sleep(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}

var allowedExtensions = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "webm": true, "mp4": true,
}

func makeStories() {
	log.Println("Scheduler: Starting story generation...")

	var bots []models.Bot
	database.DB.Where("sbr IS NOT NULL AND sbr != ''").Find(&bots)

	if len(bots) == 0 {
		log.Println("Scheduler: No bots with Safebooru tags found")
		return
	}

	for i, bot := range bots {
		log.Printf("  [%s] Processing (%d/%d)...", bot.BotID, i+1, len(bots))
		processBotStory(bot)

		if i < len(bots)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	log.Println("Scheduler: Story generation complete!")
}

func processBotStory(bot models.Bot) {
	database.DB.Exec(
		"DELETE FROM character_stories WHERE bot_id = ? AND created_at < NOW() - INTERVAL 24 HOUR",
		bot.BotID,
	)

	for attempt := 1; attempt <= 3; attempt++ {
		log.Printf("  [%s] Attempt %d/3...", bot.BotID, attempt)

		if fetchAndStore(bot) {
			return
		}

		if attempt < 3 {
			time.Sleep(3 * time.Second)
		} else {
			log.Printf("  [%s] All 3 attempts failed. Skipping.", bot.BotID)
		}
	}
}

type safebooruPost struct {
	LargeFileURL   string `json:"large_file_url"`
	FileURL        string `json:"file_url"`
	PreviewFileURL string `json:"preview_file_url"`
}

func fetchAndStore(bot models.Bot) bool {
	tags := fmt.Sprintf("%s order:random rating:g", *bot.Sbr)
	url := fmt.Sprintf("https://safebooru.donmai.us/posts.json?tags=%s&limit=1", strings.ReplaceAll(tags, " ", "+"))

	resp, err := schedulerHTTP.Get(url)
	if err != nil {
		log.Printf("  [%s] Safebooru request failed: %v", bot.BotID, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("  [%s] Safebooru returned HTTP %d", bot.BotID, resp.StatusCode)
		return false
	}

	var posts []safebooruPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		log.Printf("  [%s] Failed to decode Safebooru response: %v", bot.BotID, err)
		return false
	}

	if len(posts) == 0 {
		log.Printf("  [%s] No posts found", bot.BotID)
		return false
	}

	imageURL := posts[0].LargeFileURL
	if imageURL == "" {
		imageURL = posts[0].FileURL
	}
	if imageURL == "" {
		imageURL = posts[0].PreviewFileURL
	}
	if imageURL == "" {
		log.Printf("  [%s] Post missing image URL", bot.BotID)
		return false
	}

	parts := strings.Split(imageURL, ".")
	ext := strings.ToLower(parts[len(parts)-1])
	if !allowedExtensions[ext] {
		log.Printf("  [%s] Unsupported file type .%s", bot.BotID, ext)
		return false
	}

	imgResp, err := schedulerHTTP.Get(imageURL)
	if err != nil {
		log.Printf("  [%s] Failed to download image: %v", bot.BotID, err)
		return false
	}
	defer imgResp.Body.Close()

	imageData, err := io.ReadAll(imgResp.Body)
	if err != nil || len(imageData) == 0 {
		log.Printf("  [%s] Failed to read image data", bot.BotID)
		return false
	}

	litterboxURL, err := uploadToLitterbox(imageData, fmt.Sprintf("story_%d.%s", time.Now().UnixNano(), ext))
	if err != nil {
		log.Printf("  [%s] Litterbox upload failed: %v", bot.BotID, err)
		return false
	}

	story := models.CharacterStory{
		BotID:     bot.BotID,
		BotName:   bot.Name,
		ImagePath: litterboxURL,
		Rating:    "g",
	}
	if err := database.DB.Create(&story).Error; err != nil {
		log.Printf("  [%s] Failed to save story: %v", bot.BotID, err)
		return false
	}

	log.Printf("  [%s] Story created: %s", bot.BotID, litterboxURL)
	return true
}

func uploadToLitterbox(data []byte, filename string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	w.WriteField("reqtype", "fileupload")
	w.WriteField("time", "72h")

	fw, err := w.CreateFormFile("fileToUpload", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := fw.Write(data); err != nil {
		return "", fmt.Errorf("write form file: %w", err)
	}
	w.Close()

	resp, err := schedulerHTTP.Post("https://litterbox.catbox.moe/resources/internals/api.php", w.FormDataContentType(), &buf)
	if err != nil {
		return "", fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return strings.TrimSpace(string(body)), nil
}

// Analytics stubbed (authwall cleanup)
func sendTelegramMessage(message string) { log.Println("Analytics: disabled, skip send") }
func buildHourlyReport() string { return "" }
func buildDailyReport() string { return "" }
func buildWeeklyReport() string { return "" }
func buildMonthlyReport() string { return "" }
func sendHourlyAnalytics() {}
func sendDailyAnalytics() {}
func sendWeeklyAnalytics() {}
func sendMonthlyAnalytics() {}
func sendDailyMidnightReport() { log.Println("Analytics: disabled") }
func sendWeeklySundayReport() { log.Println("Analytics: disabled") }
func sendMonthlyFirstReport() { log.Println("Analytics: disabled") }
func sendYearlyJan1Report() { log.Println("Analytics: disabled") }
func cleanupAnalytics() { log.Println("Analytics: disabled") }



