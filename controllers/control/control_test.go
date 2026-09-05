package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type campaignReaderStub struct {
	campaigns []SafeCampaignSummary
	err       error
}

func (s campaignReaderStub) ListSafeCampaigns() ([]SafeCampaignSummary, error) {
	return s.campaigns, s.err
}

type campaignStopperStub struct {
	stoppedID int64
	err       error
}

func (s *campaignStopperStub) StopCampaign(id int64) error {
	s.stoppedID = id
	return s.err
}

func newRequest(handler http.Handler, method, path, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestHandlerRejectsMissingBearerToken(t *testing.T) {
	handler := NewHandler("control-token", campaignReaderStub{}, &campaignStopperStub{})

	response := newRequest(handler, http.MethodGet, "/health", "")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestHandlerRejectsWrongBearerToken(t *testing.T) {
	handler := NewHandler("control-token", campaignReaderStub{}, &campaignStopperStub{})

	response := newRequest(handler, http.MethodGet, "/health", "Bearer wrong-token")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestHandlerHealthReportsReadiness(t *testing.T) {
	handler := NewHandler("control-token", campaignReaderStub{}, &campaignStopperStub{})

	response := newRequest(handler, http.MethodGet, "/health", "Bearer control-token")

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	var health map[string]string
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if health["status"] != "ready" {
		t.Fatalf("expected ready status, got %q", health["status"])
	}
	if _, ok := health["version"]; !ok {
		t.Fatal("expected version in health response")
	}
}

func TestHandlerCampaignsReturnsOnlySafeFields(t *testing.T) {
	createdAt := time.Date(2026, time.September, 5, 10, 30, 0, 0, time.UTC)
	launchDate := time.Date(2026, time.September, 6, 9, 0, 0, 0, time.UTC)
	handler := NewHandler("control-token", campaignReaderStub{campaigns: []SafeCampaignSummary{{
		ID:          42,
		Name:        "Autumn awareness",
		Status:      "Completed",
		CreatedAt:   createdAt,
		LaunchDate:  launchDate,
		ResultCount: 3,
	}}}, &campaignStopperStub{})

	response := newRequest(handler, http.MethodGet, "/campaigns", "Bearer control-token")

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	var campaigns []map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&campaigns); err != nil {
		t.Fatalf("decode campaign response: %v", err)
	}
	if len(campaigns) != 1 {
		t.Fatalf("expected one campaign, got %d", len(campaigns))
	}
	campaign := campaigns[0]
	expected := map[string]interface{}{
		"id":           float64(42),
		"name":         "Autumn awareness",
		"status":       "Completed",
		"created_at":   "2026-09-05T10:30:00Z",
		"launch_date":  "2026-09-06T09:00:00Z",
		"result_count": float64(3),
	}
	if len(campaign) != len(expected) {
		t.Fatalf("expected only %d safe fields, got %d: %#v", len(expected), len(campaign), campaign)
	}
	for field, want := range expected {
		if got, ok := campaign[field]; !ok || got != want {
			t.Errorf("field %q: expected %#v, got %#v", field, want, got)
		}
	}
}

func TestHandlerRejectsNonNumericCampaignID(t *testing.T) {
	handler := NewHandler("control-token", campaignReaderStub{}, &campaignStopperStub{})

	response := newRequest(handler, http.MethodPost, "/campaigns/not-a-number/stop", "Bearer control-token")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}
