# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

HaloStory backend — an anime AI character chat platform, ported from a Laravel codebase to Go (Gin + GORM + MySQL + Redis). Users chat with AI "bots" (characters), build a relationship/bond with XP leveling, browse a bot catalog, and read auto-generated character stories. Module: `halostory_go_backend`, Go 1.26.4.

For a deep architectural reference (data model, layer diagrams, extension patterns), see `ARCHITECTURE.md` — it's thorough and mostly still accurate; treat it as a map, not gospel, and verify against code for anything load-bearing. `AGENTS.md` has the same info in a terser Indonesian cheat-sheet form.

## Commands

```bash
air                      # Start dev server with hot reload — ALWAYS use this, not `go run`/`go build` directly
make build                # Build server + worker + scheduler into ./tmp
make test                 # go test -v -count=1 ./...
make test-coverage        # test + HTML coverage report
make docker-up             # docker compose up -d --build (server + worker + scheduler + redis)
make docker-down
```

Run a single test: `go test -v -run TestName ./internal/path/to/package`.

Redis-dependent tests are gated behind `TEST_REDIS=true` (e.g. `internal/services/queue_test.go`) and are skipped otherwise — set it when touching queue/rate-limit code:
```bash
TEST_REDIS=true make test
```

There is no auto-migration and no local schema setup — the database is managed externally (migrated from Laravel) and must already exist. Never add `AutoMigrate` calls.

## Architecture

Three binaries share the same `internal/` codebase but deploy separately:

- **`cmd/server`** (:3000) — Gin REST API. Boots as `.env → DB → S3 → Cloudinary → Redis → Gin(CORS) → :3000`.
- **`cmd/worker`** (:3001 health) — consumes the `queue:ai-chat` Redis list (BLPop) and does the actual AI chat calls. Two separate semaphores: 30 goroutines for BYOK requests, 6 for System AI.
- **`cmd/scheduler`** — robfig/cron jobs: every 1min unban expired users + purge low-rated stories older than 24h, hourly story generation from Safebooru, and hourly/daily/weekly/monthly Telegram analytics reports.

No dependency injection anywhere — services are package-level global singletons (`database.DB`, `services.RedisClient`, `services.MinioClient`, `services.CloudinaryClient`), accessed directly by controllers. There's no repository layer; controllers call GORM directly.

### Async AI chat flow

Chat is never synchronous end-to-end. `POST /api/ai/chat/system` or `POST /api/byok/:provider/chat` validates the request, enqueues a UUID job onto `queue:ai-chat` (Redis RPUSH), and returns the UUID immediately. The worker BLPops the job, takes a per-`user:bot` Redis `SetNX` lock (prevents duplicate concurrent processing for the same pair), dispatches to the right provider, parses the AI response for bond tags to update XP/relationship state, then `SETEX`s the result under `chat:{uuid}` with a 10-minute TTL. The client polls `GET /api/byok/chat/status/:uuid` (or the system equivalent) until the status flips to completed. When adding a new AI provider: add a controller under `internal/app/http/controllers/api/byok/`, a client in `internal/services/byok/`, wire the route into the `verified` group in `routes/api.go`, and add a dispatch case in `cmd/worker/main.go`.

- **System AI** (`internal/services/systemai/`): first-party fallback chain — NVIDIA → Huashang → Gemini → Mistral for regular users, with a priority chain for VIP — with a circuit breaker and key rotation (`fallback.go`).
- **BYOK** (`internal/services/byok/`): user-supplied API keys (Gemini, Claude, Mistral, DeepSeek, OpenRouter, ZAI, OpenAI-compatible). Keys are AES-256-CBC + HMAC-SHA256 encrypted at rest (`internal/utils/crypto.go`, Laravel-compatible format). `prompt.go` builds the system prompt (bond context, child-safety rules, group chat, language enforcement); `bond.go` parses bond tags out of the AI's response and applies XP/state changes.

### Bond system (relationship state machine)

`UserBotBond` (`internal/app/models/user_bot_bond.go`) carries real domain logic, not just a data record: `GetLevel()` (cubic XP curve, O(1) initial guess + refinement), `GetRelationshipLabel()`, `HasPendingChoice()` (decision points at level 50 and 100), `ApplyChoice()`, `CanGainXp()` (blocked once a bond is enemy/rejected/stuck). States: `normal → {lover, rejected, enemy}`, `lover → {married, stuck, beloved}` at level 100; `rejected`/`stuck` are terminal.

### Middleware chain — order matters

`Protected()` (JWT, HS256, 7-day expiry, sets `c.Set("user", user)`) → `CheckBanned()` → `EnsureEmailIsVerified()` → `CheckAdmin()`. Chat and bot-listing endpoints require the verified group; admin endpoints require JWT + banned check + admin role. `CheckStaticToken()` is a separate pre-auth gate on login/register/forgot-password (bearer token from `API_STATIC_TOKEN`), independent of the JWT chain. `/user/report-banned-device` is intentionally JWT-only (no ban check) so banned users can still report their device.

### Admin panel

`internal/admin/` is a self-contained embedded SPA (Tailwind + vanilla JS, `panel.html` is ~700 lines) served via `//go:embed` — no separate frontend build step. It talks to the same `/api/admin/*` endpoints, authenticated with its own admin session token from `/api/admin/login`.

## Repo-specific conventions (enforced by review, not tooling)

- **Never use `gin.H` for JSON responses — always a typed struct.** `gin.H` is a map, so field order isn't guaranteed; the Laravel-derived API contract expects stable ordering.
- **Return `make([]T, 0)`, never `nil`, for empty slice responses.**
- **Never add `AutoMigrate`** — the schema is owned by the legacy Laravel app.
- Error messages returned to clients are in Bahasa Indonesia.
- Nullable DB columns are Go pointer types (`*string`, `*time.Time`), not zero values.
- Passwords and OTP fields carry `json:"-"`.
- `models.TableName()` always returns the plural table name (GORM convention already matches Laravel's).
- Image paths from the DB (`avatar` field) are already full storage paths (`Bot/{id}/file`); build URLs with `fmt.Sprintf("%s/api/image?path=%s", appUrl, avatar)` — don't prepend `Bot/%s/` again.
- Debug-only endpoints (e.g. bond debug-update) must be guarded with `config.Env("APP_DEBUG", "false") != "true"`.

## Known sharp edges

- `internal/templates/otp.html` is loaded by a path relative to the working directory — breaks if the process isn't run from the repo root.
- The JWT signing key has a hardcoded fallback (`"halo_story_default_secret_key"`) if `APP_KEY` is unset — don't rely on this in any environment that matters.
- OTP generation uses `math/rand`, not `crypto/rand`.
- Birthday-detection date logic is duplicated between `bot_controller.go` and `category_controller.go` — if you fix a bug in one, check the other.
- Route versioning is inconsistent: some newer endpoints are under `/api/v1/*`, older ones are bare `/api/*` — check `internal/routes/api.go` for the actual prefix rather than assuming.
