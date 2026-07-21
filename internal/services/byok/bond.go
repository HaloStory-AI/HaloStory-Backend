package byok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"
)

type BondParseResult struct {
	Tags         []string `json:"tags"`
	CleanText    string   `json:"clean_text"`
	AddExpAmount int      `json:"add_exp_amount"`
}

type BondResult struct {
	BondEvent         string `json:"bond_event"`
	BondStatus        string `json:"bond_status"`
	RelationshipLabel string `json:"relationship_label"`
	BondLevel         int    `json:"bond_level"`
}

var bondTagPatterns = map[string]*regexp.Regexp{
	"bond_damage":    regexp.MustCompile(`(?i)\[BOND_DAMAGE\]`),
	"want_lover":     regexp.MustCompile(`(?i)\[Mau Pacaran\]`),
	"want_marry":     regexp.MustCompile(`(?i)\[Mau Nikah\]`),
	"accepted_lover": regexp.MustCompile(`(?i)\[accepted\]`),
	"accepted_marry": regexp.MustCompile(`(?i)\[married\]`),
	"rejected_lover": regexp.MustCompile(`(?i)\[rejected\]`),
	"stay_lovers":    regexp.MustCompile(`(?i)\[stay_lovers\]`),
	"kesayangan":     regexp.MustCompile(`(?i)\[kesayangan\]`),
	"failed_nsfw":    regexp.MustCompile(`(?i)\[failed_nsfw\]`),
	"nsfw_bypassed":  regexp.MustCompile(`(?i)\[nsfw_bypassed\]`),
	"reconcile":      regexp.MustCompile(`(?i)\[Reconcile\]`),
	"need_help":      regexp.MustCompile(`(?i)\[Need Help\]`),
	"im_end":         regexp.MustCompile(`(?i)\[im_end\]|<\|im_end\|>|<\/im_end>|<\|eot_id\|>`),
}

var addExpPattern = regexp.MustCompile(`(?i)\[Add Exp:\s*(\d+)\s*\]`)

