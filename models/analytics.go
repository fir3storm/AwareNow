package models

import (
	"fmt"
	"time"
)

// AnalyticsOverview represents the top-level analytics statistics
// for the dashboard overview.
type AnalyticsOverview struct {
	TotalCampaigns  int64   `json:"total_campaigns"`
	EmailsSent      int64   `json:"emails_sent"`
	OpenRate        float64 `json:"open_rate"`
	ClickRate       float64 `json:"click_rate"`
	SubmitRate      float64 `json:"submit_rate"`
	ReportRate      float64 `json:"report_rate"`
	AvgTimeToClick  string  `json:"avg_time_to_click"`
	RiskScore       int     `json:"risk_score"`
}

// TimelineData represents a single point in the analytics timeline,
// showing the number of opens, clicks, and submits for a given date.
type TimelineData struct {
	Date   string `json:"date"`
	Opens  int64  `json:"opens"`
	Clicks int64  `json:"clicks"`
	Submits int64 `json:"submits"`
}

// DepartmentStats represents analytics statistics broken down by
// department/position.
type DepartmentStats struct {
	Department string  `json:"department"`
	UsersCount int64   `json:"users_count"`
	ClickRate  float64 `json:"click_rate"`
	SubmitRate float64 `json:"submit_rate"`
}

// TrendData represents a single data point in the trends analysis,
// showing how a metric changes over time.
type TrendData struct {
	Date       string  `json:"date"`
	OpenRate   float64 `json:"open_rate"`
	ClickRate  float64 `json:"click_rate"`
	SubmitRate float64 `json:"submit_rate"`
}

// RiskScoreBreakdown represents the breakdown of the risk score
// calculation.
type RiskScoreBreakdown struct {
	Score          int     `json:"score"`
	Level          string  `json:"level"`
	ClickRate      float64 `json:"click_rate"`
	SubmitRate     float64 `json:"submit_rate"`
	ReportRate     float64 `json:"report_rate"`
	Recommendations []string `json:"recommendations"`
}

// GetAnalyticsOverview calculates and returns the overview statistics
// for all campaigns owned by the given user.
func GetAnalyticsOverview(uid int64) (AnalyticsOverview, error) {
	overview := AnalyticsOverview{}

	// Get total campaigns
	err := db.Model(&Campaign{}).Where("user_id = ?", uid).Count(&overview.TotalCampaigns).Error
	if err != nil {
		return overview, err
	}

	// Get total emails sent (results with status >= EventSent)
	// This includes all results that were successfully sent, opened, clicked, or submitted
	err = db.Model(&Result{}).
		Where("user_id = ? AND status IN (?, ?, ?, ?)", uid, EventSent, EventOpened, EventClicked, EventDataSubmit).
		Count(&overview.EmailsSent).Error
	if err != nil {
		return overview, err
	}

	// Calculate rates
	var openedCount, clickedCount, submittedCount, reportedCount int64

	if overview.EmailsSent > 0 {
		// Count opened (includes clicked and submitted since they also opened)
		db.Model(&Result{}).
			Where("user_id = ? AND status IN (?, ?, ?)", uid, EventOpened, EventClicked, EventDataSubmit).
			Count(&openedCount)

		// Count clicked (includes submitted since they also clicked)
		db.Model(&Result{}).
			Where("user_id = ? AND status IN (?, ?)", uid, EventClicked, EventDataSubmit).
			Count(&clickedCount)

		// Count submitted
		db.Model(&Result{}).
			Where("user_id = ? AND status = ?", uid, EventDataSubmit).
			Count(&submittedCount)

		// Count reported (uses the Reported boolean field, not status)
		db.Model(&Result{}).
			Where("user_id = ? AND reported = ?", uid, true).
			Count(&reportedCount)

		overview.OpenRate = float64(openedCount) / float64(overview.EmailsSent) * 100
		overview.ClickRate = float64(clickedCount) / float64(overview.EmailsSent) * 100
		overview.SubmitRate = float64(submittedCount) / float64(overview.EmailsSent) * 100
		overview.ReportRate = float64(reportedCount) / float64(overview.EmailsSent) * 100
	}

	// Calculate average time to click
	overview.AvgTimeToClick = calculateAvgTimeToClick(uid)

	// Calculate risk score
	overview.RiskScore = calculateRiskScore(overview.ClickRate, overview.SubmitRate, overview.ReportRate)

	return overview, nil
}

