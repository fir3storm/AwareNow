package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fir3storm/AwareNow/models"
)

// seedApprovedScenario creates and approves a threat-kind scenario directly
// via the models package so assessment tests can focus on the assessment
// handlers under test.
func seedApprovedScenario(t *testing.T) models.Scenario {
	s := models.Scenario{
		OwnerID:  1,
		Name:     "Baseline scenario",
		SkillTag: "link-recognition",
		Kind:     models.ScenarioKindThreat,
		Subject:  "Test",
		HTML:     `<p><a href="{{.URL}}">link</a></p>`,
	}
	if err := models.CreateScenario(&s); err != nil {
		t.Fatalf("error creating scenario: %v", err)
	}
	if err := models.ApproveScenario(s.ID, 1, "reviewer"); err != nil {
		t.Fatalf("error approving scenario: %v", err)
	}
	s, err := models.GetScenarioByID(s.ID, 1)
	if err != nil {
		t.Fatalf("error reloading scenario: %v", err)
	}
	return s
}

// seedTestCampaign creates and "launches" a minimal, valid campaign owned by
// user 1, following the same group/template/page/smtp setup used by
// createTestData in api_test.go, so assessment-phase tests have a real
// campaign to link against.
func seedTestCampaign(t *testing.T, name string) models.Campaign {
	group := models.Group{Name: name + " Group"}
	group.Targets = []models.Target{
		models.Target{BaseRecipient: models.BaseRecipient{Email: "phase-test@example.com", FirstName: "First", LastName: "Example"}},
	}
	group.UserId = 1
	if err := models.PostGroup(&group); err != nil {
		t.Fatalf("error creating group: %v", err)
	}

	template := models.Template{Name: name + " Template"}
	template.Subject = "Test subject"
	template.Text = "Text text"
	template.HTML = "<html>Test</html>"
	template.UserId = 1
	if err := models.PostTemplate(&template); err != nil {
		t.Fatalf("error creating template: %v", err)
	}

	p := models.Page{Name: name + " Page"}
	p.HTML = "<html>Test</html>"
	p.UserId = 1
	if err := models.PostPage(&p); err != nil {
		t.Fatalf("error creating page: %v", err)
	}

	smtp := models.SMTP{Name: name + " SMTP"}
	smtp.UserId = 1
	smtp.Host = "example.com"
	smtp.FromAddress = "test@test.com"
	if err := models.PostSMTP(&smtp); err != nil {
		t.Fatalf("error creating smtp: %v", err)
	}

	c := models.Campaign{Name: name}
	c.UserId = 1
	c.Template = template
	c.Page = p
	c.SMTP = smtp
	c.Groups = []models.Group{group}
	if err := models.PostCampaign(&c, c.UserId); err != nil {
		t.Fatalf("error creating campaign: %v", err)
	}
	c.UpdateStatus(models.CampaignEmailsSent)
	return c
}

