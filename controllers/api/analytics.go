package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/gorilla/mux"
)

// AnalyticsOverview returns the top-level analytics statistics for the dashboard.
// GET /api/analytics/overview
func (as *Server) AnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving analytics overview"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, overview, http.StatusOK)
}

// CampaignTimeline returns time-series data for a specific campaign.
// GET /api/analytics/campaigns/{id}/timeline
func (as *Server) CampaignTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign ID"}, http.StatusBadRequest)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)

	// Verify the campaign belongs to the user
	_, err = models.GetCampaign(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}

	timeline, err := models.GetCampaignTimeline(id, uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving campaign timeline"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, timeline, http.StatusOK)
}

// OverallTimeline returns time-series data across all campaigns.
// GET /api/analytics/timeline
func (as *Server) OverallTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	timeline, err := models.GetOverallTimeline(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving timeline"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, timeline, http.StatusOK)
}

// DepartmentStats returns analytics broken down by department/position.
// GET /api/analytics/departments
func (as *Server) DepartmentStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	stats, err := models.GetDepartmentStats(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving department stats"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, stats, http.StatusOK)
}

// Trends returns trend data for the specified period.
// GET /api/analytics/trends?period=30d
func (as *Server) Trends(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	// Parse period parameter (default: 30 days)
	periodStr := r.URL.Query().Get("period")
	if periodStr == "" {
		periodStr = "30d"
	}

	days, err := parsePeriod(periodStr)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid period format. Use format like '30d', '7d', '90d'"}, http.StatusBadRequest)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	trends, err := models.GetTrendData(uid, days)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving trend data"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, trends, http.StatusOK)
}

// RiskScore returns the risk score calculation with breakdown.
// GET /api/analytics/risk-score
func (as *Server) RiskScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	riskScore, err := models.GetRiskScore(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error calculating risk score"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, riskScore, http.StatusOK)
}

// ExportAnalytics returns analytics data in the specified format.
// GET /api/analytics/export?format=csv|pdf|xlsx
func (as *Server) ExportAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	// Parse format parameter (default: csv)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	uid := ctx.Get(r, "user_id").(int64)

	switch format {
	case "csv":
		exportCSV(w, r, uid)
	case "json":
		exportJSON(w, r, uid)
	case "pdf", "xlsx":
		// PDF and XLSX export would require additional libraries
		JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Format '%s' export not yet implemented. Use 'csv' or 'json'.", format)}, http.StatusNotImplemented)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Unsupported format. Use 'csv', 'json', 'pdf', or 'xlsx'."}, http.StatusBadRequest)
	}
}

