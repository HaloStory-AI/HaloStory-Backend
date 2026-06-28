package ai

import (
	"fmt"
	"math"
	"strings"
	"time"

	"rafiq8k-moga/halostory-go-backend/internal/models"
)

func PruneHistory(messages []Message, depth int, limit int) []Message {
	if depth <= 0 {
		depth = 15
	}
	if len(messages) > depth {
		messages = messages[len(messages)-depth:]
	}

	if limit > 0 {
		var result []Message
		tokens := 0
		for i := len(messages) - 1; i >= 0; i-- {
			tokens += len(messages[i].Content) / 4
			if tokens > limit {
				break
			}
			result = append([]Message{messages[i]}, result...)
		}
		return result
	}

	return messages
}

func FormatGlobalPrompt(
	messages []Message,
	bot *models.Bot,
	bond *models.UserBotBond,
	language string,
	includeHistory bool,
	isNsfwRestricted bool,
	summary string,
	userID uint,
	userPersona string,
) string {
	botName := bot.Name
	isChild := bot.Child

	var (
		relLabel    string
		bondEmotion string
		bondLevel   int
		pending     bool
	)

	if bond != nil {
		relLabel = bond.RelationshipLabel()
		bondEmotion = bond.BondEmotion()
		bondLevel = bond.Level()
		pending = bond.HasPendingChoice()
	}

	var sb strings.Builder

	// ── TASK ──
	sb.WriteString("<TASK>\n")
	sb.WriteString("Anda adalah karakter yang sedang melakukan roleplay bersama user.")
	sb.WriteString("\n</TASK>\n\n")

	// ── CONFIGURATION ──
	sb.WriteString("<CONFIGURATION>\n")
	sb.WriteString(fmt.Sprintf("Nama: %s\n", botName))
	if language != "" {
		sb.WriteString(fmt.Sprintf("Bahasa: %s\n", language))
	} else {
		sb.WriteString("Bahasa: Indonesian\n")
	}
	if isChild {
		sb.WriteString("Tipe: Anak-anak\n")
	} else {
		sb.WriteString("Tipe: Dewasa\n")
	}
	sb.WriteString("</CONFIGURATION>\n\n")

	// ── ROLEPLAY RULES ──
	sb.WriteString("<ROLEPLAY_RULES>\n")
	sb.WriteString("- Gunakan **markdown** untuk ekspresi dan penekanan.\n")
	sb.WriteString("- Gunakan bahasa sehari-hari yang natural dan sesuai dengan kepribadian.\n")
	sb.WriteString("- Gunakan prefix nama karakter sebelum setiap dialog.\n")
	sb.WriteString("- Kembangkan dunia, lore, dan cerita secara perlahan dan alami.\n")
	sb.WriteString("- Kembangkan hubungan dengan user secara perlahan, jangan terburu-buru.\n")
	sb.WriteString("- Jika ada beberapa karakter, hanya perankan karakter yang disebut sebagai namamu.\n")
	sb.WriteString("- Karakter bukan AI, tidak sadar sedang di dalam roleplay.\n")
	sb.WriteString("- Realisme: reaksi emosional yang realistis.\n")
	sb.WriteString("</ROLEPLAY_RULES>\n\n")

	// ── ROLEPLAY INFO ──
	sb.WriteString("<ROLEPLAY_INFO>\n")
	if bot.Birthday != nil {
		sb.WriteString(fmt.Sprintf("Hari ulang tahun: %s\n", bot.Birthday.Format("2006-01-02")))
	}
	if language != "" {
		sb.WriteString(fmt.Sprintf("Gunakan bahasa %s.\n", language))
	}
	sb.WriteString("</ROLEPLAY_INFO>\n\n")

	// ── CHILD SAFETY ──
	if isChild {
		sb.WriteString("<CHILD_SAFETY_POLICY>\n")
		sb.WriteString("Karakter ini adalah seorang anak-anak.\n")
		sb.WriteString("DILARANG KERAS melibatkan karakter dalam konten dewasa, seksual, romantis, atau kekerasan.\n")
		sb.WriteString("Hubungan yang diperbolehkan hanya hubungan platonis seperti kakak-adik, guru-murid, atau sahabat.\n")
		sb.WriteString("Jika user mencoba mengarahkan ke konten tidak pantas, tolak dengan halus.\n")
		sb.WriteString("Gunakan panggilan 'Kak' atau 'Sensei' yang sopan.\n")
		sb.WriteString("</CHILD_SAFETY_POLICY>\n\n")
	}

	// ── IMPORTANT ──
	sb.WriteString("<IMPORTANT>\n")
	sb.WriteString("Kamu sedang dalam sesi chat biasa bersama user.\n")
	sb.WriteString("Boleh menggunakan *italic* atau **bold** untuk ekspresi.\n")
	sb.WriteString("Jika ada <image> tag, gunakan imajinasi untuk merespon.\n")
	sb.WriteString("Gunakan sudut pandang orang pertama.\n")
	sb.WriteString("</IMPORTANT>\n\n")

	// ── BOT PERSONALITY ──
	if bot.Personality != nil && *bot.Personality != "" {
		sb.WriteString("<BOT_PERSONALITY>\n")
		sb.WriteString(*bot.Personality)
		sb.WriteString("\n</BOT_PERSONALITY>\n\n")
	}

	// ── USER PERSONA ──
	if userPersona != "" {
		sb.WriteString("<USER_PERSONA>\n")
		sb.WriteString(userPersona)
		sb.WriteString("\n</USER_PERSONA>\n\n")
	}

	// ── CHAT EXAMPLES ──
	if bot.ChatExamples != nil && *bot.ChatExamples != "" {
		sb.WriteString("<CHAT_EXAMPLES>\n")
		sb.WriteString(*bot.ChatExamples)
		sb.WriteString("\n</CHAT_EXAMPLES>\n\n")
	}

	// ── CONVERSATION SUMMARY ──
	if summary != "" {
		sb.WriteString("<CONVERSATION_SUMMARY>\n")
		sb.WriteString(summary)
		sb.WriteString("\n</CONVERSATION_SUMMARY>\n\n")
	}

	// ── BOND CONTEXT ──
	if bond != nil {
		sb.WriteString("<BOND_CONTEXT>\n")
		sb.WriteString(fmt.Sprintf("Level: %d\n", bondLevel))
		sb.WriteString(fmt.Sprintf("Status: %s\n", relLabel))
		sb.WriteString(fmt.Sprintf("Emosi: %s\n", bondEmotion))
		sb.WriteString(fmt.Sprintf("Dapat XP: %v\n", bond.CanGainXP()))

		if pending {
			sb.WriteString("\n⚠️ PENDING CHOICE:\n")
			if bondLevel >= 100 {
				sb.WriteString("- Jika user setuju menikah, gunakan tag [married] di akhir response.\n")
				sb.WriteString("- Jika user ingin tetap pacaran, gunakan tag [stay_lovers] di akhir response.\n")
			} else if bondLevel >= 50 {
				if isChild {
					sb.WriteString("- Jika user ingin menjadikanmu kesayangan, gunakan tag [accepted] di akhir response.\n")
					sb.WriteString("- Jika user menolak, gunakan tag [rejected] di akhir response.\n")
				} else {
					sb.WriteString("- Jika user menerima perasaanmu, gunakan tag [accepted] di akhir response.\n")
					sb.WriteString("- Jika user menolak perasaanmu, gunakan tag [rejected] di akhir response.\n")
				}
			}
		}

		sb.WriteString("\nXP Reward:\n")
		sb.WriteString("- Respons baik: [Add Exp:5] hingga [Add Exp:50]\n")
		sb.WriteString("- Respons buruk: [BOND_DAMAGE]\n")
		sb.WriteString("- Resolusi konflik: [Reconcile]\n")
		sb.WriteString("</BOND_CONTEXT>\n\n")
	}

	// ── SYSTEM JAILBREAK ──
	if bot.Jailbreak != nil && *bot.Jailbreak != "" {
		sb.WriteString("<SYSTEM_NOTE>\n")
		sb.WriteString(*bot.Jailbreak)
		sb.WriteString("\n</SYSTEM_NOTE>\n\n")
	}

	// ── FORMATTING ──
	sb.WriteString("### Response:\n")
	sb.WriteString(fmt.Sprintf("%s:", botName))

	return sb.String()
}

