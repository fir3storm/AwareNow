package models

import (
	"gopkg.in/check.v1"
)

// sendEventsForRecipient looks up the campaign's Result for recipientIndex
// (order matches c.Groups[0].Targets, per createCampaign's fixture of 4
// targets test1-4@example.com) and records the given sequence of events for
// it via Result.createEvent, mirroring how production code itself records
// send/behavior events (see models/result.go's HandleEmailSent and
// friends). The campaign's Results already carry a non-zero SendDate from
// PostCampaign, so recording just the events here is sufficient for
// recipientsFromCampaign to treat the recipient as sent.
func (s *ModelsSuite) sendEventsForRecipient(c *check.C, campaign Campaign, recipientIndex int, statuses ...string) {
	rid := campaign.Results[recipientIndex].RId
	for _, status := range statuses {
		r, err := GetResult(rid)
		c.Assert(err, check.IsNil)
		_, err = r.createEvent(status, nil)
		c.Assert(err, check.IsNil)
	}
}

func (s *ModelsSuite) TestGetAssessmentEvidenceThreatPhase(c *check.C) {
	campaign := s.createCampaign(c)
	c.Assert(campaign.Results, check.HasLen, 4)

	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "Threat phase evidence", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)

	_, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, campaign.Id)
	c.Assert(err, check.IsNil)

	// Recipient 0: sent, then recognized (reported).
	s.sendEventsForRecipient(c, campaign, 0, EventSent, EventReported)
	// Recipient 1: sent, then clicked (not recognized, no report).
	s.sendEventsForRecipient(c, campaign, 1, EventSent, EventClicked)
	// Recipients 2 and 3: sent only (eligible, nonresponse).
	s.sendEventsForRecipient(c, campaign, 2, EventSent)
	s.sendEventsForRecipient(c, campaign, 3, EventSent)

	bundle, err := GetAssessmentEvidence(a.ID, 1)
	c.Assert(err, check.IsNil)

	c.Assert(bundle.BundleVersion, check.Equals, EvidenceBundleVersion)
	c.Assert(len(bundle.Limitations) > 0, check.Equals, true)
	c.Assert(bundle.Phases, check.HasLen, 1)

	phase := bundle.Phases[0]
	c.Assert(phase.Phase, check.Equals, PhaseBaseline)
	c.Assert(phase.CampaignID, check.Equals, campaign.Id)

	c.Assert(phase.Recognition, check.NotNil)
	c.Assert(phase.Recognition.Denominator, check.Equals, 4)
	c.Assert(phase.Recognition.Numerator, check.Equals, 1)

	c.Assert(phase.Recovery, check.NotNil)

	c.Assert(phase.Speed, check.NotNil)
	c.Assert(phase.Speed.Eligible, check.Equals, 4)

	c.Assert(phase.Discrimination, check.IsNil)
}

func (s *ModelsSuite) TestGetAssessmentEvidenceBenignPhase(c *check.C) {
	// Use the standard 4-recipient fixture so denominators match the rest
	// of this file's conventions.
	campaign := s.createCampaign(c)
	c.Assert(campaign.Results, check.HasLen, 4)

	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	benign := s.createApprovedScenario(c, 1, ScenarioKindBenign)
	a := Assessment{
		OwnerID:            1,
		Name:               "Benign phase evidence",
		SkillTag:           "test",
		BaselineScenarioID: baseline.ID,
		BenignScenarioID:   benign.ID,
	}
	c.Assert(CreateAssessment(&a), check.IsNil)

	_, err := LinkAssessmentPhase(1, a.ID, PhaseBenignControl, campaign.Id)
	c.Assert(err, check.IsNil)

	// Recipient 0 reports the benign message as if it were a threat
	// (a false positive); recipients 1-3 are sent but take no action.
	s.sendEventsForRecipient(c, campaign, 0, EventSent, EventReported)
	s.sendEventsForRecipient(c, campaign, 1, EventSent)
	s.sendEventsForRecipient(c, campaign, 2, EventSent)
	s.sendEventsForRecipient(c, campaign, 3, EventSent)

	bundle, err := GetAssessmentEvidence(a.ID, 1)
	c.Assert(err, check.IsNil)

	var benignPhase *PhaseEvidence
	for i := range bundle.Phases {
		if bundle.Phases[i].Phase == PhaseBenignControl {
			benignPhase = &bundle.Phases[i]
		}
	}
	c.Assert(benignPhase, check.NotNil)

	c.Assert(benignPhase.Discrimination, check.NotNil)
	c.Assert(benignPhase.Discrimination.Numerator, check.Equals, 1)
	c.Assert(benignPhase.Discrimination.Denominator, check.Equals, 4)

	c.Assert(benignPhase.Recognition, check.IsNil)
	c.Assert(benignPhase.Recovery, check.IsNil)
	c.Assert(benignPhase.Speed, check.IsNil)
}

