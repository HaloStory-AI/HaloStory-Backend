-- 003_add_username_avatar_to_users.sql
-- Fase 1: tambah kolom username + avatar (nullable dulu) untuk 3k user existing.
-- Jalankan MANUAL (no AutoMigrate):
--   docker compose exec -T holox-maria-r0wusp mysql -u halostory -pchange_me halostory < migrations/003_add_username_avatar_to_users.sql
-- Setelah ini: jalankan backfill `docker compose exec app go run ./cmd/backfill_usernames`
-- Lalu fase 2: `migrations/003b_make_username_notnull_unique.sql`

ALTER TABLE users
  ADD COLUMN username VARCHAR(255) NULL AFTER `name`,
  ADD COLUMN avatar VARCHAR(255) NULL AFTER `username`;