// GetCampaignTimeline returns time-series data for a specific campaign,
// showing opens, clicks, and submits per day.
func GetCampaignTimeline(cid int64, uid int64) ([]TimelineData, error) {
	timeline := []TimelineData{}

	// Get events for this campaign grouped by date and message type
	rows, err := db.Table("events").
		Select("date(events.time) as date, events.message, count(*) as count").
		Where("events.campaign_id = ?", cid).
		Group("date(events.time), events.message").
		Rows()
	if err != nil {
		return timeline, err
	}
	defer rows.Close()

	// Aggregate results by date
	dateMap := make(map[string]*TimelineData)
	for rows.Next() {
		var dateStr, message string
		var count int64
		err := rows.Scan(&dateStr, &message, &count)
		if err != nil {
			continue
		}

		if _, ok := dateMap[dateStr]; !ok {
			dateMap[dateStr] = &TimelineData{Date: dateStr}
		}

		switch message {
		case EventOpened:
			dateMap[dateStr].Opens += count
		case EventClicked:
			dateMap[dateStr].Clicks += count
		case EventDataSubmit:
			dateMap[dateStr].Submits += count
		}
	}

	// Convert map to slice
	for _, v := range dateMap {
		timeline = append(timeline, *v)
	}

	return timeline, nil
}

// GetOverallTimeline returns time-series data across all campaigns
// for the given user.
func GetOverallTimeline(uid int64) ([]TimelineData, error) {
	timeline := []TimelineData{}

	// Get events for all campaigns owned by this user grouped by date
	rows, err := db.Table("events").
		Select("date(events.time) as date, events.message, count(*) as count").
		Joins("JOIN campaigns ON events.campaign_id = campaigns.id").
		Where("campaigns.user_id = ?", uid).
		Group("date(events.time), events.message").
		Rows()
	if err != nil {
		return timeline, err
	}
	defer rows.Close()

	// Aggregate results by date
	dateMap := make(map[string]*TimelineData)
	for rows.Next() {
		var dateStr, message string
		var count int64
		err := rows.Scan(&dateStr, &message, &count)
		if err != nil {
			continue
		}

		if _, ok := dateMap[dateStr]; !ok {
			dateMap[dateStr] = &TimelineData{Date: dateStr}
		}

		switch message {
		case EventOpened:
			dateMap[dateStr].Opens += count
		case EventClicked:
			dateMap[dateStr].Clicks += count
		case EventDataSubmit:
			dateMap[dateStr].Submits += count
		}
	}

	// Convert map to slice
	for _, v := range dateMap {
		timeline = append(timeline, *v)
	}

	return timeline, nil
}

// GetDepartmentStats returns analytics broken down by department/position.
func GetDepartmentStats(uid int64) ([]DepartmentStats, error) {
	stats := []DepartmentStats{}

	// Get distinct positions for this user's results
	rows, err := db.Table("results").
		Select("position, count(*) as total").
		Where("user_id = ? AND position != ?", uid, "").
		Group("position").
		Rows()
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var position string
		var total int64
		err := rows.Scan(&position, &total)
		if err != nil {
			continue
		}

		dept := DepartmentStats{
			Department: position,
			UsersCount: total,
		}

		// Count clicked for this position
		var clickedCount int64
		db.Model(&Result{}).
			Where("user_id = ? AND position = ? AND status IN (?, ?)", uid, position, EventClicked, EventDataSubmit).
			Count(&clickedCount)

		// Count submitted for this position
		var submittedCount int64
		db.Model(&Result{}).
			Where("user_id = ? AND position = ? AND status = ?", uid, position, EventDataSubmit).
			Count(&submittedCount)

		if total > 0 {
			dept.ClickRate = float64(clickedCount) / float64(total) * 100
			dept.SubmitRate = float64(submittedCount) / float64(total) * 100
		}

		stats = append(stats, dept)
	}

	return stats, nil
}

