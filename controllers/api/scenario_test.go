package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fir3storm/AwareNow/models"
)

// seedReportedMessageWithLink creates a reported message with a live
// external link in its HTML body so tests can prove
// CreateScenarioFromReportedMessage's link-rewriting behavior.
func seedReportedMessageWithLink(t *testing.T) models.ReportedMessage {
	rm := models.ReportedMessage{
		OwnerID:       1,
		ReporterEmail: "alice@example.com",
		Subject:       "Please review",
		BodyHTML:      `<p>Click <a href="http://example.com">link</a></p>`,
	}
	if err := models.CreateReportedMessage(&rm); err != nil {
		t.Fatalf("error creating reported message: %v", err)
	}
	return rm
}

func TestCreateScenarioFromReportedMessage(t *testing.T) {
	testCtx := setupTest(t)
	rm := seedReportedMessageWithLink(t)

	body := []byte(`{"name":"Test Scenario","skill_tag":"link-recognition","kind":"threat"}`)
	url := fmt.Sprintf("/api/reported-messages/%d/create-scenario", rm.ID)
	r := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("unexpected status code creating scenario. expected %d got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}
	var s models.Scenario
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("error decoding scenario response: %v", err)
	}
	if s.ID == 0 {
		t.Fatalf("expected non-zero scenario id")
	}
	if s.Status != models.ScenarioStatusDraft {
		t.Fatalf("unexpected scenario status. expected %s got %s", models.ScenarioStatusDraft, s.Status)
	}
	if !strings.Contains(s.HTML, "{{.URL}}") {
		t.Fatalf("expected scenario HTML to contain {{.URL}}, got: %s", s.HTML)
	}
	if strings.Contains(s.HTML, "http://example.com") {
		t.Fatalf("expected scenario HTML to not contain original link, got: %s", s.HTML)
	}
	if s.SourceReportedMessageID != rm.ID {
		t.Fatalf("unexpected source reported message id. expected %d got %d", rm.ID, s.SourceReportedMessageID)
	}
}

func TestCreateScenarioInvalidKind(t *testing.T) {
	testCtx := setupTest(t)
	rm := seedReportedMessageWithLink(t)

	body := []byte(`{"name":"Test Scenario","skill_tag":"link-recognition","kind":"bogus"}`)
	url := fmt.Sprintf("/api/reported-messages/%d/create-scenario", rm.ID)
	r := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestApproveScenario(t *testing.T) {
	testCtx := setupTest(t)
	rm := seedReportedMessageWithLink(t)

	body := []byte(`{"name":"Test Scenario","skill_tag":"link-recognition","kind":"threat"}`)
	createURL := fmt.Sprintf("/api/reported-messages/%d/create-scenario", rm.ID)
	r := httptest.NewRequest(http.MethodPost, createURL, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("error creating scenario: %d %s", w.Code, w.Body.String())
	}
	var s models.Scenario
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("error decoding scenario response: %v", err)
	}

	approveURL := fmt.Sprintf("/api/scenarios/%d/approve", s.ID)
	r = httptest.NewRequest(http.MethodPost, approveURL, nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w = httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code approving scenario. expected %d got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	var approved models.Scenario
	if err := json.NewDecoder(w.Body).Decode(&approved); err != nil {
		t.Fatalf("error decoding approved scenario response: %v", err)
	}
	if approved.Status != models.ScenarioStatusApproved {
		t.Fatalf("unexpected scenario status. expected %s got %s", models.ScenarioStatusApproved, approved.Status)
	}
	if approved.ReviewedBy == "" {
		t.Fatalf("expected non-empty reviewed_by")
	}
}

// TestScenariosRequirePermission verifies that the scenario endpoints are
// gated behind PermissionModifyObjects, using the same zero-permission user
// helper as reported_message_test.go's TestReportedMessagesRequiresPermission.
func TestScenariosRequirePermission(t *testing.T) {
	testCtx := setupTest(t)
	noPermUser := createUserWithoutPermissions(t, "no-perms-scenario-user", "no-perms-scenario-key")

	r := httptest.NewRequest(http.MethodGet, "/api/scenarios/", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", noPermUser.ApiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected scenarios endpoint to reject a user without modify_objects permission, got status %d", w.Code)
	}
}
