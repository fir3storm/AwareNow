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

// createUserWithoutPermissions creates a user whose role has no permissions
// associated with it at all (not even view_objects/modify_objects). This is
// distinct from createUnpriviledgedUser(t, models.RoleUser) in user_test.go:
// the built-in "user" role actually has PermissionModifyObjects (see
// db/db_sqlite3/migrations/20190105192341_0.8.0_rbac.sql), so it would pass
// the reported-messages permission check. To exercise the "forbidden" path
// we need a role_id with no rows in role_permissions at all.
func createUserWithoutPermissions(t *testing.T, username, apiKey string) *models.User {
	u := &models.User{
		Username: username,
		Hash:     "bar",
		ApiKey:   apiKey,
		RoleID:   999999, // no such role, and no role_permissions rows reference it
	}
	err := models.PutUser(u)
	if err != nil {
		t.Fatalf("error saving user without permissions: %v", err)
	}
	return u
}

// seedReportedMessage creates a reported message directly via the models
// package (bypassing HTTP) so tests can focus on the handlers under test.
func seedReportedMessage(t *testing.T) models.ReportedMessage {
	rm := models.ReportedMessage{
		ReporterEmail: "bob@example.com",
		Subject:       "Test",
		BodyHTML:      `<p><a href="http://evil.example">link</a></p>`,
	}
	if err := models.CreateReportedMessage(&rm); err != nil {
		t.Fatalf("error creating reported message: %v", err)
	}
	return rm
}

func TestReportedMessagesListAndGet(t *testing.T) {
	testCtx := setupTest(t)
	rm := seedReportedMessage(t)

	// List endpoint
	r := httptest.NewRequest(http.MethodGet, "/api/reported-messages/", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code received listing reported messages. expected %d got %d", http.StatusOK, w.Code)
	}
	var msgs []models.ReportedMessage
	if err := json.NewDecoder(w.Body).Decode(&msgs); err != nil {
		t.Fatalf("error decoding reported messages list: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("unexpected number of reported messages received. expected 1 got %d", len(msgs))
	}
	if msgs[0].ReporterEmail != "bob@example.com" {
		t.Fatalf("unexpected reporter email received. expected %s got %s", "bob@example.com", msgs[0].ReporterEmail)
	}

	// Get-by-id endpoint
	url := fmt.Sprintf("/api/reported-messages/%d", rm.ID)
	r = httptest.NewRequest(http.MethodGet, url, nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w = httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code received getting reported message. expected %d got %d", http.StatusOK, w.Code)
	}
	var got models.ReportedMessage
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("error decoding reported message: %v", err)
	}
	if got.ReporterEmail != "bob@example.com" {
		t.Fatalf("unexpected reporter email received. expected %s got %s", "bob@example.com", got.ReporterEmail)
	}
}

func TestReportedMessagesApprove(t *testing.T) {
	testCtx := setupTest(t)
	rm := seedReportedMessage(t)

	body := []byte(`{"name":"From report: Test"}`)
	url := fmt.Sprintf("/api/reported-messages/%d/approve", rm.ID)
	r := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code received approving reported message. expected %d got %d", http.StatusOK, w.Code)
	}
	var tmpl models.Template
	if err := json.NewDecoder(w.Body).Decode(&tmpl); err != nil {
		t.Fatalf("error decoding template response: %v", err)
	}
	if tmpl.Id == 0 {
		t.Fatalf("expected non-zero template id, got %d", tmpl.Id)
	}

	updated, err := models.GetReportedMessageByID(rm.ID)
	if err != nil {
		t.Fatalf("error getting reported message by id: %v", err)
	}
	if updated.Status != models.ReportedMessageStatusApproved {
		t.Fatalf("unexpected reported message status. expected %s got %s", models.ReportedMessageStatusApproved, updated.Status)
	}
	if updated.ConvertedTemplateID == 0 {
		t.Fatalf("expected non-zero converted_template_id")
	}
}

func TestReportedMessagesReject(t *testing.T) {
	testCtx := setupTest(t)
	rm := seedReportedMessage(t)

	url := fmt.Sprintf("/api/reported-messages/%d/reject", rm.ID)
	r := httptest.NewRequest(http.MethodPost, url, nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code received rejecting reported message. expected %d got %d", http.StatusOK, w.Code)
	}

	updated, err := models.GetReportedMessageByID(rm.ID)
	if err != nil {
		t.Fatalf("error getting reported message by id: %v", err)
	}
	if updated.Status != models.ReportedMessageStatusRejected {
		t.Fatalf("unexpected reported message status. expected %s got %s", models.ReportedMessageStatusRejected, updated.Status)
	}
	if updated.ConvertedTemplateID != 0 {
		t.Fatalf("expected zero converted_template_id, got %d", updated.ConvertedTemplateID)
	}
}

// TestReportedMessagesRequiresPermission verifies that the reported-messages
// endpoints are gated behind PermissionModifyObjects.
//
// Note: the built-in "user" role (models.RoleUser) already has
// PermissionModifyObjects (see the rbac migration and
// models/rbac_test.go's TestHasPermission), same as every other
// object-management endpoint in this codebase (templates, pages, groups,
// campaigns). So createUnpriviledgedUser(t, models.RoleUser) from
// user_test.go would actually be *allowed* through this gate - it's only
// useful for endpoints gated on PermissionModifySystem. To exercise the
// actual "forbidden" path we use a user whose role has no permissions
// associated with it at all.
func TestReportedMessagesRequiresPermission(t *testing.T) {
	testCtx := setupTest(t)
	_ = testCtx
	seedReportedMessage(t)

	noPermUser := createUserWithoutPermissions(t, "no-perms-user", "no-perms-key")

	r := httptest.NewRequest(http.MethodGet, "/api/reported-messages/", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", noPermUser.ApiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code == http.StatusOK {
		t.Fatalf("expected reported-messages endpoint to reject a user without modify_objects permission, got status %d", w.Code)
	}
	expected := http.StatusForbidden
	if w.Code != expected {
		t.Fatalf("unexpected status code received. expected %d got %d", expected, w.Code)
	}
}
