package models

import (
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) createApprovedScenario(c *check.C, ownerID int64, kind string) Scenario {
	sc := Scenario{OwnerID: ownerID, Name: "Scenario " + kind, SkillTag: "test", Kind: kind}
	c.Assert(CreateScenario(&sc), check.IsNil)
	c.Assert(ApproveScenario(sc.ID, ownerID, "admin"), check.IsNil)
	got, err := GetScenarioByID(sc.ID, ownerID)
	c.Assert(err, check.IsNil)
	return got
}

func (s *ModelsSuite) TestCreateAndGetAssessment(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)

	a := Assessment{
		OwnerID:            1,
		Name:               "Invoice fraud recognition",
		SkillTag:           "invoice-fraud",
		BaselineScenarioID: baseline.ID,
	}
	c.Assert(CreateAssessment(&a), check.IsNil)
	c.Assert(a.ID, check.Not(check.Equals), int64(0))
	c.Assert(a.Status, check.Equals, AssessmentStatusDraft)
	c.Assert(a.ObservationWindowHours, check.Equals, int64(72))

	got, err := GetAssessmentByID(a.ID, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.Name, check.Equals, a.Name)

	assessments, err := GetAssessments(1)
	c.Assert(err, check.IsNil)
	c.Assert(assessments, check.HasLen, 1)
}

func (s *ModelsSuite) TestCreateAssessmentBaselineRequired(c *check.C) {
	a := Assessment{OwnerID: 1, Name: "No baseline", SkillTag: "test"}
	c.Assert(CreateAssessment(&a), check.Equals, ErrAssessmentScenarioRequired)
}

func (s *ModelsSuite) TestCreateAssessmentBaselineNotApproved(c *check.C) {
	draft := Scenario{OwnerID: 1, Name: "Draft scenario", SkillTag: "test", Kind: ScenarioKindThreat}
	c.Assert(CreateScenario(&draft), check.IsNil)

	a := Assessment{OwnerID: 1, Name: "Uses draft", SkillTag: "test", BaselineScenarioID: draft.ID}
	c.Assert(CreateAssessment(&a), check.Equals, ErrAssessmentScenarioNotApproved)
}

func (s *ModelsSuite) TestCreateAssessmentBaselineKindMismatch(c *check.C) {
	benign := s.createApprovedScenario(c, 1, ScenarioKindBenign)

	a := Assessment{OwnerID: 1, Name: "Wrong kind baseline", SkillTag: "test", BaselineScenarioID: benign.ID}
	c.Assert(CreateAssessment(&a), check.Equals, ErrAssessmentScenarioKindMismatch)
}

func (s *ModelsSuite) TestCreateAssessmentBaselineOwnershipIsolation(c *check.C) {
	otherOwnerScenario := Scenario{OwnerID: 1, Name: "Owned by 1", SkillTag: "test", Kind: ScenarioKindThreat}
	c.Assert(CreateScenario(&otherOwnerScenario), check.IsNil)
	c.Assert(ApproveScenario(otherOwnerScenario.ID, 1, "admin"), check.IsNil)

	// A second, real user account so CreateAssessment's own owner-exists
	// check succeeds and the test actually exercises the cross-owner
	// scenario-reference isolation, not the owner-required guard.
	otherUser := User{Username: "assessment-cross-owner", Hash: "12345", ApiKey: "assessment-cross-owner-key"}
	c.Assert(PutUser(&otherUser), check.IsNil)
	defer db.Delete(&otherUser)

	// A different owner attempting to use owner 1's approved scenario as
	// baseline should see it as not found (requireApprovedScenarioOfKind
	// loads via GetScenarioByID, which is owner-scoped).
	a := Assessment{OwnerID: otherUser.Id, Name: "Cross-owner reference", SkillTag: "test", BaselineScenarioID: otherOwnerScenario.ID}
	err := CreateAssessment(&a)
	c.Assert(err, check.Equals, ErrScenarioNotFound)
}

func (s *ModelsSuite) TestCreateAssessmentWithBenignControl(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	benign := s.createApprovedScenario(c, 1, ScenarioKindBenign)

	a := Assessment{
		OwnerID:            1,
		Name:               "With benign control",
		SkillTag:           "test",
		BaselineScenarioID: baseline.ID,
		BenignScenarioID:   benign.ID,
	}
	c.Assert(CreateAssessment(&a), check.IsNil)
}

func (s *ModelsSuite) TestCreateAssessmentBenignKindMismatch(c *check.C) {
	baseline := s.createApprovedScenario(c, 1, ScenarioKindThreat)
	wrongKindBenign := s.createApprovedScenario(c, 1, ScenarioKindThreat)

	a := Assessment{
		OwnerID:            1,
		Name:               "Wrong kind benign",
		SkillTag:           "test",
		BaselineScenarioID: baseline.ID,
		BenignScenarioID:   wrongKindBenign.ID,
	}
	c.Assert(CreateAssessment(&a), check.Equals, ErrAssessmentScenarioKindMismatch)
}
