-- 001_create_chat_jobs.sql
-- DB fallback untuk antrean chat saat Redis down.
-- Jalankan MANUAL sekali di MySQL/MariaDB (no AutoMigrate):
--   mysql -h <DB_HOST> -u <user> -p <DB_DATABASE> < migrations/001_create_chat_jobs.sql
CREATE TABLE IF NOT EXISTS chat_jobs (
  uuid CHAR(36) PRIMARY KEY,
  payload JSON NOT NULL,
  status ENUM('queued','processing','completed','failed') NOT NULL DEFAULT 'queued',
  response JSON NULL,
  status_code INT NOT NULL DEFAULT 0,
  locked_by VARCHAR(64) NULL,
  locked_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_status_created (status, created_at)
);
