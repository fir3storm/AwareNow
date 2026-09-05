// Package control provides the private control-plane API for an engine.
package control

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fir3storm/AwareNow/config"
)

// EngineHealth is the safe readiness response for the control plane.
type EngineHealth struct {
	Ready   bool   `json:"ready"`
	Version string `json:"version"`
}

// SafeCampaignSummary contains only aggregate campaign information that may
// cross the private control boundary.
type SafeCampaignSummary struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	LaunchDate  time.Time `json:"launch_date"`
	ResultCount int64     `json:"result_count"`
}

type campaignListResponse struct {
	Campaigns []SafeCampaignSummary `json:"campaigns"`
}

// CampaignReader returns only safe campaign summaries.
type CampaignReader interface {
	ListSafeCampaigns() ([]SafeCampaignSummary, error)
}

// CampaignStopper stops a campaign identified by its numeric ID.
type CampaignStopper interface {
	StopCampaign(id int64) error
}

// NewHandler creates the private control-plane handler. The caller is
// responsible for mounting it under /api/v1/control after validating that the
// configured AWARENOW_CONTROL_TOKEN is non-empty.
func NewHandler(token string, campaignReader CampaignReader, campaignStopper CampaignStopper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r.Header.Get("Authorization"), token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			version := config.Version
			if version == "" {
				version = "unknown"
			}
			writeJSON(w, http.StatusOK, EngineHealth{Ready: true, Version: version})
		case r.Method == http.MethodGet && r.URL.Path == "/campaigns":
			campaigns, err := campaignReader.ListSafeCampaigns()
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, campaignListResponse{Campaigns: campaigns})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/campaigns/"):
			stopCampaign(w, r.URL.Path, campaignStopper)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
}

func authorized(authorization, token string) bool {
	if token == "" {
		return false
	}
	scheme, presentedToken, ok := strings.Cut(authorization, " ")
	if !ok || scheme != "Bearer" || presentedToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presentedToken), []byte(token)) == 1
}

func stopCampaign(w http.ResponseWriter, path string, campaignStopper CampaignStopper) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "campaigns" || parts[2] != "stop" || !numericID(parts[1]) {
		http.Error(w, "invalid campaign id", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		http.Error(w, "invalid campaign id", http.StatusBadRequest)
		return
	}
	if err := campaignStopper.StopCampaign(id); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func numericID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
