package byok

import (
	"regexp"
	"strings"
)

// StreamSanitizer memfilter delta streaming secara incremental agar bond tags
// dan <think> tidak bocor ke frontend. Fragmen yang berpotensi tag terpotong
// antar-chunk ditahan (tidak di-emit) sampai chunk berikutnya tiba.
// Final CleanText tetap dihitung post-completion via ParseBondTags; Flush
// hanya agar sisa delta terakhir tidak hilang.
type StreamSanitizer struct {
	raw        strings.Builder
	emittedLen int
}

var (
	// Pola tag utuh yang langsung dibuang dari teks yang di-emit.
	streamTagPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\[BOND_DAMAGE\]`),
		regexp.MustCompile(`(?i)\[Mau Pacaran\]`),
		regexp.MustCompile(`(?i)\[Mau Nikah\]`),
		regexp.MustCompile(`(?i)\[accepted\]`),
		regexp.MustCompile(`(?i)\[married\]`),
		regexp.MustCompile(`(?i)\[rejected\]`),
		regexp.MustCompile(`(?i)\[stay_lovers\]`),
		regexp.MustCompile(`(?i)\[kesayangan\]`),
		regexp.MustCompile(`(?i)\[failed_nsfw\]`),
		regexp.MustCompile(`(?i)\[nsfw_bypassed\]`),
		regexp.MustCompile(`(?i)\[Reconcile\]`),
		regexp.MustCompile(`(?i)\[Need Help\]`),
		regexp.MustCompile(`(?i)\[im_end\]|<\|im_end\|>|<\/im_end>|<\|eot_id\|>`),
		regexp.MustCompile(`(?i)\[Add Exp:\s*\d+\s*\]`),
		regexp.MustCompile(`(?si)<think>.*?</think>`),
	}
	// Jendela maksimum tail yang ditahan karena berpotensi tag terpotong.
	streamHoldBackLen = 64
)

// cleanStreamText membuang tag utuh + <think> tak tertutup dari teks.
func cleanStreamText(text string) string {
	clean := text
	for _, p := range streamTagPatterns {
		clean = p.ReplaceAllString(clean, "")
	}
	if idx := strings.LastIndex(strings.ToLower(clean), "<think"); idx >= 0 {
		clean = clean[:idx]
	}
	return clean
}

// holdBack menghitung panjang suffix teks bersih yang harus ditahan karena
// berpotensi tag terpotong (ada '['/'<'/'|' tanpa penutup) dalam jendela.
func holdBack(clean string) int {
	start := len(clean) - streamHoldBackLen
	if start < 0 {
		start = 0
	}
	lastOpen := -1
	for i := len(clean) - 1; i >= start; i-- {
		if clean[i] == '[' || clean[i] == '<' || clean[i] == '|' {
			lastOpen = i
			break
		}
	}
	if lastOpen == -1 {
		return 0
	}
	if segment := clean[lastOpen:]; strings.Contains(segment, "]") || strings.Contains(segment, ">") {
		return 0
	}
	return len(clean) - lastOpen
}

// Feed menerima satu delta mentah, mengembalikan porsi bersih yang aman di-emit.
// Tidak pernah me-emit ulang: hanya suffix baru di luar emittedLen yang keluar.
func (s *StreamSanitizer) Feed(delta string) string {
	if delta == "" {
		return ""
	}
	s.raw.WriteString(delta)
	clean := cleanStreamText(s.raw.String())
	if len(clean) < s.emittedLen {
		// Pembersihan menyusutkan teks (tag di tengah terbuang); sinkronkan.
		s.emittedLen = len(clean)
		return ""
	}
	hold := holdBack(clean)
	end := len(clean) - hold
	if end < s.emittedLen {
		return ""
	}
	out := clean[s.emittedLen:end]
	s.emittedLen = end
	return out
}

// Flush mengembalikan sisa buffer yang sudah dibersihkan (dipanggil saat [DONE]).
func (s *StreamSanitizer) Flush() string {
	clean := cleanStreamText(s.raw.String())
	if len(clean) <= s.emittedLen {
		return ""
	}
	out := clean[s.emittedLen:]
	s.emittedLen = len(clean)
	return out
}
