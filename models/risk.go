package models

import (
	"encoding/json"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
)

// RiskLevelLow indicates a low-risk recipient
const RiskLevelLow = "low"

// RiskLevelMedium indicates a medium-risk recipient
const RiskLevelMedium = "medium"

// RiskLevelHigh indicates a high-risk recipient
const RiskLevelHigh = "high"

// RiskLevelCritical indicates a critical-risk recipient
const RiskLevelCritical = "critical"

// RiskLevelUnknown indicates the risk level could not be determined
const RiskLevelUnknown = "unknown"

// RiskFactors contains the individual metrics used to compute
// a recipient's phishing susceptibility risk score.
type RiskFactors struct {
	// ClickThroughRate is the percentage of phishing emails the recipient
	// has clicked on across campaigns
	ClickThroughRate float64 `json:"click_through_rate"`

	// SubmitRate is the percentage of phishing emails for which the recipient
	// submitted credentials across campaigns
	SubmitRate float64 `json:"submit_rate"`

	// ReportRate is the percentage of phishing emails the recipient has reported
	ReportRate float64 `json:"report_rate"`

	// AverageTimeToClick is the average duration between email delivery and
	// the recipient clicking the link (in seconds)
	AverageTimeToClick float64 `json:"average_time_to_click"`

	// TotalInteractions is the total number of phishing simulation interactions
	// the recipient has had
	TotalInteractions int `json:"total_interactions"`

	// HasSensitiveDataExposure indicates whether the recipient has submitted
	// data on pages marked as capturing sensitive information
	HasSensitiveDataExposure bool `json:"has_sensitive_data_exposure"`

	// EmailClientRisk scores the email client's known vulnerability level (0-10)
	EmailClientRisk int `json:"email_client_risk"`

	// DeviceAnomalyScore indicates whether the device fingerprint shows
	// suspicious patterns (0-10)
	DeviceAnomalyScore int `json:"device_anomaly_score"`

	// RecencyFactor weights recent behavior more heavily (1.0 = neutral)
	RecencyFactor float64 `json:"recency_factor"`
}

// CalculateRiskScore computes a risk level string based on a recipient's
// campaign result and their recorded behavior events. It returns one of:
// "low", "medium", "high", "critical", or "unknown".
//
// The scoring uses a weighted composite of:
//   - Recipient's current status progression (40% weight)
//   - Behavior event patterns (30% weight)
//   - Time-to-click speed (20% weight)
//   - Repeat interaction factor (10% weight)
func CalculateRiskScore(r Result, events []BehaviorEvent) string {
	score := 0.0

	// --- Status-based scoring (40% weight, max 40 points) ---
	switch r.Status {
	case EventDataSubmit:
		score += 40.0
	case EventClicked:
		score += 28.0
	case EventOpened:
		score += 14.0
	case EventSent:
		score += 4.0
	case EventReported:
		score -= 20.0
	default:
		score += 0.0
	}

	// --- Behavior event pattern scoring (30% weight, max 30 points) ---
	if len(events) > 0 {
		formSubmitCount := 0
		clickCount := 0
		rapidClickCount := 0
		suspiciousEventCount := 0

		for _, e := range events {
			switch e.EventType {
			case "form_submit":
				formSubmitCount++
			case "click":
				clickCount++
				// Rapid clicks (less than 2 seconds on page) suggest automated or
				// reflexive behavior - higher risk
				if e.TimeOnPage > 0 && e.TimeOnPage < 2 {
					rapidClickCount++
				}
			case "suspicious_navigation", "credential_entry", "data_exfil":
				suspiciousEventCount++
			}
		}

		// Form submissions are the strongest risk signal in behavior events
		if formSubmitCount > 0 {
			score += 20.0
			if formSubmitCount > 2 {
				score += 5.0 // repeat submitters
			}
		}

		// Rapid clicking indicates low caution
		if rapidClickCount > 0 {
			score += 10.0
		}

		// Suspicious event patterns
		if suspiciousEventCount > 0 {
			score += 5.0
		}

		// Multiple click events without form submission still indicate engagement
		if clickCount > 3 {
			score += 5.0
		}

		// Cap behavior score contribution at 30
		if score > 70.0 {
			// Status (40) + Behavior cap (30) = 70
			score = 70.0
		}
	}

	// --- Time-to-click scoring (20% weight, max 20 points) ---
	if !r.SendDate.IsZero() && !r.ModifiedDate.IsZero() &&
		(r.Status == EventClicked || r.Status == EventDataSubmit) {
		timeToClick := r.ModifiedDate.Sub(r.SendDate).Seconds()
		switch {
		case timeToClick < 60:
			score += 20.0 // Very fast click - high risk
		case timeToClick < 300:
			score += 15.0 // Under 5 minutes
		case timeToClick < 900:
			score += 10.0 // Under 15 minutes
		case timeToClick < 3600:
			score += 5.0 // Under 1 hour
		default:
			score += 2.0 // Slow but still clicked
		}
	}

	// --- Repeat interaction penalty (10% weight, max 10 points) ---
	// More behavior events means more engagement with the phishing content
	if len(events) > 5 {
		score += 10.0
	} else if len(events) > 3 {
		score += 5.0
	} else if len(events) > 1 {
		score += 2.0
	}

	// Ensure score stays within 0-100 range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	// Map the composite score to a risk level
	riskLevel := mapScoreToRiskLevel(score)

	log.Debugf("Risk score calculated for result %s: %.2f -> %s (events: %d)",
		r.RId, score, riskLevel, len(events))

	return riskLevel
}

