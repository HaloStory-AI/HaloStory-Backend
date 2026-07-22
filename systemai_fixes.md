# System AI & Codebase Fixes — Audit Findings

## 🔴 Critical (13)

### 1-3. Defer-in-loop resource leak
**Files:** `internal/services/byok/openai.go:108`, `internal/services/byok/gemini.go:133`, `internal/services/byok/claude.go:90`
`defer resp.Body.Close()` dalam retry loop → bocor 9 file descriptor/koneksi per request.
**Fix:** Pindah `defer` ke luar loop, atau close manual di tiap iterasi.

### 4. Garbage error message
**File:** `internal/services/byok/validate.go:143`
`string(rune(i))` menghasilkan `messages\x00` (null byte) instead of `"messages[0]"`.
**Fix:** Ganti `string(rune(i))` → `strconv.Itoa(i)`.

### 5. Slice bounds panic
**File:** `internal/services/byok/bond.go:244`
`aiResponse[:256]` panic kalau response < 256 karakter.
**Fix:** `min(256, len(aiResponse))`.

### 6. Nil pointer dereference
**File:** `cmd/worker/main.go:376`
`geminiResp.PromptFeedback` bisa nil → `.BlockReasonMessage` panic.
**Fix:** Guard `if geminiResp.PromptFeedback != nil`.

### 7. Fragile type assertion
**File:** `cmd/worker/main.go:1217,1219,1221`
`ConvertClaudeToOpenAIFormat` return `map[string]interface{}` — type assertion `.([]map[string]interface{})` panic kalau struktur beda. Violasi aturan "Response WAJIB struct".
**Fix:** Buat struct khusus Claude response, jangan pakai map.

### 8. Hardcoded JWT fallback secret
**Files:** `internal/app/http/middlewares/auth_middleware.go:39`, `internal/utils/jwt.go:14`
Fallback `"halo_story_default_secret_key"` — token bisa dipalsukan.
**Fix:** Return error kalau `APP_KEY` kosong, jangan pakai fallback.

### 9. Semua cron expression salah
**File:** `internal/services/scheduler.go:29-32`
| Fungsi | Cron sekarang | Seharusnya |
|--------|--------------|------------|
| `sendHourlyAnalytics` | `0 0 0 * * *` (daily) | `0 * * * *` (hourly) |
| `sendDailyAnalytics` | `0 0 0 * * 1` (weekly) | `0 0 * * *` (daily) |
| `sendWeeklyAnalytics` | `0 0 0 1 * *` (monthly) | `0 0 * * 0` (weekly) |
| `sendMonthlyAnalytics` | `0 0 0 1 1 *` (yearly) | `0 0 1 * *` (monthly) |

**Fix:** Perbaiki cron expression sesuai fungsi.

### 10. OTP pakai math/rand (bisa ditebak)
**Files:** `internal/app/http/controllers/auth/verification_controller.go:21`, `internal/app/http/controllers/auth/user_controller.go:22`
`math/rand` tidak cryptographic — OTP bisa ditebak.
**Fix:** Ganti `math/rand` → `crypto/rand`.

### 11. Birthday format tahun 2007
**File:** `internal/app/http/controllers/api/category_controller.go:128`
`birthday.Format("2007-01-02")` → tahun `2007` bukan reference year Go (`2006`).
**Fix:** Ganti `"2007-01-02"` → `"2006-01-02"`.

### 12. Gemma model detection broken
**File:** `cmd/worker/main.go:247`
`data.Model[len(data.Model)-1]` ambil byte terakhir (`t`), bukan rune. `"gemma-4-31b-it"` → char terakhir `t`, bukan digit.
**Fix:** Pake `strings.HasSuffix()` atau regex untuk deteksi versi.

### 13. BYOK controllers tanpa validasi
**Files:** `claude_controller.go`, `openai_compatible_controller.go`, `helpers.go` (Mistral/DeepSeek/OpenRouter/ZAI)
Tidak ada `SanitizeByokInput()` — beda dengan Gemini controller yang ada validasi.
**Fix:** Tambah `SanitizeByokInput()` di semua controller yang kurang.

---

## 🟡 Missing Features (from Laravel)

### M1. System AI providers incomplete
Hanya Gemini, Huashang, Nvidia, Mistral di chain. Laravel punya 10 provider:
`Gemini` `GeminiPriority` `Groq` `OpenRouter` `MistralAI` `Cohere` `Cerebras` `Huashang` `HuashangPriority` `Nvidia`
**Fix:** Tambah provider entries di `systemai.go` + env keys.

### M2. Email verification middleware
`EnsureEmailIsVerified` di Laravel — belum ada di Go.
**Fix:** Buat middleware baru, pasang di route tertentu.

### M3. User account management
`deleteAccount`, `logout`, `getProfile`, `getVipStatus`, `redeemVipCode` — belum di-port.

### M4. Admin: bot import CSV + stats + device bans
Sebagian ada, sebagian belum.

---

## 🔵 Should-Have (14-25)

| # | File | Issue |
|---|------|-------|
| 14 | `internal/services/ratelimit.go:9` | Pakai `context.Background()` bukan `RedisCtx` |
| 15 | `internal/services/ratelimit.go:17` | Error `Expire` diabaikan — key bisa hidup selamanya |
| 16 | `internal/services/queue.go:62-109` | `UpdateChatStatus` + `MarkChatCompleted` race condition (read-modify-write) |
| 17 | `cmd/worker/main.go:92` | Tanpa backoff saat Redis down → CPU burn |
| 18 | `internal/app/http/controllers/auth/auth_controller.go:25` | Rate limit `ClientIP()` — di belakang proxy salah |
| 19 | `internal/services/systemai/fallback.go:288` | Chain timeout pakai `continue` bukan `break` |
| 20 | `internal/services/systemai/fallback.go:389` | VIP key rotation di failure — bakar semua keys |
| 21 | `internal/app/http/controllers/api/byok/chat_status_controller.go:34` | `DeleteChatResult` sebelum response — risk data loss |
| 22 | `cmd/worker/main.go:441` | Double write ke Redis tanpa atomicity |
| 23 | `cmd/worker/main.go:483` | Redis lock error cuma di-log, gak abort job |
| 24 | `cmd/worker/main.go:284 vs 931` | Default temperature Gemini 0.7 vs OpenAI 1.0 — inkonsisten |
| 25 | `internal/config/config.go:22` | `godotenv.Load()` tanpa path — gagal kalau CWD bukan project root |

---

## ⚪ Nice-to-Have

- Default model `"gpt-3.5-turbo"` outdated (`internal/services/byok/openai.go:56`)
- N+1 query bot di `internal/services/byok/prompt.go` (query 2x per request)
- `internal/services/byok/models.go:67` — `fmt.Sprintf` unused placeholder `%s`
- `internal/services/byok/models.go:406` — Horde models sorted descending (terbalik)
- `cmd/worker/main.go:615` — Language conversion `"id"` → `"Indonesian"` unnecessary
- `cmd/worker/main.go:1295` — Claude summarization retry dobel (total 100 attempts)

---

## 📦 Batch Plan

| Batch | Item | Estimasi |
|-------|------|----------|
| 1 — Kebocoran & Panic | 1,2,3,5,6,7 | 30 menit |
| 2 — Keamanan | 8,10 | 15 menit |
| 3 — Data Correctness | 4,9,11,12,13 | 30 menit |
| 4 — Redis & Queue | 14,15,16,17,21 | 30 menit |
| 5 — System AI | 19,20, M1 | 30 menit |
| 6 — Missing Features | M2, M3, M4 | 2-3 jam |

Total estimasi: ~5 jam.
