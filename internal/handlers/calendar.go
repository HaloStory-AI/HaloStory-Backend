package handlers

import (
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/middleware"
	"rafiq8k-moga/halostory-go-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CalendarHandler struct {
	db *gorm.DB
}

func NewCalendarHandler(db *gorm.DB) *CalendarHandler {
	return &CalendarHandler{db: db}
}

func (h *CalendarHandler) Index(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	type calendarRequest struct {
		Month int `json:"month"`
		Year  int `json:"year"`
	}

	req := calendarRequest{}
	if err := c.BodyParser(&req); err != nil || req.Month < 1 || req.Month > 12 || req.Year < 2000 {
		now := time.Now()
		req.Month = int(now.Month())
		req.Year = now.Year()
	}

	firstOfMonth := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, time.UTC)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	var bonds []models.UserBotBond
	h.db.Where("user_id = ?", userID).Preload("Bot").Find(&bonds)

	type event struct {
		Date       string `json:"date"`
		BotName    string `json:"bot_name"`
		BotID      string `json:"bot_id"`
		EventType  string `json:"event_type"`
		Avatar     string `json:"avatar"`
	}

	events := make([]event, 0)

	for _, b := range bonds {
		if b.Bot == nil {
			continue
		}
		if b.Bot.Birthday != nil {
			bday := b.Bot.Birthday
			eventDate := time.Date(req.Year, time.Month(req.Month), bday.Day(), 0, 0, 0, 0, time.UTC)
			if eventDate.After(firstOfMonth.Add(-time.Hour*24)) && eventDate.Before(lastOfMonth.Add(time.Hour*24)) {
				avatar := ""
				if b.Bot.Avatar != nil {
					avatar = *b.Bot.Avatar
				}
				events = append(events, event{
					Date:      eventDate.Format("2006-01-02"),
					BotName:   b.Bot.Name,
					BotID:     b.Bot.BotID,
					EventType: "birthday",
					Avatar:    avatar,
				})
			}
		}
	}

	return c.JSON(fiber.Map{
		"month":  req.Month,
		"year":   req.Year,
		"events": events,
	})
}
