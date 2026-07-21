# HaloStory Go Backend — Agent Guide

Gin + GORM + MySQL + Redis, migrasi dari Laravel. DB sudah ada (tidak ada auto-migration).

## Quick commands

| Command | Description |
|---|---|
| `air` | Auto-reload on save |
| `make test` | Run all tests |
| `make build` | Build binary |
| `make docker-up` | Docker compose up |
| `make docker-down` | Docker compose down |

## Dev server

- Entrypoint: `cmd/server/main.go`
- Port: `:3000`
- Jangan jalankan `go run` / `go build` langsung — pakai `air`.

## Architecture

```
cmd/server/main.go        -- .env → DB → S3 → Redis → Gin (CORS) → :3000
internal/
  routes/api.go            -- Semua route
  config/config.go         -- Load .env + Env(key, fallback)
  database/database.go     -- Global `database.DB` (GORM)
  services/
    s3.go                  -- Global MinIO client
    redis.go               -- Global Redis client
  utils/
    jwt.go                 -- 7d expiry HS256
    mailer.go              -- Brevo SMTP
  app/
    models/                -- GORM structs + TableName() + hooks
    http/
      controllers/auth/    -- Login, Register, OTP
      controllers/api/     -- Bot, Bond, Category, Story, ImageProxy
      middlewares/
        auth_middleware.go -- Protected() (JWT)
        banned_middleware.go -- CheckBanned()
        cors.go            -- SetupCORS()
  templates/otp.html       -- Template email (path hardcoded relatif)
  tests/
    testutil.go            -- TestMain setup
```

## Key rules

1. **Response JSON WAJIB struct**, jangan `gin.H` — urutan field tidak terjamin dengan map.
2. **Nil slice → `make([]T, 0)`** — jangan return null.
3. **Image URL**: `fmt.Sprintf("%s/api/image?path=%s", appUrl, avatar)` — `avatar` dari DB sudah path lengkap (`Bot/{id}/file`). Jangan ditambah `Bot/%s/` lagi.
4. **No auto-migration.** Jangan tambah `AutoMigrate` ke `main.go`.

## Auth

- `Authorization: Bearer <token>` — HS256 JWT, signing key dari `APP_KEY`.
- Middleware chain: `Protected()` → `CheckBanned()` (urutan penting).
- User di context: `c.Get("user")` → type assert `models.User`.

## Models

- `TableName()` balikin nama plural (`bots`, `user_bot_bonds`, dll).
- Nullable fields pake pointer (`*string`, `*time.Time`).
- Password/OTP: `json:"-"` biar gak bocor ke response.

## Controllers

- Request binding: `c.ShouldBind(&req)` — handle JSON body & form params.
- User dari context: `userVal, _ := c.Get("user"); user := userVal.(models.User)`.
- Error messages pake Bahasa Indonesia.
- Debug-only endpoints: guard dengan `config.Env("APP_DEBUG", "false") != "true"`.

## Daftar endpoint (18)

| Method | Path | Auth |
|--------|------|------|
| GET | `/api/ping` | Public |
| GET | `/api/image?path=` | Public (S3 proxy) |
| POST | `/api/login` | Public |
| POST | `/api/register` | Public |
| GET | `/api/user` | Protected |
| POST | `/api/email/send-verification-otp` | Protected |
| POST | `/api/email/verify` | Protected |
| GET | `/api/v1/categories` | Protected |
| GET | `/api/v1/category/:slug` | Protected |
| GET | `/api/bots` | Protected |
| GET | `/api/v1/bot-bio/:bot_id` | Protected |
| GET | `/api/v1/bot-tab` | Protected |
| GET | `/api/v1/recruited-bots` | Protected |
| GET | `/api/v1/stories` | Protected |
| POST | `/api/bots/recruit` | Protected |
| GET | `/api/bots/bond/:bot_id` | Protected |
| POST | `/api/bots/bond/:bot_id/choice` | Protected |
| POST | `/api/bots/bond/:bot_id/debug` | Protected (DEBUG) |

## Konvensi rute

- Public: `/api/login`, `/api/register`, `/api/ping`, `/api/image`
- Versi: `/api/v1/*` untuk endpoint baru
- Bond detail: `/api/bots/bond/:bot_id/*`
- Naming campuran (kadang `/api/bots`, kadang `/api/v1/bot-tab`)

## Quirks yang perlu diketahui

- **Duplicate birthday logic** di `bot_controller.go` dan `category_controller.go`.
- **CORS**: `middlewares.SetupCORS()` memakai `gin-contrib/cors` — allow all origins.
- **`math/rand`** untuk OTP (bukan crypto/rand) — production issue.
- **Template path hardcoded**: `"internal/templates/otp.html"` — gagal kalau working directory bukan project root.
- **JWT fallback secret**: `"halo_story_default_secret_key"` — amankan di production.