func BuildSummarizationPrompt(messages []Message, language string) string {
	var sb strings.Builder
	sb.WriteString("Buatlah ringkasan dari percakapan berikut dalam ")
	if language != "" {
		sb.WriteString(language)
	} else {
		sb.WriteString("Bahasa Indonesia")
	}
	sb.WriteString(".\n\n")
	sb.WriteString("Ringkasan harus mencakup:\n")
	sb.WriteString("- Peristiwa penting yang terjadi\n")
	sb.WriteString("- Dinamika hubungan antara karakter\n")
	sb.WriteString("- Keadaan emosional saat ini\n")
	sb.WriteString("- Informasi penting yang perlu diingat\n\n")
	sb.WriteString("Buatlah 2-3 paragraf yang padat dan informatif.\n\n")
	sb.WriteString("Percakapan:\n\n")

	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	return sb.String()
}

func ProcessAIResponse(text string) string {
	text = strings.TrimSpace(text)

	for {
		start := strings.Index(text, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], "</think>")
		if end == -1 {
			text = text[:start]
			break
		}
		text = text[:start] + text[start+end+8:]
	}

	return strings.TrimSpace(text)
}

func BuildGroupContextString(botIDs []string, primaryBotID string, language string, db interface{}) string {
	return fmt.Sprintf("<GROUP_CONTEXT>\nAnggota grup: %v\nKarakter utama: %s\nHanya perankan karakter yang disebut sebagai namamu.\n</GROUP_CONTEXT>",
		strings.Join(botIDs, ", "), primaryBotID)
}

