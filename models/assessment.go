package models

import (
	"errors"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/jinzhu/gorm"
)

// AssessmentStatusDraft indicates an assessment definition that has not
// been launched (no cohort/campaign orchestration exists yet — that is
// USP-3's job, built on top of this record).
const AssessmentStatusDraft = "draft"

var (
	// ErrAssessmentNotFound indicates no assessment was found for the given criteria.
	ErrAssessmentNotFound = errors.New("assessment not found")
	// ErrAssessmentOwnerRequired prevents creation without a trusted owner.
	ErrAssessmentOwnerRequired = errors.New("assessment owner required")
	// ErrAssessmentScenarioRequired requires at least a baseline threat scenario.
	ErrAssessmentScenarioRequired = errors.New("assessment requires an approved baseline scenario")
	// ErrAssessmentScenarioNotApproved rejects referencing a scenario a
	// reviewer hasn't signed off on yet.
	ErrAssessmentScenarioNotApproved = errors.New("referenced scenario is not approved")
	// ErrAssessmentScenarioKindMismatch rejects assigning a benign-kind
	// scenario as the baseline/follow-up (threat) role, or vice versa.
	ErrAssessmentScenarioKindMismatch = errors.New("scenario kind does not match its assessment role")
)

// Assessment is a measurement-exercise definition per the USP-1 spec
// (docs/superpowers/specs/2026-09-06-usp-measurement-spec.md §1): a
// baseline phase and an optional follow-up phase, each using a distinct
// scenario variant so follow-up measures transfer of a skill rather than
// memorization of the original message, plus an optional benign control
// scenario for the Discrimination metric.
//
// This record defines WHAT an assessment measures. It intentionally does
// not yet say WHO is in it or WHEN it runs — cohort assignment, campaign
// linkage, and launch timing are USP-3's job, built on top of this ID.
type Assessment struct {
	ID                     int64     `json:"id" gorm:"column:id; primary_key:yes"`
	OwnerID                int64     `json:"-" gorm:"column:owner_id; index"`
	Name                   string    `json:"name" gorm:"column:name; not null"`
	SkillTag               string    `json:"skill_tag" gorm:"column:skill_tag; not null"`
	BaselineScenarioID     int64     `json:"baseline_scenario_id" gorm:"column:baseline_scenario_id; not null"`
	FollowupScenarioID     int64     `json:"followup_scenario_id" gorm:"column:followup_scenario_id"`
	BenignScenarioID       int64     `json:"benign_scenario_id" gorm:"column:benign_scenario_id"`
	ObservationWindowHours int64     `json:"observation_window_hours" gorm:"column:observation_window_hours; not null"`
	Status                 string    `json:"status" gorm:"column:status; not null"`
	CreatedAt              time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName specifies the table name for the Assessment model.
func (Assessment) TableName() string {
	return "assessments"
}

// requireApprovedScenarioOfKind loads a scenario the caller intends to use
// in the given role and confirms it belongs to ownerID, is approved, and
// matches the expected kind (threat for baseline/follow-up, benign for the
// optional control) — this is the one place USP-2 enforces that only a
// human-approved, mechanically-sanitized scenario can ever be attached to
// an assessment.
func requireApprovedScenarioOfKind(id, ownerID int64, wantKind string) error {
	if id <= 0 {
		return nil // optional references (FollowupScenarioID, BenignScenarioID) may be unset
	}
	s, err := GetScenarioByID(id, ownerID)
	if err != nil {
		return err
	}
	if s.Status != ScenarioStatusApproved {
		return ErrAssessmentScenarioNotApproved
	}
	if s.Kind != wantKind {
		return ErrAssessmentScenarioKindMismatch
	}
	return nil
}

// CreateAssessment saves a new assessment definition after validating
// ownership and that every referenced scenario is owned, approved, and of
// the correct kind for its role. A baseline (threat) scenario is required;
// follow-up and benign-control scenarios are optional at creation time.
func CreateAssessment(a *Assessment) error {
	if a.OwnerID <= 0 {
		return ErrAssessmentOwnerRequired
	}
	if _, err := GetUser(a.OwnerID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrAssessmentOwnerRequired
		}
		return err
	}
	if a.BaselineScenarioID <= 0 {
		return ErrAssessmentScenarioRequired
	}
	if err := requireApprovedScenarioOfKind(a.BaselineScenarioID, a.OwnerID, ScenarioKindThreat); err != nil {
		return err
	}
	if err := requireApprovedScenarioOfKind(a.FollowupScenarioID, a.OwnerID, ScenarioKindThreat); err != nil {
		return err
	}
	if err := requireApprovedScenarioOfKind(a.BenignScenarioID, a.OwnerID, ScenarioKindBenign); err != nil {
		return err
	}
	if a.ObservationWindowHours <= 0 {
		a.ObservationWindowHours = 72 // matches the USP-1 spec's worked example default
	}
	a.Status = AssessmentStatusDraft
	a.CreatedAt = time.Now().UTC()
	err := db.Create(a).Error
	if err != nil {
		log.Errorf("error creating assessment: %v", err)
	}
	return err
}

// GetAssessments returns every assessment owned by ownerID, most recent first.
func GetAssessments(ownerID int64) ([]Assessment, error) {
	assessments := []Assessment{}
	err := db.Where("owner_id = ? AND owner_id > 0", ownerID).Order("created_at desc").Find(&assessments).Error
	if err != nil {
		log.Errorf("error getting assessments: %v", err)
	}
	return assessments, err
}

// GetAssessmentByID retrieves a single assessment by ID, scoped to ownerID.
func GetAssessmentByID(id, ownerID int64) (Assessment, error) {
	a := Assessment{}
	err := db.Where("id = ? AND owner_id = ? AND owner_id > 0", id, ownerID).First(&a).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return a, ErrAssessmentNotFound
		}
		log.Errorf("error getting assessment by id: %v", err)
	}
	return a, err
}
