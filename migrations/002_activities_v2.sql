-- 002_activities_v2: analytics v2 — segmentasi + rollup per-bot
-- Apply manually: mysql < 002_activities_v2.sql (AGENTS.md: No AutoMigrate)

ALTER TABLE user_activities
  ADD COLUMN activity_type varchar(32) NOT NULL DEFAULT 'chat' COMMENT 'login|api_hit|chat|chat_summarize|bond_recruit|bond_choice|admin_action' AFTER user_id,
  ADD COLUMN bot_id varchar(255) NULL AFTER activity_type,
  ADD COLUMN provider varchar(32) NULL AFTER bot_id,
  ADD COLUMN ip varchar(45) NULL AFTER provider,
  ADD COLUMN user_agent varchar(512) NULL AFTER ip,
  ADD COLUMN metadata json NULL AFTER user_agent,
  ADD INDEX idx_type_created (activity_type, created_at),
  ADD INDEX idx_user_type_created (user_id, activity_type, created_at),
  ADD INDEX idx_bot_created (bot_id, created_at);

CREATE TABLE IF NOT EXISTS activity_daily_rollups (
  date date NOT NULL,
  activity_type varchar(32) NOT NULL,
  bot_id varchar(255) NULL,
  distinct_users int NOT NULL,
  total_events int NOT NULL,
  PRIMARY KEY (date, activity_type, bot_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
