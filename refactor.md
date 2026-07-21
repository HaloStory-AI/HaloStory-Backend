# Refactor Plan: bluetalk_backend → halostory_go_backend

## ✅ Yang SUDAH di-porting (match dengan bluetalk)

| Feature | Status |
|---|---|
| Register / Login (JWT) | ✅ |
| Email Verification (OTP) | ✅ |
| User Profile (`GET /user`) | ✅ |
| Bot Index / Bot Bio | ✅ |
| Category Index / Category Bots | ✅ |
| Bond System (recruit, show, choice, debug) | ✅ |
| Bot Tab / Recruited Bots | ✅ |
| Character Stories (by bonded bots) | ✅ |
| Image Proxy (S3 / MinIO) | ✅ |
| JWT Auth + Banned Middleware | ✅ |
| Models: User, Bot, Category, UserBotBond, CharacterStory, EmailVerificationOtp | ✅ |

---

## ❌ Yang BELUM di-porting

### P1 — AI Chat System (Prioritas Tertinggi)

Bluetalk memiliki 17+ endpoint AI chat; halostory **0**.

| Endpoint / Feature | bluetalk | halostory |
|---|---|---|
| `POST /ai/chat` — Chat proxy ke AI provider | ✅ | ❌ |
| `GET /chat/status/{uuid}` — Cek status chat | ✅ | ❌ |
| `POST /ai/chat/system` — System-level chat | ✅ | ❌ |
| `POST /ai/summarize` — Summarize percakapan | ✅ | ❌ |
| `GET /ai/system-prompt/{bot_id}` — Ambil system prompt bot | ✅ | ❌ |
| 16 BYOK (Bring Your Own Key) providers | ✅ | ❌ |
| `POST /byok/models` — Fetch available models | ✅ | ❌ |
| Job queue system (Redis) untuk chat processing | ✅ | ❌ |
| 17 provider-specific job classes | ✅ | ❌ |
| VIP priority key pools | ✅ | ❌ |
| Static auth middleware (`API_STATIC_TOKEN`) | ✅ | ❌ |

**BYOK providers yang belum di-port:**

| No | Provider | Route |
|---|---|---|
| 1 | Gemini | `POST /byok/gemini/chat` |
| 2 | Claude | `POST /byok/claude/chat` |
| 3 | DeepSeek | `POST /byok/deepseek/chat` |
| 4 | Groq | `POST /byok/groq/chat` |
| 5 | Mistral | `POST /byok/mistral/chat` |
| 6 | OpenRouter | `POST /byok/openrouter/chat` |
| 7 | Arli | `POST /byok/arli/chat` |
| 8 | Mancer | `POST /byok/mancer/chat` |
| 9 | Featherless | `POST /byok/featherless/chat` |
| 10 | Chutes | `POST /byok/chutes/chat` |
| 11 | NanoGPT | `POST /byok/nanogpt/chat` |
| 12 | Venus | `POST /byok/venus/chat` |
| 13 | ZAI | `POST /byok/zai/chat` |
| 14 | NovelAI | `POST /byok/novel/chat` |
| 15 | Horde | `POST /byok/horde/chat` |
| 16 | OpenAI-compatible | `POST /byok/openai-compatible/chat` |

---

### P2 — Admin Panel

| Endpoint | bluetalk | halostory |
|---|---|---|
| `GET /admin/users` | ✅ | ❌ |
| `PUT /admin/users/{id}/role` | ✅ | ❌ |
| `DELETE /admin/users/{id}` | ✅ | ❌ |
| `GET /admin/bots` | ✅ | ❌ |
| `POST /admin/bots` | ✅ | ❌ |
| `POST /admin/bots/import` | ✅ | ❌ |
| `PUT /admin/bots/{id}` | ✅ | ❌ |
| `DELETE /admin/bots/{id}` | ✅ | ❌ |
| `GET /admin/student-requests` | ✅ | ❌ |
| `PUT /admin/student-requests/{id}/status` | ✅ | ❌ |

---

### P3 — User Management

| Endpoint | bluetalk | halostory |
|---|---|---|
| `POST /forgot-password` | ✅ | ❌ |
| `POST /reset-password` | ✅ | ❌ |
| `POST /user/update-email` | ✅ | ❌ |
| `POST /user/update-name` | ✅ | ❌ |
| `POST /user/update-password` | ✅ | ❌ |
| `POST /user/update-persona` | ✅ | ❌ |
| `POST /user/report-banned-device` | ✅ | ❌ |

---

### P4 — Missing Features

| Endpoint | bluetalk | halostory |
|---|---|---|
| `GET /app/status` (version, maintenance mode) | ✅ | ❌ |
| `POST /v1/calendar` | ✅ | ❌ |
| `GET /student-requests` (user-side) | ✅ | ❌ |
| `POST /student-requests` (user-side) | ✅ | ❌ |
| `POST /user/backup` | ✅ | ❌ |
| `GET /user/restore` | ✅ | ❌ |

---

### P5 — Database Tables / Models (belum ada di halostory)

| Table | Model baru yang diperlukan |
|---|---|
| `student_requests` | `models.StudentRequest` |
| `app_settings` | `models.AppSetting` |
| `user_backups` | `models.UserBackup` |
| `device_bans` | `models.DeviceBan` |
| `user_activities` | `models.UserActivity` |

---

### P6 — Infrastructure

| Komponen | bluetalk | halostory |
|---|---|---|
| CORS middleware (terkonfigurasi) | ✅ | ⚠️ (Gin default allow-all) |
| Queue / Job system (Redis) | ✅ | ❌ |
| Queue worker + Scheduler (Docker) | ✅ | ❌ |
| Docker compose (app + worker + scheduler + redis) | ✅ | ❌ |
| `.env.example` dengan dokumentasi | ✅ | ❌ |
| Test suite | ✅ (PHPUnit) | ❌ |

---

## Ringkasan Prioritas

```
P1 — AI Chat System        (17 endpoint, core value)
P2 — Admin Panel           (10 endpoint, operasional)
P3 — User Management       (7 endpoint)
P4 — Missing Features      (6 endpoint)
P5 — Database Models       (5 tabel baru)
P6 — Infrastructure        (Docker, CORS, test, env)
```
