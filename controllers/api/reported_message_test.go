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
		OwnerID:       1,
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
	var listResp struct {
		Data    []models.ReportedMessage `json:"data"`
		Total   int64                    `json:"total"`
		Page    int                      `json:"page"`
		PerPage int                      `json:"per_page"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("error decoding reported messages list: %v", err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("unexpected number of reported messages received. expected 1 got %d", len(listResp.Data))
	}
	if listResp.Total != 1 {
		t.Fatalf("unexpected total received. expected 1 got %d", listResp.Total)
	}
	if listResp.Data[0].ReporterEmail != "bob@example.com" {
		t.Fatalf("unexpected reporter email received. expected %s got %s", "bob@example.com", listResp.Data[0].ReporterEmail)
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

func TestReportedMessagesAPIPagination(t *testing.T) {
	testCtx := setupTest(t)
	for i := 0; i < 3; i++ {
		rm := models.ReportedMessage{
			OwnerID:       1,
			ReporterEmail: fmt.Sprintf("reporter%d@example.com", i),
			Subject:       "Test",
			BodyHTML:      `<p><a href="http://evil.example">link</a></p>`,
		}
		if err := models.CreateReportedMessage(&rm); err != nil {
			t.Fatalf("error creating reported message: %v", err)
		}
	}

	r := httptest.NewRequest(http.MethodGet, "/api/reported-messages/?page=1&per_page=2", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code received listing reported messages. expected %d got %d", http.StatusOK, w.Code)
	}
	var listResp struct {
		Data    []models.ReportedMessage `json:"data"`
		Total   int64                    `json:"total"`
		Page    int                      `json:"page"`
		PerPage int                      `json:"per_page"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("error decoding reported messages list: %v", err)
	}
	if listResp.Total != 3 {
		t.Fatalf("unexpected total. expected 3 got %d", listResp.Total)
	}
	if listResp.Page != 1 {
		t.Fatalf("unexpected page. expected 1 got %d", listResp.Page)
	}
	if listResp.PerPage != 2 {
		t.Fatalf("unexpected per_page. expected 2 got %d", listResp.PerPage)
	}
	if len(listResp.Data) != 2 {
		t.Fatalf("unexpected number of reported messages on page 1. expected 2 got %d", len(listResp.Data))
	}

	// Second page should contain the remaining one message.
	r = httptest.NewRequest(http.MethodGet, "/api/reported-messages/?page=2&per_page=2", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w = httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code received listing reported messages page 2. expected %d got %d", http.StatusOK, w.Code)
	}
	var listResp2 struct {
		Data    []models.ReportedMessage `json:"data"`
		Total   int64                    `json:"total"`
		Page    int                      `json:"page"`
		PerPage int                      `json:"per_page"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp2); err != nil {
		t.Fatalf("error decoding reported messages list page 2: %v", err)
	}
	if listResp2.Page != 2 {
		t.Fatalf("unexpected page. expected 2 got %d", listResp2.Page)
	}
	if len(listResp2.Data) != 1 {
		t.Fatalf("unexpected number of reported messages on page 2. expected 1 got %d", len(listResp2.Data))
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

	updated, err := models.GetReportedMessageByID(rm.ID, 1)
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

	updated, err := models.GetReportedMessageByID(rm.ID, 1)
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

func TestReportedMessagesOwnerIsolation(t *testing.T) {
	testCtx := setupTest(t)
	rm := seedReportedMessage(t)
	other := createUnpriviledgedUser(t, models.RoleUser)
	for _, route := range []struct{ method, suffix string }{
		{http.MethodGet, ""}, {http.MethodPost, "/approve"}, {http.MethodPost, "/reject"},
	} {
		r := httptest.NewRequest(route.method, fmt.Sprintf("/api/reported-messages/%d%s", rm.ID, route.suffix), bytes.NewBufferString(`{"name":"Foreign"}`))
		r.Header.Set("Authorization", "Bearer "+other.ApiKey)
		w := httptest.NewRecorder()
		testCtx.apiServer.ServeHTTP(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d: %s", route.method, route.suffix, w.Code, w.Body.String())
		}
	}
	for _, filter := range []string{"", "?status=pending"} {
		r := httptest.NewRequest(http.MethodGet, "/api/reported-messages/"+filter, nil)
		r.Header.Set("Authorization", "Bearer "+other.ApiKey)
		w := httptest.NewRecorder()
		testCtx.apiServer.ServeHTTP(w, r)
		var listResp struct {
			Data []models.ReportedMessage `json:"data"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &listResp) != nil || len(listResp.Data) != 0 {
			t.Fatalf("foreign list exposed reports: %d %s", w.Code, w.Body.String())
		}
	}
	stored, err := models.GetReportedMessageByID(rm.ID, 1)
	if err != nil || stored.Status != models.ReportedMessageStatusPending {
		t.Fatalf("foreign review changed report: %+v, %v", stored, err)
	}
	templates, err := models.GetTemplates(other.Id)
	if err != nil || len(templates) != 0 {
		t.Fatalf("foreign review created templates: %+v, %v", templates, err)
	}
}