// exportCSV exports analytics data as CSV.
func exportCSV(w http.ResponseWriter, r *http.Request, uid int64) {
	data, err := models.ExportAnalyticsData(uid, "csv")
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
		return
	}

	// Type assert to get the export data structure
	exportData, ok := data.(models.AnalyticsExportData)
	if !ok {
		// Fallback: generate CSV from individual API calls
		generateCSVFromAnalytics(w, uid)
		return
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write overview section
	writer.Write([]string{"Analytics Overview"})
	writer.Write([]string{"Total Campaigns", "Emails Sent", "Open Rate", "Click Rate", "Submit Rate", "Report Rate", "Avg Time to Click", "Risk Score"})
	writer.Write([]string{
		strconv.FormatInt(exportData.Overview.TotalCampaigns, 10),
		strconv.FormatInt(exportData.Overview.EmailsSent, 10),
		fmt.Sprintf("%.2f%%", exportData.Overview.OpenRate),
		fmt.Sprintf("%.2f%%", exportData.Overview.ClickRate),
		fmt.Sprintf("%.2f%%", exportData.Overview.SubmitRate),
		fmt.Sprintf("%.2f%%", exportData.Overview.ReportRate),
		exportData.Overview.AvgTimeToClick,
		strconv.Itoa(exportData.Overview.RiskScore),
	})
	writer.Write([]string{})

	// Write timeline section
	writer.Write([]string{"Timeline"})
	writer.Write([]string{"Date", "Opens", "Clicks", "Submits"})
	for _, t := range exportData.Timeline {
		writer.Write([]string{
			t.Date,
			strconv.FormatInt(t.Opens, 10),
			strconv.FormatInt(t.Clicks, 10),
			strconv.FormatInt(t.Submits, 10),
		})
	}
	writer.Write([]string{})

	// Write department stats section
	writer.Write([]string{"Department Statistics"})
	writer.Write([]string{"Department", "Users Count", "Click Rate", "Submit Rate"})
	for _, d := range exportData.Departments {
		writer.Write([]string{
			d.Department,
			strconv.FormatInt(d.UsersCount, 10),
			fmt.Sprintf("%.2f%%", d.ClickRate),
			fmt.Sprintf("%.2f%%", d.SubmitRate),
		})
	}
	writer.Write([]string{})

	// Write risk score section
	writer.Write([]string{"Risk Score"})
	writer.Write([]string{"Score", "Level", "Click Rate", "Submit Rate", "Report Rate"})
	writer.Write([]string{
		strconv.Itoa(exportData.RiskScore.Score),
		exportData.RiskScore.Level,
		fmt.Sprintf("%.2f%%", exportData.RiskScore.ClickRate),
		fmt.Sprintf("%.2f%%", exportData.RiskScore.SubmitRate),
		fmt.Sprintf("%.2f%%", exportData.RiskScore.ReportRate),
	})

	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Errorf("error flushing CSV writer: %v", err)
		JSONResponse(w, models.Response{Success: false, Message: "Error generating CSV"}, http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	filename := fmt.Sprintf("analytics_export_%s.csv", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// generateCSVFromAnalytics generates CSV data by calling individual analytics functions.
func generateCSVFromAnalytics(w http.ResponseWriter, uid int64) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write overview section
	writer.Write([]string{"Analytics Overview"})
	writer.Write([]string{"Total Campaigns", "Emails Sent", "Open Rate", "Click Rate", "Submit Rate", "Report Rate", "Avg Time to Click", "Risk Score"})

	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
		return
	}

	writer.Write([]string{
		strconv.FormatInt(overview.TotalCampaigns, 10),
		strconv.FormatInt(overview.EmailsSent, 10),
		fmt.Sprintf("%.2f%%", overview.OpenRate),
		fmt.Sprintf("%.2f%%", overview.ClickRate),
		fmt.Sprintf("%.2f%%", overview.SubmitRate),
		fmt.Sprintf("%.2f%%", overview.ReportRate),
		overview.AvgTimeToClick,
		strconv.Itoa(overview.RiskScore),
	})
	writer.Write([]string{})

	// Write timeline section
	writer.Write([]string{"Timeline"})
	writer.Write([]string{"Date", "Opens", "Clicks", "Submits"})

	timeline, err := models.GetOverallTimeline(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting timeline"}, http.StatusInternalServerError)
		return
	}

	for _, t := range timeline {
		writer.Write([]string{
			t.Date,
			strconv.FormatInt(t.Opens, 10),
			strconv.FormatInt(t.Clicks, 10),
			strconv.FormatInt(t.Submits, 10),
		})
	}
	writer.Write([]string{})

	// Write department stats section
	writer.Write([]string{"Department Statistics"})
	writer.Write([]string{"Department", "Users Count", "Click Rate", "Submit Rate"})

	depts, err := models.GetDepartmentStats(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting department stats"}, http.StatusInternalServerError)
		return
	}

	for _, d := range depts {
		writer.Write([]string{
			d.Department,
			strconv.FormatInt(d.UsersCount, 10),
			fmt.Sprintf("%.2f%%", d.ClickRate),
			fmt.Sprintf("%.2f%%", d.SubmitRate),
		})
	}

	writer.Flush()

	// Set headers for file download
	filename := fmt.Sprintf("analytics_export_%s.csv", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

// exportJSON exports analytics data as JSON.
func exportJSON(w http.ResponseWriter, r *http.Request, uid int64) {
	// Gather all analytics data
	exportData := map[string]interface{}{}

	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
		return
	}
	exportData["overview"] = overview

	timeline, err := models.GetOverallTimeline(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting timeline"}, http.StatusInternalServerError)
		return
	}
	exportData["timeline"] = timeline

	depts, err := models.GetDepartmentStats(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting department stats"}, http.StatusInternalServerError)
		return
	}
	exportData["departments"] = depts

	trends, err := models.GetTrendData(uid, 30)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting trends"}, http.StatusInternalServerError)
		return
	}
	exportData["trends"] = trends

	risk, err := models.GetRiskScore(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting risk score"}, http.StatusInternalServerError)
		return
	}
	exportData["risk_score"] = risk

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error creating export"}, http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	filename := fmt.Sprintf("analytics_export_%s.json", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}

// parsePeriod parses a period string like "30d", "7d", "90d" into days.
func parsePeriod(period string) (int, error) {
	if len(period) < 2 {
		return 0, fmt.Errorf("invalid period format")
	}

	// Extract the numeric part and the unit
	unit := period[len(period)-1:]
	numStr := period[:len(period)-1]

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}

	switch unit {
	case "d":
		return num, nil
	case "w":
		return num * 7, nil
	case "m":
		return num * 30, nil
	default:
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}
}