// createTestAssessment creates an assessment via the API and returns the
// decoded response.
func createTestAssessment(t *testing.T, testCtx *testContext, body string) models.Assessment {
	r := httptest.NewRequest(http.MethodPost, "/api/assessments/", bytes.NewReader([]byte(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("error creating assessment: %d %s", w.Code, w.Body.String())
	}
	var a models.Assessment
	if err := json.NewDecoder(w.Body).Decode(&a); err != nil {
		t.Fatalf("error decoding created assessment: %v", err)
	}
	return a
}

// seedDraftScenario creates a scenario that is never approved.
func seedDraftScenario(t *testing.T) models.Scenario {
	s := models.Scenario{
		OwnerID:  1,
		Name:     "Draft scenario",
		SkillTag: "link-recognition",
		Kind:     models.ScenarioKindThreat,
		Subject:  "Test",
		HTML:     `<p><a href="{{.URL}}">link</a></p>`,
	}
	if err := models.CreateScenario(&s); err != nil {
		t.Fatalf("error creating scenario: %v", err)
	}
	return s
}

func TestCreateAssessment(t *testing.T) {
	testCtx := setupTest(t)
	baseline := seedApprovedScenario(t)

	body := []byte(fmt.Sprintf(`{"name":"Q1 Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d}`, baseline.ID))
	r := httptest.NewRequest(http.MethodPost, "/api/assessments/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("unexpected status code creating assessment. expected %d got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
	var a models.Assessment
	if err := json.NewDecoder(w.Body).Decode(&a); err != nil {
		t.Fatalf("error decoding assessment response: %v", err)
	}
	if a.ID == 0 {
		t.Fatalf("expected non-zero assessment id")
	}
	if a.Status != models.AssessmentStatusDraft {
		t.Fatalf("unexpected assessment status. expected %s got %s", models.AssessmentStatusDraft, a.Status)
	}
	if a.BaselineScenarioID != baseline.ID {
		t.Fatalf("unexpected baseline scenario id. expected %d got %d", baseline.ID, a.BaselineScenarioID)
	}
}

func TestCreateAssessmentRejectsUnapprovedScenario(t *testing.T) {
	testCtx := setupTest(t)
	draft := seedDraftScenario(t)

	body := []byte(fmt.Sprintf(`{"name":"Q1 Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d}`, draft.ID))
	r := httptest.NewRequest(http.MethodPost, "/api/assessments/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestCreateAssessmentRejectsMissingBaseline(t *testing.T) {
	testCtx := setupTest(t)

	body := []byte(`{"name":"Q1 Assessment","skill_tag":"link-recognition"}`)
	r := httptest.NewRequest(http.MethodPost, "/api/assessments/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestGetAssessment(t *testing.T) {
	testCtx := setupTest(t)
	baseline := seedApprovedScenario(t)

	body := []byte(fmt.Sprintf(`{"name":"Q1 Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d,"observation_window_hours":48}`, baseline.ID))
	r := httptest.NewRequest(http.MethodPost, "/api/assessments/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("error creating assessment: %d %s", w.Code, w.Body.String())
	}
	var created models.Assessment
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("error decoding created assessment: %v", err)
	}

	r = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessments/%d", created.ID), nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w = httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code getting assessment. expected %d got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	var got models.Assessment
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("error decoding assessment: %v", err)
	}
	if got.Name != "Q1 Assessment" {
		t.Fatalf("unexpected name. expected %q got %q", "Q1 Assessment", got.Name)
	}
	if got.ObservationWindowHours != 48 {
		t.Fatalf("unexpected observation window. expected 48 got %d", got.ObservationWindowHours)
	}
	if got.BaselineScenarioID != baseline.ID {
		t.Fatalf("unexpected baseline scenario id. expected %d got %d", baseline.ID, got.BaselineScenarioID)
	}
}

func TestLinkAssessmentPhase(t *testing.T) {
	testCtx := setupTest(t)
	baseline := seedApprovedScenario(t)
	a := createTestAssessment(t, testCtx, fmt.Sprintf(`{"name":"Q1 Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d}`, baseline.ID))
	campaign := seedTestCampaign(t, "Phase Campaign")

	body := []byte(fmt.Sprintf(`{"phase":"baseline","campaign_id":%d}`, campaign.Id))
	r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/assessments/%d/phases", a.ID), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code linking assessment phase. expected %d got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	var p models.AssessmentPhase
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("error decoding assessment phase response: %v", err)
	}
	if p.CampaignID != campaign.Id {
		t.Fatalf("unexpected campaign id. expected %d got %d", campaign.Id, p.CampaignID)
	}
	if p.Phase != models.PhaseBaseline {
		t.Fatalf("unexpected phase. expected %q got %q", models.PhaseBaseline, p.Phase)
	}
}

func TestLinkAssessmentPhaseIdempotent(t *testing.T) {
	testCtx := setupTest(t)
	baseline := seedApprovedScenario(t)
	a := createTestAssessment(t, testCtx, fmt.Sprintf(`{"name":"Q1 Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d}`, baseline.ID))
	campaign1 := seedTestCampaign(t, "Phase Campaign 1")
	campaign2 := seedTestCampaign(t, "Phase Campaign 2")

	linkPhase := func(campaignID int64) int {
		body := []byte(fmt.Sprintf(`{"phase":"baseline","campaign_id":%d}`, campaignID))
		r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/assessments/%d/phases", a.ID), bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
		w := httptest.NewRecorder()
		testCtx.apiServer.ServeHTTP(w, r)
		return w.Code
	}

	if code := linkPhase(campaign1.Id); code != http.StatusOK {
		t.Fatalf("unexpected status code on first link. expected %d got %d", http.StatusOK, code)
	}
	if code := linkPhase(campaign2.Id); code != http.StatusOK {
		t.Fatalf("unexpected status code on second link. expected %d got %d", http.StatusOK, code)
	}

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessments/%d/phases", a.ID), nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code getting assessment phases. expected %d got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	var phases []models.AssessmentPhase
	if err := json.NewDecoder(w.Body).Decode(&phases); err != nil {
		t.Fatalf("error decoding assessment phases: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected exactly one phase entry, got %d", len(phases))
	}
	if phases[0].CampaignID != campaign2.Id {
		t.Fatalf("unexpected campaign id after idempotent link. expected %d got %d", campaign2.Id, phases[0].CampaignID)
	}
}

func TestLinkAssessmentPhaseInvalidPhase(t *testing.T) {
	testCtx := setupTest(t)
	baseline := seedApprovedScenario(t)
	a := createTestAssessment(t, testCtx, fmt.Sprintf(`{"name":"Q1 Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d}`, baseline.ID))
	campaign := seedTestCampaign(t, "Phase Campaign")

	body := []byte(fmt.Sprintf(`{"phase":"bogus","campaign_id":%d}`, campaign.Id))
	r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/assessments/%d/phases", a.ID), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestLinkAssessmentPhaseScenarioNotConfigured(t *testing.T) {
	testCtx := setupTest(t)
	baseline := seedApprovedScenario(t)
	// No followup_scenario_id set.
	a := createTestAssessment(t, testCtx, fmt.Sprintf(`{"name":"Q1 Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d}`, baseline.ID))
	campaign := seedTestCampaign(t, "Phase Campaign")

	body := []byte(fmt.Sprintf(`{"phase":"followup","campaign_id":%d}`, campaign.Id))
	r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/assessments/%d/phases", a.ID), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestGetAssessmentPhases(t *testing.T) {
	testCtx := setupTest(t)
	baseline := seedApprovedScenario(t)
	a := createTestAssessment(t, testCtx, fmt.Sprintf(`{"name":"Q1 Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d}`, baseline.ID))
	campaign := seedTestCampaign(t, "Phase Campaign")

	body := []byte(fmt.Sprintf(`{"phase":"baseline","campaign_id":%d}`, campaign.Id))
	r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/assessments/%d/phases", a.ID), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("error linking assessment phase: %d %s", w.Code, w.Body.String())
	}

	r = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessments/%d/phases", a.ID), nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w = httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code getting assessment phases. expected %d got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	var phases []models.AssessmentPhase
	if err := json.NewDecoder(w.Body).Decode(&phases); err != nil {
		t.Fatalf("error decoding assessment phases: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected exactly one phase entry, got %d", len(phases))
	}
	if phases[0].Phase != models.PhaseBaseline {
		t.Fatalf("unexpected phase. expected %q got %q", models.PhaseBaseline, phases[0].Phase)
	}
	if phases[0].CampaignID != campaign.Id {
		t.Fatalf("unexpected campaign id. expected %d got %d", campaign.Id, phases[0].CampaignID)
	}
	if phases[0].AssessmentID != a.ID {
		t.Fatalf("unexpected assessment id. expected %d got %d", a.ID, phases[0].AssessmentID)
	}
}
