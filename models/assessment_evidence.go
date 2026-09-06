package models

import (
	"time"

	"github.com/fir3storm/AwareNow/assessment"
)

// EvidenceBundleVersion is bumped whenever PhaseEvidence/EvidenceBundle's
// shape changes in a way that would break independent recomputation from a
// previously-exported bundle. The plan requires "independent recomputation
// from exported aggregate counts must match the UI" — a version field lets
// a consumer detect a shape they don't understand instead of silently
// misreading it.
const EvidenceBundleVersion = 1

// PhaseEvidence is the computed USP-1 metrics for one linked assessment
// phase, derived from that phase's campaign's real Results/Events. Exactly
// one of the threat-only fields (Recognition/Recovery/Speed) or the
// benign-only field (Discrimination) is populated, matching which kind of
// scenario this phase actually uses — never both, since a single campaign
// is never simultaneously a threat and a benign-control send.
type PhaseEvidence struct {
	Phase          string                  `json:"phase"`
	CampaignID     int64                   `json:"campaign_id"`
	Recognition    *assessment.Proportion  `json:"recognition,omitempty"`
	Recovery       *assessment.Proportion  `json:"recovery,omitempty"`
	Speed          *assessment.SpeedResult `json:"speed,omitempty"`
	Discrimination *assessment.Proportion  `json:"discrimination,omitempty"`
}

// EvidenceBundle is the full, versioned evidence export for one assessment:
// its definition plus every linked phase's computed metrics. This is
// deliberately the same structure whether serialized to JSON for download
// or rendered in the React evidence view, per the plan's "independent
// recomputation... must match the UI" requirement — there is exactly one
// code path that produces these numbers, not a UI-side recalculation.
type EvidenceBundle struct {
	BundleVersion          int             `json:"bundle_version"`
	GeneratedAt            time.Time       `json:"generated_at"`
	Assessment             Assessment      `json:"assessment"`
	ObservationWindowHours int64           `json:"observation_window_hours"`
	Phases                 []PhaseEvidence `json:"phases"`
	// Limitations is fixed, static text (not computed from data) stating
	// what this bundle does NOT claim, per the plan's explicit
	// requirement that a before/after change alone is observational and
	// that causal wording requires a justified comparison design this
	// package does not attempt to provide.
	Limitations []string `json:"limitations"`
}

// standardLimitations is deliberately not configurable per-assessment —
// every bundle carries the same disclosures regardless of how good the
// numbers look, so a reviewer cannot quietly drop them for a favorable
// result.
func standardLimitations() []string {
	return []string{
		"This is an observational comparison, not a controlled experiment. A change between baseline and follow-up phases does not by itself establish that any specific intervention caused it.",
		"Send-attempt time (SendDate) is used for all timing calculations; this application does not have confirmed-delivery or read-receipt data, so time-to-report and eligibility are measured from send attempt, not confirmed inbox delivery.",
		"Email-open tracking undercounts actual opens (e.g. image-blocking clients) and is not used as a substitute for delivery confirmation anywhere in these metrics.",
		"Proportions computed from small cohorts are marked insufficient data below the documented threshold rather than shown as a percentage; even above that threshold, the reported confidence interval should be read alongside the rate, not the rate alone.",
		"No automated scanner-activity filtering has been applied to these numbers; raw results may include non-human interactions (security scanners, link-preview services).",
	}
}

// recipientsFromCampaign translates one campaign's real Results/Events
// into assessment.Recipient values for the given scenario kind. A result
// counts as Sent only if an actual EventSent row exists for that
// recipient's email — a Result row alone does not guarantee the send
// succeeded (see EventSendingError).
func recipientsFromCampaign(c Campaign, kind assessment.ScenarioKind) []assessment.Recipient {
	recipients := make([]assessment.Recipient, 0, len(c.Results))
	for _, r := range c.Results {
		sent := false
		var events []assessment.Event
		for _, e := range c.Events {
			if e.Email != r.Email {
				continue
			}
			var ek assessment.EventKind
			switch e.Message {
			case EventSent:
				sent = true
				continue
			case EventOpened:
				ek = assessment.EventOpened
			case EventClicked:
				ek = assessment.EventClicked
			case EventDataSubmit:
				ek = assessment.EventDataSubmit
			case EventReported:
				ek = assessment.EventReported
			default:
				continue
			}
			events = append(events, assessment.Event{Kind: ek, Offset: e.Time.Sub(r.SendDate)})
		}
		if !sent || r.SendDate.IsZero() {
			recipients = append(recipients, assessment.Recipient{Sent: false, Scenario: kind})
			continue
		}
		recipients = append(recipients, assessment.Recipient{Sent: true, Scenario: kind, Events: events})
	}
	return recipients
}

// scenarioKindForPhase maps an AssessmentPhase's Phase string to the
// assessment.ScenarioKind it must use, matching the invariant
// CreateAssessment already enforces (baseline/followup reference a
// ScenarioKindThreat scenario; benign_control references
// ScenarioKindBenign).
func scenarioKindForPhase(phase string) assessment.ScenarioKind {
	if phase == PhaseBenignControl {
		return assessment.ScenarioBenign
	}
	return assessment.ScenarioThreat
}

// computePhaseEvidence loads phaseRow's linked campaign and computes the
// metrics appropriate to its scenario kind.
func computePhaseEvidence(phaseRow AssessmentPhase, ownerID int64, window time.Duration) (PhaseEvidence, error) {
	c, err := GetCampaign(phaseRow.CampaignID, ownerID)
	if err != nil {
		return PhaseEvidence{}, err
	}
	kind := scenarioKindForPhase(phaseRow.Phase)
	recipients := recipientsFromCampaign(c, kind)

	ev := PhaseEvidence{Phase: phaseRow.Phase, CampaignID: c.Id}
	if kind == assessment.ScenarioBenign {
		d := assessment.Discrimination(recipients, window)
		ev.Discrimination = &d
		return ev, nil
	}
	rec := assessment.Recognition(recipients, window)
	rcv := assessment.Recovery(recipients, window)
	sp := assessment.Speed(recipients, window)
	ev.Recognition = &rec
	ev.Recovery = &rcv
	ev.Speed = &sp
	return ev, nil
}

// GetAssessmentEvidence builds the full evidence bundle for an assessment:
// its definition plus computed metrics for every phase that has actually
// been linked to a campaign so far (a phase with no linked campaign is
// silently omitted, not an error — an assessment may still be in progress
// with only its baseline phase linked). Returns ErrAssessmentNotFound if
// the assessment doesn't exist or isn't owned by ownerID.
func GetAssessmentEvidence(assessmentID, ownerID int64) (EvidenceBundle, error) {
	a, err := GetAssessmentByID(assessmentID, ownerID)
	if err != nil {
		return EvidenceBundle{}, err
	}
	linked, err := GetAssessmentPhases(assessmentID, ownerID)
	if err != nil {
		return EvidenceBundle{}, err
	}

	window := time.Duration(a.ObservationWindowHours) * time.Hour
	bundle := EvidenceBundle{
		BundleVersion:          EvidenceBundleVersion,
		GeneratedAt:            time.Now().UTC(),
		Assessment:             a,
		ObservationWindowHours: a.ObservationWindowHours,
		Phases:                 make([]PhaseEvidence, 0, len(linked)),
		Limitations:            standardLimitations(),
	}
	for _, phaseRow := range linked {
		ev, err := computePhaseEvidence(phaseRow, ownerID, window)
		if err != nil {
			return EvidenceBundle{}, err
		}
		bundle.Phases = append(bundle.Phases, ev)
	}
	return bundle, nil
}
