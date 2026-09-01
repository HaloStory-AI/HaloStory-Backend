package memory

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/database"
	"halostory_go_backend/internal/services"
)

// Cache lorebooks per bot_id 5m
var loreCache sync.Map // map[string]cachedLore

type cachedLore struct {
	entries []models.Lorebook
	expires time.Time
}

const loreCacheTTL = 5 * time.Minute

func GetLorebooks(botID string) []models.Lorebook {
	if v, ok := loreCache.Load(botID); ok {
		c := v.(cachedLore)
		if time.Now().Before(c.expires) {
			return c.entries
		}
	}
	var entries []models.Lorebook
	// hanya load enabled
	database.DB.Where("bot_id = ? AND enabled = ?", botID, true).Order("insertion_order ASC").Find(&entries)
	loreCache.Store(botID, cachedLore{entries: entries, expires: time.Now().Add(loreCacheTTL)})
	return entries
}

func InvalidateLoreCache(botID string) {
	loreCache.Delete(botID)
	// also redis cache
	services.RedisClient.Del(services.RedisCtx, "lorebooks:bot:"+botID)
}

// Scan returns matching lorebooks for given messages (keyword scan, staff-based only)
func Scan(messages []string, lorebooks []models.Lorebook) []models.Lorebook {
	if len(lorebooks) == 0 || len(messages) == 0 {
		return nil
	}
	// concat all messages lower for search
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(strings.ToLower(m))
		sb.WriteString(" ")
	}
	concat := sb.String()

	var matched []models.Lorebook
	for _, lb := range lorebooks {
		if !lb.Enabled {
			continue
		}
		if lb.IsGlobal {
			matched = append(matched, lb)
			continue
		}
		keys := parseKeys(lb.Keys)
		if len(keys) == 0 {
			continue
		}
		for _, k := range keys {
			kl := strings.ToLower(strings.TrimSpace(k))
			if kl == "" {
				continue
			}
			if strings.Contains(concat, kl) {
				matched = append(matched, lb)
				break
			}
		}
	}
	return matched
}

func parseKeys(raw string) []string {
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		return arr
	}
	// fallback comma split
	parts := strings.Split(raw, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// BuildLorebookInjection returns concatenated content for matched lorebooks
func BuildLorebookInjection(matched []models.Lorebook) string {
	if len(matched) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n<LOREBOOK>\n")
	for _, lb := range matched {
		sb.WriteString(lb.Content)
		sb.WriteString("\n")
	}
	sb.WriteString("</LOREBOOK>\n")
	return sb.String()
}