// GetTrendData returns trend data for the specified period in days.
func GetTrendData(uid int64, days int) ([]TrendData, error) {
	trends := []TrendData{}

	// Calculate the start date
	startDate := time.Now().UTC().AddDate(0, 0, -days)

	// Get daily stats for the period
	rows, err := db.Table("events").
		Select("date(events.time) as date, events.message, count(*) as count").
		Joins("JOIN campaigns ON events.campaign_id = campaigns.id").
		Where("campaigns.user_id = ? AND events.time >= ?", uid, startDate).
		Group("date(events.time), events.message").
		Rows()
	if err != nil {
		return trends, err
	}
	defer rows.Close()

	// We need total emails sent per day to calculate rates
	sentRows, err := db.Table("results").
		Select("date(results.send_date) as date, count(*) as total").
		Where("user_id = ? AND send_date >= ? AND status IN (?, ?, ?, ?)",
			uid, startDate, EventSent, EventOpened, EventClicked, EventDataSubmit).
		Group("date(results.send_date)").
		Rows()
	if err != nil {
		return trends, err
	}
	defer sentRows.Close()

	// Build a map of date -> total sent
	sentMap := make(map[string]int64)
	for sentRows.Next() {
		var dateStr string
		var total int64
		err := sentRows.Scan(&dateStr, &total)
		if err != nil {
			continue
		}
		sentMap[dateStr] = total
	}

	// Aggregate events by date
	type dailyMetrics struct {
		Opens   int64
		Clicks  int64
		Submits int64
	}
	dateMap := make(map[string]*dailyMetrics)

	for rows.Next() {
		var dateStr, message string
		var count int64
		err := rows.Scan(&dateStr, &message, &count)
		if err != nil {
			continue
		}

		if _, ok := dateMap[dateStr]; !ok {
			dateMap[dateStr] = &dailyMetrics{}
		}

		switch message {
		case EventOpened:
			dateMap[dateStr].Opens += count
		case EventClicked:
			dateMap[dateStr].Clicks += count
		case EventDataSubmit:
			dateMap[dateStr].Submits += count
		}
	}

	// Build trend data with rates
	for dateStr, metrics := range dateMap {
		trend := TrendData{
			Date: dateStr,
		}
		total := sentMap[dateStr]
		if total > 0 {
			trend.OpenRate = float64(metrics.Opens) / float64(total) * 100
			trend.ClickRate = float64(metrics.Clicks) / float64(total) * 100
			trend.SubmitRate = float64(metrics.Submits) / float64(total) * 100
		}
		trends = append(trends, trend)
	}

	return trends, nil
}

// GetRiskScore calculates and returns the risk score with breakdown.
func GetRiskScore(uid int64) (RiskScoreBreakdown, error) {
	breakdown := RiskScoreBreakdown{}

	// Get total emails sent
	var totalSent int64
	err := db.Model(&Result{}).
		Where("user_id = ? AND status IN (?, ?, ?, ?)", uid, EventSent, EventOpened, EventClicked, EventDataSubmit).
		Count(&totalSent).Error
	if err != nil {
		return breakdown, err
	}

	if totalSent == 0 {
		breakdown.Score = 0
		breakdown.Level = "N/A"
		breakdown.Recommendations = []string{"No campaign data available. Run campaigns to calculate risk score."}
		return breakdown, nil
	}

	// Count clicked and submitted
	var clickedCount, submittedCount, reportedCount int64
	db.Model(&Result{}).
		Where("user_id = ? AND status IN (?, ?)", uid, EventClicked, EventDataSubmit).
		Count(&clickedCount)
	db.Model(&Result{}).
		Where("user_id = ? AND status = ?", uid, EventDataSubmit).
		Count(&submittedCount)
	db.Model(&Result{}).
		Where("user_id = ? AND reported = ?", uid, true).
		Count(&reportedCount)

	clickRate := float64(clickedCount) / float64(totalSent) * 100
	submitRate := float64(submittedCount) / float64(totalSent) * 100
	reportRate := float64(reportedCount) / float64(totalSent) * 100

	breakdown.ClickRate = clickRate
	breakdown.SubmitRate = submitRate
	breakdown.ReportRate = reportRate

	// Calculate risk score (0-100, higher = more risk)
	// Weight: click rate 40%, submit rate 40%, inverse of report rate 20%
	score := int(clickRate*0.4 + submitRate*0.4 + (100-reportRate)*0.2)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	breakdown.Score = score

	// Determine risk level
	switch {
	case score >= 75:
		breakdown.Level = "Critical"
	case score >= 50:
		breakdown.Level = "High"
	case score >= 25:
		breakdown.Level = "Medium"
	default:
		breakdown.Level = "Low"
	}

	// Generate recommendations
	breakdown.Recommendations = generateRecommendations(clickRate, submitRate, reportRate)

	return breakdown, nil
}

