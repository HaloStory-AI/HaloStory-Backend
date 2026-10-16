package byok

import (
	"regexp"
	"strings"
)

// Bond system dihapus total: tidak ada lagi XP/level/relationship.
// ParseBondTags dipertahankan hanya sebagai pembersih tag legacy
// (mis. sisa output model lama yang masih mengemisikan [Add Exp:...])
// agar tidak bocor ke frontend. Tidak ada efek samping DB.

type BondParseResult struct {
	Tags      []string `json:"tags"`
	CleanText string   `json:"clean_text"`
}

var bondTagPatterns = map[string]*regexp.Regexp{
	"bond_damage":    regexp.MustCompile(`(?i)\[BOND_DAMAGE\]`),
	"want_lover":     regexp.MustCompile(`(?i)\[Mau Pacaran\]`),
	"want_marry":     regexp.MustCompile(`(?i)\[Mau Nikah\]`),
	"accepted_lover": regexp.MustCompile(`(?i)\[accepted\]`),
	"accepted_marry": regexp.MustCompile(`(?i)\[married\]`),
	"rejected_lover": regexp.MustCompile(`(?i)\[rejected\]`),
	"stay_lovers":    regexp.MustCompile(`(?i)\[stay_lovers\]`),
	"reconcile":      regexp.MustCompile(`(?i)\[Reconcile\]`),
	"need_help":      regexp.MustCompile(`(?i)\[Need Help\]`),
	"im_end":         regexp.MustCompile(`(?i)\[im_end\]|<\|im_end\|>|<\/im_end>|<\|eot_id\|>`),
}

var addExpPattern = regexp.MustCompile(`(?i)\[Add Exp:\s*\d+\s*\]`)

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

// ParseBondTags menghapus semua tag legacy dari teks AI.
// Nilai Add Exp diabaikan (tidak ada lagi penambahan XP).
func ParseBondTags(text string) BondParseResult {
	result := BondParseResult{
		Tags: make([]string, 0),
	}
	for key, pattern := range bondTagPatterns {
		if pattern.MatchString(text) {
			result.Tags = append(result.Tags, key)
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