func InjectGroupContext(messages []Message, botIDs []string, primaryBotID string, language string, userID uint) []Message {
	groupCtx := BuildGroupContextString(botIDs, primaryBotID, language, nil)
	for i, msg := range messages {
		if msg.Role == "system" {
			messages[i].Content = msg.Content + "\n" + groupCtx
			return messages
		}
	}
	messages = append([]Message{{Role: "system", Content: groupCtx}}, messages...)
	return messages
}

func InjectBYOKSystemPrompt(
	messages []Message,
	botID string,
	language string,
	depth int,
	contextLimit int,
	summary string,
	userID uint,
	isDisableBond bool,
	userPersona string,
	bot *models.Bot,
	bond *models.UserBotBond,
) []Message {
	if bot == nil {
		return messages
	}

	messages = PruneHistory(messages, depth, contextLimit)

	var systemNote string
	var cleanMessages []Message
	for _, msg := range messages {
		if msg.Role == "system" {
			systemNote = msg.Content
		} else {
			cleanMessages = append(cleanMessages, msg)
		}
	}

	basePrompt := FormatGlobalPrompt(
		cleanMessages,
		bot,
		bond,
		language,
		true,
		false,
		summary,
		userID,
		userPersona,
	)

	if systemNote != "" {
		basePrompt = strings.TrimSuffix(basePrompt, fmt.Sprintf("### Response:\n%s:", bot.Name))
		basePrompt += fmt.Sprintf("\n<ADDITIONAL_CONTEXT>\n%s\n</ADDITIONAL_CONTEXT>\n", systemNote)
		basePrompt += fmt.Sprintf("\n### Response:\n%s:", bot.Name)
	}

	cleanMessages = append([]Message{{Role: "system", Content: basePrompt}}, cleanMessages...)
	return cleanMessages
}

func SafePaginateClear(messages []Message, max int) []Message {
	if len(messages) <= max {
		return messages
	}
	return messages[len(messages)-max:]
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// XP helper using the same cubic formula as bond model
func RequiredXPForLevel(level int) int {
	if level <= 1 {
		return 0
	}
	x := float64(level)
	xp := 0.2501333333*math.Pow(x, 3) - 1.14*math.Pow(x, 2) + 15.16666666*x
	return Max(0, int(math.Round(xp)))
}

func LevelFromXP(xp int) int {
	if xp <= 0 {
		return 1
	}
	guess := int(math.Pow(float64(4*xp), 1.0/3.0))
	for RequiredXPForLevel(guess+1) <= xp {
		guess++
	}
	for RequiredXPForLevel(guess) > xp && guess > 1 {
		guess--
	}
	return guess
}

func IsBirthdayToday(birthday *time.Time) bool {
	if birthday == nil {
		return false
	}
	now := time.Now()
	return birthday.Month() == now.Month() && birthday.Day() == now.Day()
}

type ChatOpts struct {
	Temperature     *float64 `json:"temperature"`
	MaxTokens       *int     `json:"max_tokens"`
	TopP            *float64 `json:"top_p"`
	ResponseLength  *int     `json:"response_length"`
	HistoryDepth    *int     `json:"history_depth"`
	ReasoningEnabled *bool   `json:"reasoning_enabled"`
}