// calculateAvgTimeToClick calculates the average time between email sent
// and first click across all campaigns for a user.
func calculateAvgTimeToClick(uid int64) string {
	// Get the average time difference between send and click events
	// by joining events for the same email
	var avgSeconds struct {
		Avg float64
	}

	err := db.Table("events as sent").
		Select("avg(strftime('%s', clicked.time) - strftime('%s', sent.time)) as avg").
		Joins("JOIN events as clicked ON sent.email = clicked.email AND sent.campaign_id = clicked.campaign_id").
		Joins("JOIN campaigns ON sent.campaign_id = campaigns.id").
		Where("campaigns.user_id = ? AND sent.message = ? AND clicked.message = ?",
			uid, EventSent, EventClicked).
		Scan(&avgSeconds).Error

	if err != nil || avgSeconds.Avg <= 0 {
		return "N/A"
	}

	// Convert seconds to a human-readable duration
	seconds := int64(avgSeconds.Avg)
	d := time.Duration(seconds) * time.Second

	switch {
	case d.Hours() >= 24:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd %dh", days, int(d.Hours())%24)
	case d.Hours() >= 1:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	case d.Minutes() >= 1:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), seconds%60)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// calculateRiskScore calculates a risk score based on click, submit, and report rates.
func calculateRiskScore(clickRate, submitRate, reportRate float64) int {
	score := int(clickRate*0.4 + submitRate*0.4 + (100-reportRate)*0.2)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// generateRecommendations generates security awareness recommendations
// based on the given rates.
func generateRecommendations(clickRate, submitRate, reportRate float64) []string {
	recommendations := []string{}

	if clickRate > 30 {
		recommendations = append(recommendations,
			"Click rate is high. Consider implementing additional phishing awareness training focusing on identifying suspicious links.")
	}

	if submitRate > 15 {
		recommendations = append(recommendations,
			"Credential submission rate is elevated. Deploy training on recognizing fake login pages and verifying URLs before entering credentials.")
	}

	if reportRate < 20 {
		recommendations = append(recommendations,
			"Report rate is low. Encourage employees to report suspicious emails and make the reporting process as simple as possible.")
	}

	if clickRate > 50 {
		recommendations = append(recommendations,
			"Critical: Over half of recipients are clicking phishing links. Immediate organization-wide security awareness training is recommended.")
	}

	if submitRate > 30 {
		recommendations = append(recommendations,
			"Critical: High credential submission rate. Consider implementing technical controls such as password managers and multi-factor authentication.")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"Security awareness metrics are within acceptable ranges. Continue regular training and simulated phishing campaigns.")
	}

	return recommendations
}

// ExportAnalyticsData returns all analytics data for export in the specified format.
func ExportAnalyticsData(uid int64, format string) (interface{}, error) {
	// Gather all analytics data for export
	type ExportData struct {
		Overview    AnalyticsOverview `json:"overview"`
		Timeline    []TimelineData    `json:"timeline"`
		Departments []DepartmentStats `json:"departments"`
		Trends      []TrendData       `json:"trends"`
		RiskScore   RiskScoreBreakdown `json:"risk_score"`
	}

	data := ExportData{}

	// Get overview
	overview, err := GetAnalyticsOverview(uid)
	if err != nil {
		return nil, err
	}
	data.Overview = overview

	// Get timeline
	timeline, err := GetOverallTimeline(uid)
	if err != nil {
		return nil, err
	}
	data.Timeline = timeline

	// Get department stats
	depts, err := GetDepartmentStats(uid)
	if err != nil {
		return nil, err
	}
	data.Departments = depts

	// Get trends (last 30 days)
	trends, err := GetTrendData(uid, 30)
	if err != nil {
		return nil, err
	}
	data.Trends = trends

	// Get risk score
	risk, err := GetRiskScore(uid)
	if err != nil {
		return nil, err
	}
	data.RiskScore = risk

	return data, nil
}

// GetCampaignAnalytics returns comprehensive analytics for a single campaign.
func GetCampaignAnalytics(cid int64, uid int64) (map[string]interface{}, error) {
	// Verify the campaign belongs to the user
	campaign, err := GetCampaign(cid, uid)
	if err != nil {
		return nil, err
	}

	// Get campaign stats
	stats, err := getCampaignStats(cid)
	if err != nil {
		return nil, err
	}

	// Calculate rates
	var openRate, clickRate, submitRate, reportRate float64
	if stats.Total > 0 {
		openRate = float64(stats.OpenedEmail) / float64(stats.Total) * 100
		clickRate = float64(stats.ClickedLink) / float64(stats.Total) * 100
		submitRate = float64(stats.SubmittedData) / float64(stats.Total) * 100
		reportRate = float64(stats.EmailReported) / float64(stats.Total) * 100
	}

	result := map[string]interface{}{
		"campaign_id":   campaign.Id,
		"campaign_name": campaign.Name,
		"status":        campaign.Status,
		"stats":         stats,
		"rates": map[string]float64{
			"open_rate":   openRate,
			"click_rate":  clickRate,
			"submit_rate": submitRate,
			"report_rate": reportRate,
		},
	}

	return result, nil
}