// CalculateRiskFactors computes a detailed RiskFactors struct for a recipient
// based on their full history of behavior events and campaign results.
func CalculateRiskFactors(r Result, events []BehaviorEvent) RiskFactors {
	factors := RiskFactors{
		RecencyFactor: 1.0,
	}

	if len(events) == 0 {
		return factors
	}

	// Calculate event type distribution
	formSubmitCount := 0
	clickCount := 0
	var firstClickTime time.Time

	for _, e := range events {
		switch e.EventType {
		case "form_submit":
			formSubmitCount++
		case "click":
			clickCount++
			if firstClickTime.IsZero() {
				firstClickTime = e.CreatedAt
			}
		case "credential_entry", "data_exfil":
			factors.HasSensitiveDataExposure = true
		}
	}

	// Derive rates from event patterns
	totalEvents := len(events)
	if totalEvents > 0 {
		factors.SubmitRate = float64(formSubmitCount) / float64(totalEvents) * 100
		factors.ClickThroughRate = float64(clickCount) / float64(totalEvents) * 100
	}

	factors.TotalInteractions = totalEvents

	// Calculate average time to click
	if !firstClickTime.IsZero() && !r.SendDate.IsZero() {
		factors.AverageTimeToClick = firstClickTime.Sub(r.SendDate).Seconds()
	}

	// Calculate recency factor - weight recent events more heavily
	if totalEvents > 0 {
		now := time.Now().UTC()
		var weightedSum float64
		for _, e := range events {
			hoursAgo := now.Sub(e.CreatedAt).Hours()
			if hoursAgo < 0 {
				hoursAgo = 0
			}
			// Exponential decay: events from 30+ days ago have minimal weight
			weight := 1.0 / (1.0 + hoursAgo/168.0) // 168 hours = 1 week half-life
			weightedSum += weight
		}
		factors.RecencyFactor = weightedSum / float64(totalEvents)
	}

	// Assess email client risk based on user agent analysis if events carry that info
	for _, e := range events {
		if e.EventType == "page_load" && len(e.EventData) > 0 {
			var eventInfo map[string]string
			// Use the standard encoding/json through the EventData field
			if err := json.Unmarshal(e.EventData, &eventInfo); err == nil {
				if ua, ok := eventInfo["user_agent"]; ok {
					factors.EmailClientRisk = assessEmailClientRisk(ua)
				}
			}
			break
		}
	}

	return factors
}

// mapScoreToRiskLevel converts a numeric score (0-100) to a risk level string.
func mapScoreToRiskLevel(score float64) string {
	switch {
	case score >= 75:
		return RiskLevelCritical
	case score >= 50:
		return RiskLevelHigh
	case score >= 25:
		return RiskLevelMedium
	case score > 0:
		return RiskLevelLow
	default:
		return RiskLevelUnknown
	}
}

// assessEmailClientRisk returns a 0-10 risk score based on the email client's
// known vulnerability to phishing attacks.
func assessEmailClientRisk(userAgent string) int {
	if userAgent == "" {
		return 5 // unknown client, moderate risk
	}

	// Lowercase would be needed for case-insensitive matching; using simple contains
	switch {
	case containsString(userAgent, "outlook") || containsString(userAgent, "ms-office"):
		return 6 // Outlook has scripting/phishing vulnerabilities
	case containsString(userAgent, "thunderbird"):
		return 4 // Thunderbird has decent protections
	case containsString(userAgent, "applemail") || containsString(userAgent, "macos"):
		return 5 // Apple Mail - moderate
	case containsString(userAgent, "gmail"):
		return 3 // Gmail has strong phishing detection
	case containsString(userAgent, "ios") || containsString(userAgent, "iphone"):
		return 5 // Mobile clients often have limited security indicators
	default:
		return 5 // Default moderate risk for unknown clients
	}
}

// containsString is a simple case-sensitive substring check
func containsString(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RiskLevelFromScore converts an integer score (0-100) to a risk level string
func RiskLevelFromScore(score int) string {
	switch {
	case score >= 75:
		return RiskLevelCritical
	case score >= 50:
		return RiskLevelHigh
	case score >= 25:
		return RiskLevelMedium
	case score > 0:
		return RiskLevelLow
	default:
		return RiskLevelUnknown
	}
}

// UpdateResultRiskLevel updates the risk_level field on a result record
func UpdateResultRiskLevel(rid string, riskLevel string) error {
	err := db.Model(&Result{}).Where("r_id = ?", rid).Update("risk_level", riskLevel).Error
	if err != nil {
		log.Errorf("error updating risk level for result %s: %v", rid, err)
		return err
	}
	return nil
}
