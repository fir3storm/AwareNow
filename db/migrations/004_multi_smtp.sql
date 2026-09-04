-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Enhanced delivery system: multi-SMTP support, delivery configuration,
-- per-SMTP rate limiting, and SMTP usage tracking

-- ============================================================================
-- Campaign SMTPs Table
-- Maps multiple sending profiles to a single campaign with usage tracking
-- ============================================================================
CREATE TABLE IF NOT EXISTS "campaign_smtps" (
	"id" integer primary key autoincrement,
	"campaign_id" integer NOT NULL,
	"smtp_id" integer NOT NULL,
	"emails_sent" integer DEFAULT 0,
	"created_at" datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY ("campaign_id") REFERENCES "campaigns" ("id") ON DELETE CASCADE,
	FOREIGN KEY ("smtp_id") REFERENCES "smtp" ("id") ON DELETE CASCADE,
	UNIQUE ("campaign_id", "smtp_id")
);

-- ============================================================================
-- Alter Campaigns Table - Add delivery configuration columns
-- ============================================================================
ALTER TABLE "campaigns" ADD COLUMN "delay_between_ms" integer DEFAULT 0;
ALTER TABLE "campaigns" ADD COLUMN "selection_strategy" varchar(64) DEFAULT 'round_robin';
ALTER TABLE "campaigns" ADD COLUMN "max_emails_per_profile" integer DEFAULT 0;
ALTER TABLE "campaigns" ADD COLUMN "retry_failed_profiles" integer DEFAULT 0;

-- ============================================================================
-- Alter SMTP Table - Add rate limiting columns
-- ============================================================================
ALTER TABLE "smtp" ADD COLUMN "max_emails_per_hour" integer DEFAULT 0;
ALTER TABLE "smtp" ADD COLUMN "current_hour_count" integer DEFAULT 0;
ALTER TABLE "smtp" ADD COLUMN "hour_reset_time" datetime;

-- ============================================================================
-- Indexes for Campaign SMTPs
-- ============================================================================
CREATE INDEX IF NOT EXISTS "idx_campaign_smtps_campaign_id" ON "campaign_smtps" ("campaign_id");
CREATE INDEX IF NOT EXISTS "idx_campaign_smtps_smtp_id" ON "campaign_smtps" ("smtp_id");

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- WARNING: This will drop the campaign_smtps table and remove added columns.

-- Drop indexes
DROP INDEX IF EXISTS "idx_campaign_smtps_smtp_id";
DROP INDEX IF EXISTS "idx_campaign_smtps_campaign_id";

-- Drop campaign_smtps table
DROP TABLE IF EXISTS "campaign_smtps";

-- Note: SQLite does not support DROP COLUMN on ALTER TABLE.
-- The columns added to campaigns and smtp will remain if using SQLite.
-- For MySQL/PostgreSQL, you would use:
-- ALTER TABLE "campaigns" DROP COLUMN "delay_between_ms";
-- ALTER TABLE "campaigns" DROP COLUMN "selection_strategy";
-- ALTER TABLE "campaigns" DROP COLUMN "max_emails_per_profile";
-- ALTER TABLE "campaigns" DROP COLUMN "retry_failed_profiles";
-- ALTER TABLE "smtp" DROP COLUMN "max_emails_per_hour";
-- ALTER TABLE "smtp" DROP COLUMN "current_hour_count";
-- ALTER TABLE "smtp" DROP COLUMN "hour_reset_time";
