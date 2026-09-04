-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Enhanced tracking: device fingerprinting, behavior events, sessions,
-- and result enrichment for risk scoring

-- ============================================================================
-- Device Fingerprints Table
-- Stores browser fingerprinting data collected from campaign recipients
-- ============================================================================
CREATE TABLE IF NOT EXISTS "device_fingerprints" (
	"id" integer primary key autoincrement,
	"campaign_id" integer NOT NULL,
	"r_id" varchar(255) NOT NULL,
	"fingerprint" text,
	"user_agent" text,
	"screen_resolution" varchar(255),
	"color_depth" integer DEFAULT 0,
	"timezone" varchar(255),
	"language" varchar(255),
	"platform" varchar(255),
	"cookies" text,
	"created_at" datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY ("campaign_id") REFERENCES "campaigns" ("id") ON DELETE CASCADE
);

-- ============================================================================
-- Behavior Events Table
-- Records granular user interactions (clicks, form submits, navigation, etc.)
-- ============================================================================
CREATE TABLE IF NOT EXISTS "behavior_events" (
	"id" integer primary key autoincrement,
	"campaign_id" integer NOT NULL,
	"r_id" varchar(255) NOT NULL,
	"event_type" varchar(255) NOT NULL,
	"event_data" text,
	"session_id" varchar(255),
	"time_on_page" integer DEFAULT 0,
	"created_at" datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY ("campaign_id") REFERENCES "campaigns" ("id") ON DELETE CASCADE
);

-- ============================================================================
-- Sessions Table
-- Represents a single visit/engagement session by a campaign recipient
-- ============================================================================
CREATE TABLE IF NOT EXISTS "sessions" (
	"id" integer primary key autoincrement,
	"r_id" varchar(255) NOT NULL,
	"campaign_id" integer NOT NULL,
	"started_at" datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
	"ended_at" datetime,
	"duration" integer DEFAULT 0,
	"pages_viewed" integer DEFAULT 0,
	"events_count" integer DEFAULT 0,
	"device_fingerprint" varchar(512),
	FOREIGN KEY ("campaign_id") REFERENCES "campaigns" ("id") ON DELETE CASCADE
);

-- ============================================================================
-- Alter Results Table - Add enhanced tracking columns
-- ============================================================================
ALTER TABLE "results" ADD COLUMN "email_client" varchar(255) DEFAULT '';
ALTER TABLE "results" ADD COLUMN "device_type" varchar(255) DEFAULT '';
ALTER TABLE "results" ADD COLUMN "referrer" varchar(512) DEFAULT '';
ALTER TABLE "results" ADD COLUMN "tls_version" varchar(64) DEFAULT '';
ALTER TABLE "results" ADD COLUMN "total_opens" integer DEFAULT 0;
ALTER TABLE "results" ADD COLUMN "total_clicks" integer DEFAULT 0;
ALTER TABLE "results" ADD COLUMN "last_activity" datetime;
ALTER TABLE "results" ADD COLUMN "time_to_click_ms" integer DEFAULT 0;
ALTER TABLE "results" ADD COLUMN "risk_level" varchar(64) DEFAULT 'unknown';

-- ============================================================================
-- Indexes for Device Fingerprints
-- ============================================================================
CREATE INDEX IF NOT EXISTS "idx_device_fingerprints_campaign_id" ON "device_fingerprints" ("campaign_id");
CREATE INDEX IF NOT EXISTS "idx_device_fingerprints_r_id" ON "device_fingerprints" ("r_id");
CREATE INDEX IF NOT EXISTS "idx_device_fingerprints_created_at" ON "device_fingerprints" ("created_at");
CREATE INDEX IF NOT EXISTS "idx_device_fingerprints_campaign_r_id" ON "device_fingerprints" ("campaign_id", "r_id");