func (s *ModelsSuite) TestGetAssessmentEvidenceOmitsUnlinkedPhases(c *check.C) {
	campaign := s.createCampaign(c)

	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	followup := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{
		OwnerID:            1,
		Name:               "Unlinked followup",
		SkillTag:           "test",
		BaselineScenarioID: baseline.ID,
		FollowupScenarioID: followup.ID,
	}
	c.Assert(CreateAssessment(&a), check.IsNil)

	_, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, campaign.Id)
	c.Assert(err, check.IsNil)
	// Followup is intentionally never linked to a campaign.

	s.sendEventsForRecipient(c, campaign, 0, EventSent)
	s.sendEventsForRecipient(c, campaign, 1, EventSent)
	s.sendEventsForRecipient(c, campaign, 2, EventSent)
	s.sendEventsForRecipient(c, campaign, 3, EventSent)

	bundle, err := GetAssessmentEvidence(a.ID, 1)
	c.Assert(err, check.IsNil)
	c.Assert(bundle.Phases, check.HasLen, 1)
	c.Assert(bundle.Phases[0].Phase, check.Equals, PhaseBaseline)
}

func (s *ModelsSuite) TestGetAssessmentEvidenceOwnershipIsolation(c *check.C) {
	campaign := s.createCampaign(c)

	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "Ownership isolation evidence", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)

	_, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, campaign.Id)
	c.Assert(err, check.IsNil)

	otherUser := User{Username: "evidence-ownership-other", Hash: "12345", ApiKey: "evidence-ownership-other-key"}
	c.Assert(PutUser(&otherUser), check.IsNil)
	defer db.Delete(&otherUser)

	_, err = GetAssessmentEvidence(a.ID, otherUser.Id)
	c.Assert(err, check.Equals, ErrAssessmentNotFound)
}

func (s *ModelsSuite) TestGetAssessmentEvidenceUnsentRecipientsExcluded(c *check.C) {
	campaign := s.createCampaign(c)

	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "Unsent excluded", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)

	_, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, campaign.Id)
	c.Assert(err, check.IsNil)

	// Recipients 0-2 are actually sent; recipient 3 gets no events at all
	// (never sent) and must not count toward the denominator.
	s.sendEventsForRecipient(c, campaign, 0, EventSent, EventReported)
	s.sendEventsForRecipient(c, campaign, 1, EventSent)
	s.sendEventsForRecipient(c, campaign, 2, EventSent)

	bundle, err := GetAssessmentEvidence(a.ID, 1)
	c.Assert(err, check.IsNil)
	c.Assert(bundle.Phases, check.HasLen, 1)

	phase := bundle.Phases[0]
	c.Assert(phase.Recognition, check.NotNil)
	c.Assert(phase.Recognition.Denominator, check.Equals, 3)
	c.Assert(phase.Recognition.Numerator, check.Equals, 1)
}