func TestReportedMessagesConcurrentApproval(t *testing.T) {
	testCtx := setupTest(t)
	rm := seedReportedMessage(t)
	const reviewers = 8
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, reviewers)
	for i := 0; i < reviewers; i++ {
		go func() {
			<-start
			r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/reported-messages/%d/approve", rm.ID), bytes.NewBufferString(`{"name":"Concurrent approval"}`))
			r.Header.Set("Authorization", "Bearer "+testCtx.apiKey)
			w := httptest.NewRecorder()
			testCtx.apiServer.ServeHTTP(w, r)
			responses <- w
		}()
	}
	close(start)
	successes := 0
	for i := 0; i < reviewers; i++ {
		w := <-responses
		if w.Code == http.StatusOK {
			successes++
		} else if w.Code != http.StatusConflict {
			t.Errorf("expected success or conflict, got %d: %s", w.Code, w.Body.String())
		}
	}
	templates, err := models.GetTemplates(1)
	if err != nil || successes != 1 || len(templates) != 1 {
		t.Fatalf("expected one approval and template; successes=%d templates=%d error=%v", successes, len(templates), err)
	}
	stored, err := models.GetReportedMessageByID(rm.ID, 1)
	if err != nil || stored.ConvertedTemplateID != templates[0].Id || stored.Status != models.ReportedMessageStatusApproved {
		t.Fatalf("approval/template mismatch: %+v, %v", stored, err)
	}
}

func TestReportedMessagesTerminalReviewConflicts(t *testing.T) {
	for _, decision := range []string{"approve", "reject"} {
		t.Run(decision, func(t *testing.T) {
			testCtx := setupTest(t)
			rm := seedReportedMessage(t)
			for i, action := range []string{decision, "approve", "reject"} {
				r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/reported-messages/%d/%s", rm.ID, action), bytes.NewBufferString(`{"name":"Terminal review"}`))
				r.Header.Set("Authorization", "Bearer "+testCtx.apiKey)
				w := httptest.NewRecorder()
				testCtx.apiServer.ServeHTTP(w, r)
				want := http.StatusConflict
				if i == 0 {
					want = http.StatusOK
				}
				if w.Code != want {
					t.Fatalf("%s after %s: expected %d, got %d: %s", action, decision, want, w.Code, w.Body.String())
				}
			}
			stored, err := models.GetReportedMessageByID(rm.ID, 1)
			wantStatus := models.ReportedMessageStatusApproved
			if decision == "reject" {
				wantStatus = models.ReportedMessageStatusRejected
			}
			if err != nil || stored.Status != wantStatus {
				t.Fatalf("terminal decision changed: %+v, %v", stored, err)
			}
		})
	}
}
