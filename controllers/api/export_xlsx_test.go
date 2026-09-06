package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExportAnalyticsXLSX(t *testing.T) {
	testCtx := setupTest(t)
	r := httptest.NewRequest(http.MethodGet, "/api/analytics/export?format=xlsx", nil)
	r.Header.Set("Authorization", "Bearer "+testCtx.apiKey)
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	wantCT := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if ct := w.Header().Get("Content-Type"); ct != wantCT {
		t.Fatalf("expected Content-Type %s, got %s", wantCT, ct)
	}
	// XLSX files are zip archives; PK\x03\x04 is the zip local-file-header magic.
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Fatal("expected response body to start with the zip/xlsx magic bytes")
	}
}

func TestSanitizeSpreadsheetCell(t *testing.T) {
	cases := map[string]string{
		"Engineering":        "Engineering",
		"=cmd|' /c calc'!A0": "'=cmd|' /c calc'!A0",
		"+1+1":               "'+1+1",
		"-1+1":               "'-1+1",
		"@SUM(A1:A2)":        "'@SUM(A1:A2)",
		"  =evil":            "'  =evil",
		"":                   "",
	}
	for input, want := range cases {
		if got := sanitizeSpreadsheetCell(input); got != want {
			t.Errorf("sanitizeSpreadsheetCell(%q) = %q, want %q", input, got, want)
		}
	}
}