func ProcessAIResponse(text string) string {
	if text == "" {
		return ""
	}
	reThinkOpen := regexp.MustCompile(`(?si)<think>.*?</think>`)
	text = reThinkOpen.ReplaceAllString(text, "")
	reThinkUnclosed := regexp.MustCompile(`(?si)<think>.*$`)
	text = reThinkUnclosed.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func ParseBondTags(text string) BondParseResult {
	result := BondParseResult{
		Tags:         make([]string, 0),
		AddExpAmount: 5,
	}
	for key, pattern := range bondTagPatterns {
		if pattern.MatchString(text) {
			result.Tags = append(result.Tags, key)
		}
	}
	if matches := addExpPattern.FindStringSubmatch(text); len(matches) > 1 {
		var expAmount int
		fmt.Sscanf(matches[1], "%d", &expAmount)
		if expAmount > 0 {
			result.AddExpAmount = int(math.Max(5, math.Min(500, float64(expAmount))))
		}
	}
	clean := text
	for _, pattern := range bondTagPatterns {
		clean = pattern.ReplaceAllString(clean, "")
	}
	clean = addExpPattern.ReplaceAllString(clean, "")
	result.CleanText = strings.TrimSpace(clean)
	return result
}

func AddBondXp(botID string, parsed BondParseResult, userID uint) BondResult {
	result := BondResult{
		BondEvent:         "",
		BondStatus:        "normal",
		RelationshipLabel: "Orang Asing",
		BondLevel:         1,
	}
	if botID == "" || userID == 0 {
		return result
	}

	var bond models.UserBotBond
	err := database.DB.Where("user_id = ? AND bot_id = ?", userID, botID).First(&bond).Error
	if err != nil {
		bond = models.UserBotBond{
			UserID:     userID,
			BotID:      botID,
			Xp:         0,
			BondStatus: "normal",
		}
		database.DB.Create(&bond)
	}

	levelBefore := bond.GetLevel()

	var bot models.Bot
	database.DB.Where("bot_id = ?", botID).First(&bot)
	isChild := bot.Child

	tagSet := make(map[string]bool)
	for _, t := range parsed.Tags {
		tagSet[t] = true
	}

	switch {
	case tagSet["bond_damage"]:
		newXp := int(math.Max(0, float64(bond.Xp-100)))
		bond.Xp = newXp
		levelAfter := bond.GetLevel()
		if levelBefore-levelAfter >= 5 {
			bond.BondStatus = "enemy"
			result.BondEvent = "relationship_damaged"
		} else {
			result.BondEvent = "xp_lost"
		}
		database.DB.Save(&bond)

	case tagSet["reconcile"] && bond.CanReconcile():
		bond.ApplyReconcile()
		database.DB.Save(&bond)
		result.BondEvent = "relationship_reconciled"

	case isChild:
		switch {
		case tagSet["kesayangan"] || tagSet["accepted_lover"] || tagSet["accepted_marry"]:
			bond.ApplyChoice("beloved")
			database.DB.Save(&bond)
			result.BondEvent = "kesayangan"
		case tagSet["rejected_lover"]:
			bond.ApplyChoice("reject_love")
			database.DB.Save(&bond)
			result.BondEvent = "relationship_stuck"
		case tagSet["stay_lovers"]:
			bond.ApplyChoice("stay_lovers")
			database.DB.Save(&bond)
			result.BondEvent = "relationship_stuck"
		case tagSet["want_lover"] || tagSet["want_marry"]:
			result.BondEvent = "romance_blocked"
		case bond.CanGainXp():
			bond.Xp += parsed.AddExpAmount
			database.DB.Save(&bond)
			database.DB.First(&bond, bond.ID)
			if bond.GetLevel() > levelBefore {
				result.BondEvent = "level_up"
			}
		}

	case tagSet["accepted_lover"]:
		bond.ApplyChoice("accept_love")
		database.DB.Save(&bond)
		result.BondEvent = "relationship_upgraded"

	case tagSet["accepted_marry"]:
		bond.ApplyChoice("marry")
		database.DB.Save(&bond)
		result.BondEvent = "relationship_upgraded"

	case tagSet["rejected_lover"]:
		bond.ApplyChoice("reject_love")
		database.DB.Save(&bond)
		result.BondEvent = "relationship_stuck"

	case tagSet["stay_lovers"]:
		bond.ApplyChoice("stay_lovers")
		database.DB.Save(&bond)
		result.BondEvent = "relationship_stuck"

	case tagSet["want_lover"] && bond.HasPendingChoice():
		result.BondEvent = "confession_triggered"

	case tagSet["want_marry"] && bond.HasPendingChoice():
		result.BondEvent = "marriage_triggered"

	case bond.CanGainXp():
		bond.Xp += parsed.AddExpAmount
		database.DB.Save(&bond)
		database.DB.First(&bond, bond.ID)
		if bond.GetLevel() > levelBefore {
			result.BondEvent = "level_up"
		}
	}

	database.DB.Preload("Bot").First(&bond, bond.ID)
	result.BondStatus = bond.BondStatus
	result.RelationshipLabel = bond.GetRelationshipLabel(bond.Bot)
	result.BondLevel = bond.GetLevel()
	return result
}

func HandleChildSafety(bot *models.Bot, user *models.User, userInput, aiResponse string, tags []string) {
	if bot == nil || !bot.Child {
		return
	}
	isFailed := false
	isBypassed := false
	for _, t := range tags {
		if t == "failed_nsfw" {
			isFailed = true
		}
		if t == "nsfw_bypassed" {
			isBypassed = true
		}
	}
	if !isFailed && !isBypassed {
		return
	}
	status := "Gagal"
	if isBypassed {
		status = "Berhasil"
	}
	log.Printf("Child Safety Triggered for Bot: %s, User: %d, Status: %s", bot.BotID, user.ID, status)
	if user.Role == "admin" {
		log.Printf("Child Safety Alert suppressed for admin user %d", user.ID)
		return
	}
	sendChildSafetyToDiscord(bot, user, userInput, aiResponse, status)
}

func sendChildSafetyToDiscord(bot *models.Bot, user *models.User, userInput, aiResponse, status string) {
	webhookURL := config.Env("DISCORD_CHILD_SAFETY_WEBHOOK", "")
	if webhookURL == "" {
		return
	}
	truncatedResponse := aiResponse
	if len(aiResponse) > 256 {
		truncatedResponse = aiResponse[:256] + "..."
	}
	inputText := userInput
	if inputText == "" {
		inputText = "(No Input)"
	}

	payload := map[string]interface{}{
		"content": "SESEORANG MENCOBA NSFW ANAK KECIL!!! ",
		"allowed_mentions": map[string]interface{}{
			"parse": []interface{}{},
		},
		"embeds": []interface{}{
			map[string]interface{}{
				"author":      map[string]interface{}{"name": "MyBini Server Alert"},
				"title":       "Seseorang telah melakukan percobaan NSFW ke karakter anak kecil",
				"color":       14040640,
				"description": "Sesuai dengan peraturan terbaru pada Seluruh naungan MyBini Project, kita harus melakukan diskusi terhadap ini, apakah pantas untuk dilakukan BAN 🔨",
				"fields": []interface{}{
					map[string]interface{}{"name": "Status:", "value": status},
					map[string]interface{}{"name": "User: " + user.Name, "value": inputText, "inline": false},
					map[string]interface{}{"name": "Karakter: " + bot.Name, "value": truncatedResponse, "inline": false},
				},
				"footer": map[string]interface{}{"text": "Auto generated by MyBini System"},
			},
		},
	}

	body, _ := json.Marshal(payload)
	client := &http.Client{}
	req, _ := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send Discord Child Safety Alert: %v", err)
		return
	}
	defer resp.Body.Close()
}
