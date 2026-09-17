package api

import (
	"net/http"

	"halostory_go_backend/internal/database"

	"github.com/gin-gonic/gin"
)

func AdminGetActivities(c *gin.Context) {
	page, perPage := getPagination(c, 20)
	qType := c.Query("type")
	botID := c.Query("bot_id")
	userID := c.Query("user_id")

	// Count
	countQ := database.DB.Table("user_activities")
	listQ := database.DB.Table("user_activities").Order("created_at DESC").Offset((page-1)*perPage).Limit(perPage)
	if qType != "" {
		countQ = countQ.Where("activity_type = ?", qType)
		listQ = listQ.Where("activity_type = ?", qType)
	}
	if botID != "" {
		countQ = countQ.Where("bot_id = ?", botID)
		listQ = listQ.Where("bot_id = ?", botID)
	}
	if userID != "" {
		countQ = countQ.Where("user_id = ?", userID)
		listQ = listQ.Where("user_id = ?", userID)
	}
	var total int64
	countQ.Count(&total)

	type row struct {
		ID           uint   `json:"id"`
		UserID       uint   `json:"user_id"`
		ActivityType string `json:"activity_type"`
		BotID        *string `json:"bot_id"`
		Provider     *string `json:"provider"`
		CreatedAt    string `json:"created_at"`
	}
	var rows []row
	listQ.Select("id, user_id, activity_type, bot_id, provider, created_at").Scan(&rows)

	c.JSON(http.StatusOK, gin.H{
		"items": rows,
		"total": total,
		"page": page,
		"per_page": perPage,
		"total_pages": pageCount(total, perPage),
	})
}

func AdminGetActivitiesSummary(c *gin.Context) {
	groupBy := c.DefaultQuery("group_by", "type") // type|bot|provider
	from := c.Query("from")
	to := c.Query("to")

	var selectExpr, groupExpr string
	switch groupBy {
	case "bot":
		selectExpr = "COALESCE(bot_id,'-') as k, COUNT(*) as cnt, COUNT(DISTINCT user_id) as distinct_cnt"
		groupExpr = "bot_id"
	case "provider":
		selectExpr = "COALESCE(provider,'-') as k, COUNT(*) as cnt, COUNT(DISTINCT user_id) as distinct_cnt"
		groupExpr = "provider"
	default:
		selectExpr = "activity_type as k, COUNT(*) as cnt, COUNT(DISTINCT user_id) as distinct_cnt"
		groupExpr = "activity_type"
	}
	q := database.DB.Table("user_activities").Select(selectExpr)
	if from != "" {
		q = q.Where("created_at >= ?", from)
	}
	if to != "" {
		q = q.Where("created_at <= ?", to)
	}
	type agg struct {
		K            string `json:"k"`
		Cnt          int64  `json:"cnt"`
		DistinctCnt  int64  `json:"distinct_cnt"`
	}
	var out []agg
	q.Group(groupExpr).Scan(&out)
	c.JSON(http.StatusOK, gin.H{"items": out, "group_by": groupBy})
}
