package models

import (
	"errors"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/jinzhu/gorm"
)

// PhaseBaseline, PhaseFollowup, and PhaseBenignControl are the only valid
// AssessmentPhase.Phase values, corresponding to Assessment's
// BaselineScenarioID/FollowupScenarioID/BenignScenarioID roles.
const (
	PhaseBaseline      = "baseline"
	PhaseFollowup      = "followup"
	PhaseBenignControl = "benign_control"
)

var (
	// ErrAssessmentPhaseInvalid rejects a Phase value other than the three
	// constants above.
	ErrAssessmentPhaseInvalid = errors.New("assessment phase must be \"baseline\", \"followup\", or \"benign_control\"")
	// ErrAssessmentPhaseScenarioNotConfigured rejects linking a followup or
	// benign_control phase when the parent Assessment never had that
	// scenario role set at creation time.
	ErrAssessmentPhaseScenarioNotConfigured = errors.New("assessment has no scenario configured for this phase")
	// ErrAssessmentPhaseCampaignNotFound rejects a CampaignID that doesn't
	// exist or doesn't belong to the same owner as the assessment.
	ErrAssessmentPhaseCampaignNotFound = errors.New("campaign not found")
)

// AssessmentPhase links one phase of an Assessment (USP-2) to an
// already-created, existing Campaign that will actually deliver that
// phase's scenario to a cohort. This package deliberately never creates or
// launches a Campaign itself — per the plan's explicit "no autonomous
// campaign launch" requirement, an administrator creates the real Campaign
// through the ordinary campaign flow (its own recipient groups ARE the
// cohort — this repo does not duplicate Group/Target as a separate
// "cohort" concept) and then links it here. USP-1's metrics are computed
// from that Campaign's existing Results/Events once it has actually run;
// this record is what tells USP-4 which Campaign corresponds to which
// phase of which Assessment.
type AssessmentPhase struct {
	ID           int64     `json:"id" gorm:"column:id; primary_key:yes"`
	OwnerID      int64     `json:"-" gorm:"column:owner_id; index"`
	AssessmentID int64     `json:"assessment_id" gorm:"column:assessment_id; index; not null"`
	Phase        string    `json:"phase" gorm:"column:phase; not null"`
	CampaignID   int64     `json:"campaign_id" gorm:"column:campaign_id; not null"`
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName specifies the table name for the AssessmentPhase model.
func (AssessmentPhase) TableName() string {
	return "assessment_phases"
}

func validPhase(phase string) bool {
	switch phase {
	case PhaseBaseline, PhaseFollowup, PhaseBenignControl:
		return true
	default:
		return false
	}
}

// phaseScenarioConfigured reports whether the parent assessment has a
// scenario assigned to the given role at all (regardless of that
// scenario's approval status, which was already enforced when the
// Assessment itself was created).
func phaseScenarioConfigured(a Assessment, phase string) bool {
	switch phase {
	case PhaseBaseline:
		return a.BaselineScenarioID > 0
	case PhaseFollowup:
		return a.FollowupScenarioID > 0
	case PhaseBenignControl:
		return a.BenignScenarioID > 0
	default:
		return false
	}
}

// LinkAssessmentPhase records that the given Campaign delivers the given
// phase of the given assessment. It is safe to call again for the same
// (AssessmentID, Phase) pair — per the plan's "repeatable assignment"
// requirement, a retry (or an admin correcting a mistaken CampaignID)
// updates the existing row's CampaignID rather than creating a duplicate
// or erroring.
func LinkAssessmentPhase(ownerID, assessmentID int64, phase string, campaignID int64) (AssessmentPhase, error) {
	if !validPhase(phase) {
		return AssessmentPhase{}, ErrAssessmentPhaseInvalid
	}
	a, err := GetAssessmentByID(assessmentID, ownerID)
	if err != nil {
		return AssessmentPhase{}, err // ErrAssessmentNotFound already covers ownership
	}
	if !phaseScenarioConfigured(a, phase) {
		return AssessmentPhase{}, ErrAssessmentPhaseScenarioNotConfigured
	}
	if _, err := GetCampaign(campaignID, ownerID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return AssessmentPhase{}, ErrAssessmentPhaseCampaignNotFound
		}
		return AssessmentPhase{}, err
	}

	existing := AssessmentPhase{}
	lookupErr := db.Where("owner_id = ? AND assessment_id = ? AND phase = ?", ownerID, assessmentID, phase).First(&existing).Error
	now := time.Now().UTC()
	if lookupErr == nil {
		existing.CampaignID = campaignID
		existing.UpdatedAt = now
		if err := db.Save(&existing).Error; err != nil {
			log.Errorf("error updating assessment phase link: %v", err)
			return AssessmentPhase{}, err
		}
		return existing, nil
	}
	if lookupErr != gorm.ErrRecordNotFound {
		log.Errorf("error looking up assessment phase: %v", lookupErr)
		return AssessmentPhase{}, lookupErr
	}

	created := AssessmentPhase{
		OwnerID:      ownerID,
		AssessmentID: assessmentID,
		Phase:        phase,
		CampaignID:   campaignID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&created).Error; err != nil {
		// The (assessment_id, phase) unique index exists specifically to
		// catch two concurrent callers who both missed the SELECT above and
		// both tried to insert. Rather than parse a driver-specific
		// duplicate-key error string (which differs between sqlite3 and
		// mysql), re-run the lookup: if a row now exists, the other caller
		// won the race and this call becomes the "update" branch instead,
		// preserving the idempotent-retry guarantee this function promises.
		// If the row still doesn't exist, the failure was something else,
		// and the original error is what the caller needs to see.
		var raced AssessmentPhase
		if lookupErr := db.Where("owner_id = ? AND assessment_id = ? AND phase = ?", ownerID, assessmentID, phase).First(&raced).Error; lookupErr == nil {
			raced.CampaignID = campaignID
			raced.UpdatedAt = time.Now().UTC()
			if saveErr := db.Save(&raced).Error; saveErr != nil {
				log.Errorf("error updating assessment phase link after create race: %v", saveErr)
				return AssessmentPhase{}, saveErr
			}
			return raced, nil
		}
		log.Errorf("error creating assessment phase link: %v", err)
		return AssessmentPhase{}, err
	}
	return created, nil
}

// GetAssessmentPhases returns every phase linked so far for the given
// assessment, owned by ownerID.
func GetAssessmentPhases(assessmentID, ownerID int64) ([]AssessmentPhase, error) {
	phases := []AssessmentPhase{}
	err := db.Where("owner_id = ? AND assessment_id = ? AND owner_id > 0", ownerID, assessmentID).
		Order("created_at asc").Find(&phases).Error
	if err != nil {
		log.Errorf("error getting assessment phases: %v", err)
	}
	return phases, err
}
