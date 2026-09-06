package models

import (
	"errors"
	"testing"

	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestCreateAndGetScenario(c *check.C) {
	sc := Scenario{
		OwnerID:  1,
		Name:     "Invoice phishing baseline",
		SkillTag: "invoice-fraud",
		Kind:     ScenarioKindThreat,
		Subject:  "Your invoice is overdue",
		HTML:     `<p>Please <a href="{{.URL}}">click here</a> to pay</p>`,
		Text:     "Please click here to pay",
	}
	c.Assert(CreateScenario(&sc), check.IsNil)
	c.Assert(sc.ID, check.Not(check.Equals), int64(0))
	c.Assert(sc.Status, check.Equals, ScenarioStatusDraft)
	c.Assert(sc.Version, check.Equals, int64(1))

	got, err := GetScenarioByID(sc.ID, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.Name, check.Equals, sc.Name)

	scenarios, err := GetScenarios(1)
	c.Assert(err, check.IsNil)
	c.Assert(scenarios, check.HasLen, 1)
}

func (s *ModelsSuite) TestCreateScenarioOwnerRequired(c *check.C) {
	c.Assert(CreateScenario(&Scenario{Kind: ScenarioKindThreat}), check.Equals, ErrScenarioOwnerRequired)
	c.Assert(CreateScenario(&Scenario{OwnerID: 99999, Kind: ScenarioKindThreat}), check.Equals, ErrScenarioOwnerRequired)
}

func (s *ModelsSuite) TestCreateScenarioInvalidKind(c *check.C) {
	sc := Scenario{OwnerID: 1, Name: "Bad kind", SkillTag: "test", Kind: "not-a-kind"}
	c.Assert(CreateScenario(&sc), check.Equals, ErrScenarioInvalidKind)
}

func (s *ModelsSuite) TestCreateScenarioUnsafeContent(c *check.C) {
	sc := Scenario{
		OwnerID:  1,
		Name:     "Unsafe scenario",
		SkillTag: "test",
		Kind:     ScenarioKindThreat,
		HTML:     `<p>Hello</p><script>alert(1)</script>`,
	}
	err := CreateScenario(&sc)
	c.Assert(err, check.NotNil)
	c.Assert(errors.Is(err, ErrScenarioUnsafeContent), check.Equals, true)
}

func (s *ModelsSuite) TestApproveScenario(c *check.C) {
	sc := Scenario{OwnerID: 1, Name: "To approve", SkillTag: "test", Kind: ScenarioKindThreat}
	c.Assert(CreateScenario(&sc), check.IsNil)

	c.Assert(ApproveScenario(sc.ID, 1, "admin"), check.IsNil)

	got, err := GetScenarioByID(sc.ID, 1)
	c.Assert(err, check.IsNil)
	c.Assert(got.Status, check.Equals, ScenarioStatusApproved)
	c.Assert(got.ReviewedBy, check.Equals, "admin")
	c.Assert(got.ReviewedAt.IsZero(), check.Equals, false)
}

func (s *ModelsSuite) TestApproveScenarioNotFound(c *check.C) {
	c.Assert(ApproveScenario(99999, 1, "admin"), check.Equals, ErrScenarioNotFound)
}

func (s *ModelsSuite) TestScenarioOwnership(c *check.C) {
	sc := Scenario{OwnerID: 1, Name: "Owned by 1", SkillTag: "test", Kind: ScenarioKindThreat}
	c.Assert(CreateScenario(&sc), check.IsNil)

	_, err := GetScenarioByID(sc.ID, 2)
	c.Assert(err, check.Equals, ErrScenarioNotFound)

	scenarios, err := GetScenarios(2)
	c.Assert(err, check.IsNil)
	c.Assert(scenarios, check.HasLen, 0)

	c.Assert(ApproveScenario(sc.ID, 2, "other"), check.Equals, ErrScenarioNotFound)
}

// TestValidateScenarioSafety is a plain testing.T function (not a
// ModelsSuite/gocheck method) since ValidateScenarioSafety is pure and
// needs no database fixture. It covers the original safe/unsafe cases plus
// three real bypasses (base-tag anchor retargeting, <style> CSS url(), and
// <img srcset>) found in an internal security review on 2026-09-07 and
// fixed in the same change — see the doc comment on ValidateScenarioSafety
// itself for the incident note. Do not remove these cases when refactoring
// the function; they are regression tests for a previously-exploitable gap.
func TestValidateScenarioSafety(t *testing.T) {
	cases := []struct {
		name    string
		html    string
		wantErr bool
	}{
		{"safe placeholder link", `<p>Click <a href="{{.URL}}">here</a></p>`, false},
		{"safe anchor", `<a href="#section">jump</a>`, false},
		{"safe data img", `<img src="data:image/png;base64,abc">`, false},
		{"empty html", ``, false},
		{"external link", `<a href="http://evil.example">click</a>`, true},
		{"script tag", `<script>alert(1)</script>`, true},
		{"iframe", `<iframe src="http://evil.example"></iframe>`, true},
		{"external img", `<img src="http://evil.example/track.png">`, true},
		{"form action", `<form action="http://evil.example/collect"></form>`, true},
		{"onclick handler", `<a href="{{.URL}}" onclick="steal()">click</a>`, true},
		{"javascript href", `<a href="javascript:alert(1)">click</a>`, true},
		{"mailto href", `<a href="mailto:evil@example.com">click</a>`, true},
		{"base tag retargets anchor", `<base href="https://evil.example/"><a href="#track">Click here</a>`, true},
		{"style tag css url", `<style>body{background:url(https://evil.example/beacon.png)}</style>`, true},
		{"img srcset external", `<img src="{{.URL}}" srcset="https://evil.example/track.png 1x">`, true},
		{"inline style url", `<p style="background:url(https://evil.example/x.png)">x</p>`, true},
		{"meta refresh", `<meta http-equiv="refresh" content="0;url=https://evil.example">`, true},
		{"safe srcset placeholder", `<img src="{{.URL}}" srcset="{{.URL}} 1x">`, false},
	}
	for _, c := range cases {
		err := ValidateScenarioSafety(c.html)
		if c.wantErr && err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
		if !c.wantErr && err != nil {
			t.Errorf("%s: expected no error, got %v", c.name, err)
		}
	}
}
