package models

import (
	"gopkg.in/check.v1"
)

// createCampaignForOwner builds a full Group/Template/Page/SMTP/Campaign
// fixture owned by ownerID and launches it, mirroring
// ModelsSuite.createCampaign/createCampaignDependencies in models_test.go,
// but parameterized so tests can exercise cross-owner isolation (those
// helpers hardcode owner 1).
func (s *ModelsSuite) createCampaignForOwner(ch *check.C, ownerID int64, name string) Campaign {
	group := Group{Name: name + " Group"}
	group.Targets = []Target{
		Target{BaseRecipient: BaseRecipient{Email: "phase-test1@example.com", FirstName: "First", LastName: "Example"}},
	}
	group.UserId = ownerID
	ch.Assert(PostGroup(&group), check.Equals, nil)

	t := Template{Name: name + " Template"}
	t.Subject = "{{.RId}} - Subject"
	t.Text = "{{.RId}} - Text"
	t.HTML = "{{.RId}} - HTML"
	t.UserId = ownerID
	ch.Assert(PostTemplate(&t), check.Equals, nil)

	p := Page{Name: name + " Page"}
	p.HTML = "<html>Test</html>"
	p.UserId = ownerID
	ch.Assert(PostPage(&p), check.Equals, nil)

	smtp := SMTP{Name: name + " SMTP"}
	smtp.UserId = ownerID
	smtp.Host = "example.com"
	smtp.FromAddress = "test@test.com"
	ch.Assert(PostSMTP(&smtp), check.Equals, nil)

	c := Campaign{Name: name}
	c.UserId = ownerID
	c.Template = t
	c.Page = p
	c.SMTP = smtp
	c.Groups = []Group{group}
	ch.Assert(PostCampaign(&c, c.UserId), check.Equals, nil)

	c, err := GetCampaign(c.Id, c.UserId)
	ch.Assert(err, check.IsNil)
	return c
}

func (s *ModelsSuite) TestLinkAssessmentPhase(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "Phase link", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)

	campaign := s.createCampaignForOwner(c, 1, "Phase Link Campaign")

	phase, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, campaign.Id)
	c.Assert(err, check.IsNil)
	c.Assert(phase.CampaignID, check.Equals, campaign.Id)
	c.Assert(phase.AssessmentID, check.Equals, a.ID)
	c.Assert(phase.Phase, check.Equals, PhaseBaseline)
}

func (s *ModelsSuite) TestLinkAssessmentPhaseIsIdempotent(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "Phase idempotent", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)

	campaign1 := s.createCampaignForOwner(c, 1, "Phase Idempotent Campaign 1")
	campaign2 := s.createCampaignForOwner(c, 1, "Phase Idempotent Campaign 2")

	first, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, campaign1.Id)
	c.Assert(err, check.IsNil)
	c.Assert(first.CampaignID, check.Equals, campaign1.Id)

	second, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, campaign2.Id)
	c.Assert(err, check.IsNil)
	c.Assert(second.CampaignID, check.Equals, campaign2.Id)
	c.Assert(second.ID, check.Equals, first.ID)

	phases, err := GetAssessmentPhases(a.ID, 1)
	c.Assert(err, check.IsNil)
	c.Assert(phases, check.HasLen, 1)
	c.Assert(phases[0].CampaignID, check.Equals, campaign2.Id)
}

func (s *ModelsSuite) TestLinkAssessmentPhaseInvalidPhase(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "Phase invalid", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)

	campaign := s.createCampaignForOwner(c, 1, "Phase Invalid Campaign")

	_, err := LinkAssessmentPhase(1, a.ID, "bogus", campaign.Id)
	c.Assert(err, check.Equals, ErrAssessmentPhaseInvalid)
}

func (s *ModelsSuite) TestLinkAssessmentPhaseScenarioNotConfigured(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "No followup configured", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)
	c.Assert(a.FollowupScenarioID, check.Equals, int64(0))

	campaign := s.createCampaignForOwner(c, 1, "Phase Not Configured Campaign")

	_, err := LinkAssessmentPhase(1, a.ID, PhaseFollowup, campaign.Id)
	c.Assert(err, check.Equals, ErrAssessmentPhaseScenarioNotConfigured)
}

func (s *ModelsSuite) TestLinkAssessmentPhaseCampaignNotFound(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "Campaign not found", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)

	// A campaign ID that doesn't exist at all.
	_, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, 999999)
	c.Assert(err, check.Equals, ErrAssessmentPhaseCampaignNotFound)

	// A campaign that exists but belongs to a different owner.
	otherUser := User{Username: "phase-cross-owner", Hash: "12345", ApiKey: "phase-cross-owner-key"}
	c.Assert(PutUser(&otherUser), check.IsNil)
	defer db.Delete(&otherUser)
	otherCampaign := s.createCampaignForOwner(c, otherUser.Id, "Phase Cross Owner Campaign")

	_, err = LinkAssessmentPhase(1, a.ID, PhaseBaseline, otherCampaign.Id)
	c.Assert(err, check.Equals, ErrAssessmentPhaseCampaignNotFound)
}

func (s *ModelsSuite) TestLinkAssessmentPhaseOwnershipIsolation(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	a := Assessment{OwnerID: 1, Name: "Ownership isolation", SkillTag: "test", BaselineScenarioID: baseline.ID}
	c.Assert(CreateAssessment(&a), check.IsNil)

	campaign := s.createCampaignForOwner(c, 1, "Phase Ownership Campaign")

	_, err := LinkAssessmentPhase(1, a.ID, PhaseBaseline, campaign.Id)
	c.Assert(err, check.IsNil)

	otherUser := User{Username: "phase-ownership-other", Hash: "12345", ApiKey: "phase-ownership-other-key"}
	c.Assert(PutUser(&otherUser), check.IsNil)
	defer db.Delete(&otherUser)

	phases, err := GetAssessmentPhases(a.ID, otherUser.Id)
	c.Assert(err, check.IsNil)
	c.Assert(phases, check.HasLen, 0)
}