-- ============================================================================
-- Indexes for Behavior Events
-- ============================================================================
CREATE INDEX IF NOT EXISTS "idx_behavior_events_campaign_id" ON "behavior_events" ("campaign_id");
CREATE INDEX IF NOT EXISTS "idx_behavior_events_r_id" ON "behavior_events" ("r_id");
CREATE INDEX IF NOT EXISTS "idx_behavior_events_event_type" ON "behavior_events" ("event_type");
CREATE INDEX IF NOT EXISTS "idx_behavior_events_session_id" ON "behavior_events" ("session_id");
CREATE INDEX IF NOT EXISTS "idx_behavior_events_created_at" ON "behavior_events" ("created_at");
CREATE INDEX IF NOT EXISTS "idx_behavior_events_campaign_r_id" ON "behavior_events" ("campaign_id", "r_id");
CREATE INDEX IF NOT EXISTS "idx_behavior_events_campaign_type" ON "behavior_events" ("campaign_id", "event_type");

-- ============================================================================
-- Indexes for Sessions
-- ============================================================================
CREATE INDEX IF NOT EXISTS "idx_sessions_r_id" ON "sessions" ("r_id");
CREATE INDEX IF NOT EXISTS "idx_sessions_campaign_id" ON "sessions" ("campaign_id");
CREATE INDEX IF NOT EXISTS "idx_sessions_started_at" ON "sessions" ("started_at");
CREATE INDEX IF NOT EXISTS "idx_sessions_campaign_r_id" ON "sessions" ("campaign_id", "r_id");

-- ============================================================================
-- Indexes for Results (enhanced columns)
-- ============================================================================
CREATE INDEX IF NOT EXISTS "idx_results_risk_level" ON "results" ("risk_level");
CREATE INDEX IF NOT EXISTS "idx_results_campaign_risk_level" ON "results" ("campaign_id", "risk_level");
CREATE INDEX IF NOT EXISTS "idx_results_last_activity" ON "results" ("last_activity");
CREATE INDEX IF NOT EXISTS "idx_results_device_type" ON "results" ("device_type");
CREATE INDEX IF NOT EXISTS "idx_results_email_client" ON "results" ("email_client");

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
-- WARNING: This will delete all enhanced tracking data and remove the added columns.

-- Drop indexes for Results
DROP INDEX IF EXISTS "idx_results_email_client";
DROP INDEX IF EXISTS "idx_results_device_type";
DROP INDEX IF EXISTS "idx_results_last_activity";
DROP INDEX IF EXISTS "idx_results_campaign_risk_level";
DROP INDEX IF EXISTS "idx_results_risk_level";

-- Drop indexes for Sessions
DROP INDEX IF EXISTS "idx_sessions_campaign_r_id";
DROP INDEX IF EXISTS "idx_sessions_started_at";
DROP INDEX IF EXISTS "idx_sessions_campaign_id";
DROP INDEX IF EXISTS "idx_sessions_r_id";

-- Drop indexes for Behavior Events
DROP INDEX IF EXISTS "idx_behavior_events_campaign_type";
DROP INDEX IF EXISTS "idx_behavior_events_campaign_r_id";
DROP INDEX IF EXISTS "idx_behavior_events_created_at";
DROP INDEX IF EXISTS "idx_behavior_events_session_id";
DROP INDEX IF EXISTS "idx_behavior_events_event_type";
DROP INDEX IF EXISTS "idx_behavior_events_r_id";
DROP INDEX IF EXISTS "idx_behavior_events_campaign_id";

-- Drop indexes for Device Fingerprints
DROP INDEX IF EXISTS "idx_device_fingerprints_campaign_r_id";
DROP INDEX IF EXISTS "idx_device_fingerprints_created_at";
DROP INDEX IF EXISTS "idx_device_fingerprints_r_id";
DROP INDEX IF EXISTS "idx_device_fingerprints_campaign_id";

-- Drop tables
DROP TABLE IF EXISTS "sessions";
DROP TABLE IF EXISTS "behavior_events";
DROP TABLE IF EXISTS "device_fingerprints";
