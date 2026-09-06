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
