-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
--
-- models/result.go, models/smtp.go, and models/campaign.go already expect
-- these columns (added for enhanced tracking / multi-SMTP delivery), but the
-- corresponding SQL previously only existed under db/migrations/ (001, 003,
-- 004), a directory this application never actually reads at runtime
-- (MigrationsPath is configured as db/db_<driver>/migrations/, see
-- config.json's migrations_prefix + db_name). This relocates and applies
-- the missing pieces here. The new campaign_smtps table itself is not
-- recreated here since models.Setup already creates it via a GORM
-- AutoMigrate safety net (autoMigrateMultiSMTP); these are the columns on
-- pre-existing tables that AutoMigrate does not know to add on its own.
ALTER TABLE `results` ADD COLUMN `email_client` varchar(255) DEFAULT '';
ALTER TABLE `results` ADD COLUMN `device_type` varchar(255) DEFAULT '';
ALTER TABLE `results` ADD COLUMN `referrer` varchar(512) DEFAULT '';
ALTER TABLE `results` ADD COLUMN `tls_version` varchar(64) DEFAULT '';
ALTER TABLE `results` ADD COLUMN `total_opens` integer DEFAULT 0;
ALTER TABLE `results` ADD COLUMN `total_clicks` integer DEFAULT 0;
ALTER TABLE `results` ADD COLUMN `last_activity` datetime;
ALTER TABLE `results` ADD COLUMN `time_to_click_ms` integer DEFAULT 0;
ALTER TABLE `results` ADD COLUMN `risk_level` varchar(64) DEFAULT 'unknown';

ALTER TABLE `campaigns` ADD COLUMN `delay_between_ms` integer DEFAULT 0;
ALTER TABLE `campaigns` ADD COLUMN `selection_strategy` varchar(64) DEFAULT 'round_robin';
ALTER TABLE `campaigns` ADD COLUMN `max_emails_per_profile` integer DEFAULT 0;
ALTER TABLE `campaigns` ADD COLUMN `retry_failed_profiles` integer DEFAULT 0;

ALTER TABLE `smtp` ADD COLUMN `max_emails_per_hour` integer DEFAULT 0;
ALTER TABLE `smtp` ADD COLUMN `current_hour_count` integer DEFAULT 0;
ALTER TABLE `smtp` ADD COLUMN `hour_reset_time` datetime;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE `results` DROP COLUMN `email_client`;
ALTER TABLE `results` DROP COLUMN `device_type`;
ALTER TABLE `results` DROP COLUMN `referrer`;
ALTER TABLE `results` DROP COLUMN `tls_version`;
ALTER TABLE `results` DROP COLUMN `total_opens`;
ALTER TABLE `results` DROP COLUMN `total_clicks`;
ALTER TABLE `results` DROP COLUMN `last_activity`;
ALTER TABLE `results` DROP COLUMN `time_to_click_ms`;
ALTER TABLE `results` DROP COLUMN `risk_level`;
ALTER TABLE `campaigns` DROP COLUMN `delay_between_ms`;
ALTER TABLE `campaigns` DROP COLUMN `selection_strategy`;
ALTER TABLE `campaigns` DROP COLUMN `max_emails_per_profile`;
ALTER TABLE `campaigns` DROP COLUMN `retry_failed_profiles`;
ALTER TABLE `smtp` DROP COLUMN `max_emails_per_hour`;
ALTER TABLE `smtp` DROP COLUMN `current_hour_count`;
ALTER TABLE `smtp` DROP COLUMN `hour_reset_time`;
