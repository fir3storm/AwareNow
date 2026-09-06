package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xuri/excelize/v2"
)

// seedEvidenceAssessment creates an approved baseline scenario, an
// assessment referencing it, a real launched campaign, and links that
// campaign as the assessment's baseline phase, so evidence tests have a
// linked phase with computable metrics.
func seedEvidenceAssessment(t *testing.T, testCtx *testContext) int64 {
	baseline := seedApprovedScenario(t)
	a := createTestAssessment(t, testCtx, fmt.Sprintf(`{"name":"Evidence Assessment","skill_tag":"link-recognition","baseline_scenario_id":%d}`, baseline.ID))
	campaign := seedTestCampaign(t, "Evidence Phase Campaign")

	body := []byte(fmt.Sprintf(`{"phase":"baseline","campaign_id":%d}`, campaign.Id))
	r := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/assessments/%d/phases", a.ID), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("error linking assessment phase: %d %s", w.Code, w.Body.String())
	}
	return a.ID
}

func TestAssessmentEvidenceJSON(t *testing.T) {
	testCtx := setupTest(t)
	id := seedEvidenceAssessment(t, testCtx)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessments/%d/evidence", id), nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	var bundle struct {
		BundleVersion int `json:"bundle_version"`
		Phases        []struct {
			Phase string `json:"phase"`
		} `json:"phases"`
	}
	if err := json.NewDecoder(w.Body).Decode(&bundle); err != nil {
		t.Fatalf("error decoding evidence bundle: %v", err)
	}
	if bundle.BundleVersion == 0 {
		t.Fatalf("expected non-zero bundle version")
	}
	if len(bundle.Phases) < 1 {
		t.Fatalf("expected at least one phase, got %d", len(bundle.Phases))
	}
}

func TestAssessmentEvidencePDF(t *testing.T) {
	testCtx := setupTest(t)
	id := seedEvidenceAssessment(t, testCtx)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessments/%d/evidence?format=pdf", id), nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf, got %s", ct)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("expected response body to start with the PDF magic bytes")
	}
}

func TestAssessmentEvidenceXLSX(t *testing.T) {
	testCtx := setupTest(t)
	id := seedEvidenceAssessment(t, testCtx)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessments/%d/evidence?format=xlsx", id), nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
	wantCT := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if ct := w.Header().Get("Content-Type"); ct != wantCT {
		t.Fatalf("expected Content-Type %s, got %s", wantCT, ct)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Fatal("expected response body to start with the zip/xlsx magic bytes")
	}

	// Verifying the magic bytes alone would have missed a real gap found
	// during review: the Speed metric's eligible/any-report-count/median/
	// p25/p75 fields were previously never written anywhere in the
	// workbook. Parse the response back and confirm the dedicated "Speed"
	// sheet actually exists with the expected header and a data row for
	// the linked baseline phase.
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("error re-opening generated xlsx: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	found := false
	for _, s := range sheets {
		if s == "Speed" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a \"Speed\" sheet in the workbook, got sheets: %v", sheets)
	}

	header, err := f.GetRows("Speed")
	if err != nil {
		t.Fatalf("error reading Speed sheet: %v", err)
	}
	if len(header) < 2 {
		t.Fatalf("expected at least a header row and one data row in the Speed sheet, got %d rows", len(header))
	}
	wantHeader := []string{"Phase", "Eligible", "Any Report Count", "Median (hours)", "P25 (hours)", "P75 (hours)"}
	for i, want := range wantHeader {
		if i >= len(header[0]) || header[0][i] != want {
			t.Fatalf("Speed sheet header mismatch at column %d: got %v, want %v", i, header[0], wantHeader)
		}
	}
	if header[1][0] != "baseline" {
		t.Fatalf("expected the baseline phase's row in the Speed sheet, got %v", header[1])
	}
}

func TestAssessmentEvidenceInvalidFormat(t *testing.T) {
	testCtx := setupTest(t)
	id := seedEvidenceAssessment(t, testCtx)

	r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/assessments/%d/evidence?format=bogus", id), nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestAssessmentEvidenceNotFound(t *testing.T) {
	testCtx := setupTest(t)

	r := httptest.NewRequest(http.MethodGet, "/api/assessments/999999/evidence", nil)
	r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", testCtx.apiKey))
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code. expected %d got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}
