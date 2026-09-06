
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
--
-- models/assessment_phase.go (USP-3: assessment orchestration) already
-- expects this table; models.Setup also creates it via a GORM AutoMigrate
-- safety net (autoMigrateAssessmentPhases), but this migration documents
-- the schema explicitly, matching the convention already used elsewhere
-- in this repo (see 20260906120000_scenarios_and_assessments.sql).
CREATE TABLE IF NOT EXISTS `assessment_phases` (
    `id`            INTEGER PRIMARY KEY AUTO_INCREMENT,
    `owner_id`      INTEGER,
    `assessment_id` INTEGER NOT NULL,
    `phase`         VARCHAR(64) NOT NULL,
    `campaign_id`   INTEGER NOT NULL,
    `created_at`    DATETIME,
    `updated_at`    DATETIME,
    INDEX `idx_assessment_phases_owner_id` (`owner_id`),
    INDEX `idx_assessment_phases_assessment_id` (`assessment_id`),
    -- Defense-in-depth: LinkAssessmentPhase already selects-then-inserts-or-
    -- updates by (assessment_id, phase) to enforce the "one campaign per
    -- phase per assessment" invariant, but this unique index also protects
    -- against a genuine race between two concurrent calls that both miss
    -- the SELECT.
    UNIQUE INDEX `idx_assessment_phases_assessment_id_phase` (`assessment_id`, `phase`)
);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE `assessment_phases`;
