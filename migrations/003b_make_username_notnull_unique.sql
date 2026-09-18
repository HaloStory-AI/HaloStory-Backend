-- 003b_make_username_notnull_unique.sql
-- Fase 2: Jadikan username NOT NULL + UNIQUE setelah backfill 3k user selesai.
-- Jalankan MANUAL (no AutoMigrate) — PASTIKAN dulu:
--   docker compose exec app go run ./cmd/backfill_usernames
-- Verifikasi sebelum run:
--   SELECT COUNT(*) FROM users WHERE username IS NULL OR username = '';
--   SELECT username, COUNT(*) c FROM users GROUP BY username HAVING c > 1;

-- Pastikan tidak ada NULL/kosong sebelum MODIFY (opsional guard):
-- Jika masih ada NULL, query ini akan error dan mencegah MODIFY.
-- Manual check di atas lebih aman daripada guard otomatis.

ALTER TABLE users MODIFY COLUMN username VARCHAR(255) NOT NULL;

CREATE UNIQUE INDEX idx_users_username_unique ON users(username);
