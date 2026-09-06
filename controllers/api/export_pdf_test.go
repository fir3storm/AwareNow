package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExportAnalyticsPDF(t *testing.T) {
	testCtx := setupTest(t)
	r := httptest.NewRequest(http.MethodGet, "/api/analytics/export?format=pdf", nil)
	r.Header.Set("Authorization", "Bearer "+testCtx.apiKey)
	w := httptest.NewRecorder()
	testCtx.apiServer.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf, got %s", ct)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("expected response body to start with the PDF magic bytes")
	}
}
