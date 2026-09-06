
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
--
-- models/scenario.go and models/assessment.go (USP-2: provenance and
-- scenarios) already expect these tables; models.Setup also creates them
-- via a GORM AutoMigrate safety net (autoMigrateScenariosAndAssessments),
-- but this migration documents the schema explicitly, matching the
-- convention already used elsewhere in this repo (see the reported
-- message tables' own history, and 20260905120000).
CREATE TABLE "scenarios" (
    "id"                          INTEGER PRIMARY KEY AUTOINCREMENT,
    "owner_id"                    INTEGER,
    "source_reported_message_id" INTEGER,
    "name"                        VARCHAR(255) NOT NULL,
    "skill_tag"                   VARCHAR(255) NOT NULL,
    "kind"                        VARCHAR(64) NOT NULL,
    "version"                     INTEGER NOT NULL,
    "subject"                     VARCHAR(255),
    "html"                        TEXT,
    "text"                        TEXT,
    "status"                      VARCHAR(64) NOT NULL,
    "reviewed_by"                 VARCHAR(255),
    "reviewed_at"                 DATETIME,
    "created_at"                  DATETIME
);

CREATE INDEX "idx_scenarios_owner_id" ON "scenarios" ("owner_id");
CREATE INDEX "idx_scenarios_source_reported_message_id" ON "scenarios" ("source_reported_message_id");

CREATE TABLE "assessments" (
    "id"                        INTEGER PRIMARY KEY AUTOINCREMENT,
    "owner_id"                  INTEGER,
    "name"                      VARCHAR(255) NOT NULL,
    "skill_tag"                 VARCHAR(255) NOT NULL,
    "baseline_scenario_id"      INTEGER NOT NULL,
    "followup_scenario_id"      INTEGER,
    "benign_scenario_id"        INTEGER,
    "observation_window_hours"  INTEGER NOT NULL,
    "status"                    VARCHAR(64) NOT NULL,
    "created_at"                DATETIME
);

CREATE INDEX "idx_assessments_owner_id" ON "assessments" ("owner_id");

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE "assessments";
DROP TABLE "scenarios";
