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
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"github.com/robfig/cron/v3"
)

var schedulerHTTP = &http.Client{Timeout: 30 * time.Second}

func StartScheduler() *cron.Cron {
	c := cron.New()

	c.AddFunc("@every 1m", unbanExpired)
	c.AddFunc("@every 1m", cleanExpiredStories)
	c.AddFunc("@every 1h", makeStories)
	c.AddFunc("0 * * * *", sendHourlyAnalytics)
	c.AddFunc("0 0 * * *", sendDailyAnalytics)
	c.AddFunc("0 0 * * 0", sendWeeklyAnalytics)
	c.AddFunc("0 0 1 * *", sendMonthlyAnalytics)
	c.AddFunc("0 0 0 * * 0", cleanupAnalytics)

	c.Start()
	log.Println("Scheduler: Started with tasks: unban-expired, clean-stories, make-stories, analytics")
	return c
}

func unbanExpired() {
	log.Println("Scheduler: Checking for expired bans...")

	result := database.DB.Exec(
		"UPDATE users SET is_banned = 0, ban_reason = NULL, banned_until = NULL WHERE is_banned = 1 AND banned_until < NOW()",
	)

	if result.Error != nil {
		log.Printf("Scheduler: Error unbanning expired users: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Scheduler: Successfully unbanned %d user(s)", result.RowsAffected)
	} else {
		log.Println("Scheduler: No expired bans found")
	}

	log.Println("Scheduler: Unban check complete!")
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

func RunUnbanExpiredOnce() {
	log.Println("Running unban-expired once...")
	unbanExpired()
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

func sendTelegramMessage(message string) {
	token := config.Env("TELEGRAM_BOT_TOKEN", "")
	chatID := config.Env("TELEGRAM_USER_ID", "")

	if token == "" || chatID == "" {
		log.Println("Analytics: Telegram credentials not configured")
		return
	}

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	resp, err := schedulerHTTP.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Analytics: Failed to send Telegram message: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Println("Analytics: Report sent to Telegram")
}

type analyticsRow struct {
	Period string `gorm:"column:period"`
	Count  int    `gorm:"column:count"`
}

func buildHourlyReport() string {
	var rows []analyticsRow
	database.DB.Raw(
		`SELECT HOUR(created_at) as period, COUNT(DISTINCT user_id) as count
		 FROM user_activities
		 WHERE created_at >= NOW() - INTERVAL 24 HOUR
		 GROUP BY HOUR(created_at)
		 ORDER BY period`,
	).Scan(&rows)

	data := make(map[int]int)
	total := 0
	for _, r := range rows {
		var h int
		fmt.Sscanf(r.Period, "%d", &h)
		data[h] = r.Count
		total += r.Count
	}

	lines := []string{"<b>📊 Analytics Report — Hourly (24h)</b>", "━━━━━━━━━━━━━━━━━━━━━━━"}
	now := time.Now()
	for i := 23; i >= 0; i-- {
		h := now.Add(-time.Duration(i) * time.Hour).Hour()
		count := data[h]
		lines = append(lines, fmt.Sprintf("%02d:00 → %d users", h, count))
	}
	lines = append(lines, "━━━━━━━━━━━━━━━━━━━━━━━")
	lines = append(lines, fmt.Sprintf("<b>Total unique: %d users</b>", total))

	return strings.Join(lines, "\n")
}

func buildDailyReport() string {
	var rows []analyticsRow
	database.DB.Raw(
		`SELECT DATE(created_at) as period, COUNT(DISTINCT user_id) as count
		 FROM user_activities
		 WHERE created_at >= CURDATE() - INTERVAL 6 DAY
		 GROUP BY DATE(created_at)
		 ORDER BY period`,
	).Scan(&rows)

	data := make(map[string]int)
	total := 0
	for _, r := range rows {
		data[r.Period] = r.Count
		total += r.Count
	}

	lines := []string{"<b>📊 Analytics Report — Daily (7 days)</b>", "━━━━━━━━━━━━━━━━━━━━━━━"}
	now := time.Now()
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		count := data[key]
		lines = append(lines, fmt.Sprintf("%s → %d users", day.Format("Mon, 02 Jan"), count))
	}
	lines = append(lines, "━━━━━━━━━━━━━━━━━━━━━━━")
	lines = append(lines, fmt.Sprintf("<b>Total unique: %d users</b>", total))

	return strings.Join(lines, "\n")
}

func buildWeeklyReport() string {
	var rows []analyticsRow
	database.DB.Raw(
		`SELECT YEARWEEK(created_at) as period, COUNT(DISTINCT user_id) as count
		 FROM user_activities
		 WHERE created_at >= CURDATE() - INTERVAL (WEEKDAY(CURDATE()) + 21) DAY
		 GROUP BY YEARWEEK(created_at)
		 ORDER BY period`,
	).Scan(&rows)

	data := make(map[string]int)
	for _, r := range rows {
		data[r.Period] = r.Count
	}

	total := 0
	lines := []string{"<b>📊 Analytics Report — Weekly (4 weeks)</b>", "━━━━━━━━━━━━━━━━━━━━━━━"}
	now := time.Now()
	for i := 3; i >= 0; i-- {
		week := now.AddDate(0, 0, -i*7)
		yw := week.Format("2006-02") // YEARWEEK format
		count := data[yw]
		if count == 0 {
			_, ywNum := week.ISOWeek()
			altKey := fmt.Sprintf("%d%02d", week.Year(), ywNum)
			count = data[altKey]
		}
		total += count
		lines = append(lines, fmt.Sprintf("Week %s, %d → %d users", week.Format("02"), week.Year(), count))
	}
	lines = append(lines, "━━━━━━━━━━━━━━━━━━━━━━━")
	lines = append(lines, fmt.Sprintf("<b>Total unique: %d users</b>", total))

	return strings.Join(lines, "\n")
}

func buildMonthlyReport() string {
	var rows []analyticsRow
	database.DB.Raw(
		`SELECT DATE_FORMAT(created_at, '%Y-%m') as period, COUNT(DISTINCT user_id) as count
		 FROM user_activities
		 WHERE created_at >= DATE_SUB(CURDATE(), INTERVAL 11 MONTH) - INTERVAL (DAY(CURDATE()) - 1) DAY
		 GROUP BY DATE_FORMAT(created_at, '%Y-%m')
		 ORDER BY period`,
	).Scan(&rows)

	data := make(map[string]int)
	total := 0
	for _, r := range rows {
		data[r.Period] = r.Count
		total += r.Count
	}

	lines := []string{"<b>📊 Analytics Report — Monthly (12 months)</b>", "━━━━━━━━━━━━━━━━━━━━━━━"}
	now := time.Now()
	for i := 11; i >= 0; i-- {
		month := now.AddDate(0, -i, 0)
		key := month.Format("2006-01")
		count := data[key]
		lines = append(lines, fmt.Sprintf("%s → %d users", month.Format("Jan 2006"), count))
	}
	lines = append(lines, "━━━━━━━━━━━━━━━━━━━━━━━")
	lines = append(lines, fmt.Sprintf("<b>Total unique: %d users</b>", total))

	return strings.Join(lines, "\n")
}

func sendHourlyAnalytics() {
	log.Println("Analytics: Sending hourly report...")
	msg := buildHourlyReport()
	sendTelegramMessage(msg)
}

func sendDailyAnalytics() {
	log.Println("Analytics: Sending daily report...")
	msg := buildDailyReport()
	sendTelegramMessage(msg)
}

func sendWeeklyAnalytics() {
	log.Println("Analytics: Sending weekly report...")
	msg := buildWeeklyReport()
	sendTelegramMessage(msg)
}

func sendMonthlyAnalytics() {
	log.Println("Analytics: Sending monthly report...")
	msg := buildMonthlyReport()
	sendTelegramMessage(msg)
}

func cleanupAnalytics() {
	log.Println("Analytics: Cleaning up old activity records...")

	result := database.DB.Exec(
		"DELETE FROM user_activities WHERE created_at < NOW() - INTERVAL 365 DAY",
	)

	if result.Error != nil {
		log.Printf("Analytics: Error cleaning up: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Analytics: Deleted %d old activity records", result.RowsAffected)
	} else {
		log.Println("Analytics: No old activities found")
	}
}
