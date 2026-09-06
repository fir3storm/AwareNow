package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/xuri/excelize/v2"
)

// sanitizeSpreadsheetCell prevents a string from being interpreted as a
// formula by spreadsheet applications when it originates from untrusted,
// user-controlled data (e.g. an imported target list's position/department
// field). Values starting with '=', '+', '-', or '@' are forced to text.
func sanitizeSpreadsheetCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func exportXLSX(w http.ResponseWriter, r *http.Request, uid int64) {
	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
		return
	}
	timeline, err := models.GetOverallTimeline(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting timeline"}, http.StatusInternalServerError)
		return
	}
	depts, err := models.GetDepartmentStats(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting department stats"}, http.StatusInternalServerError)
		return
	}
	risk, err := models.GetRiskScore(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting risk score"}, http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	const overviewSheet = "Overview"
	f.SetSheetName("Sheet1", overviewSheet)
	overviewRows := [][2]interface{}{
		{"Total Campaigns", overview.TotalCampaigns},
		{"Emails Sent", overview.EmailsSent},
		{"Open Rate (%)", overview.OpenRate},
		{"Click Rate (%)", overview.ClickRate},
		{"Submit Rate (%)", overview.SubmitRate},
		{"Report Rate (%)", overview.ReportRate},
		{"Risk Score", risk.Score},
		{"Risk Level", risk.Level},
	}
	for i, row := range overviewRows {
		f.SetCellValue(overviewSheet, fmt.Sprintf("A%d", i+1), row[0])
		f.SetCellValue(overviewSheet, fmt.Sprintf("B%d", i+1), row[1])
	}

	const timelineSheet = "Timeline"
	f.NewSheet(timelineSheet)
	f.SetCellValue(timelineSheet, "A1", "Date")
	f.SetCellValue(timelineSheet, "B1", "Opens")
	f.SetCellValue(timelineSheet, "C1", "Clicks")
	f.SetCellValue(timelineSheet, "D1", "Submits")
	for i, t := range timeline {
		row := i + 2
		f.SetCellValue(timelineSheet, fmt.Sprintf("A%d", row), t.Date)
		f.SetCellValue(timelineSheet, fmt.Sprintf("B%d", row), t.Opens)
		f.SetCellValue(timelineSheet, fmt.Sprintf("C%d", row), t.Clicks)
		f.SetCellValue(timelineSheet, fmt.Sprintf("D%d", row), t.Submits)
	}

	const deptSheet = "Departments"
	f.NewSheet(deptSheet)
	f.SetCellValue(deptSheet, "A1", "Department")
	f.SetCellValue(deptSheet, "B1", "Users")
	f.SetCellValue(deptSheet, "C1", "Click Rate (%)")
	f.SetCellValue(deptSheet, "D1", "Submit Rate (%)")
	for i, d := range depts {
		row := i + 2
		f.SetCellValue(deptSheet, fmt.Sprintf("A%d", row), sanitizeSpreadsheetCell(d.Department))
		f.SetCellValue(deptSheet, fmt.Sprintf("B%d", row), d.UsersCount)
		f.SetCellValue(deptSheet, fmt.Sprintf("C%d", row), d.ClickRate)
		f.SetCellValue(deptSheet, fmt.Sprintf("D%d", row), d.SubmitRate)
	}

	f.SetActiveSheet(0)

	filename := fmt.Sprintf("analytics_export_%s.xlsx", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if err := f.Write(w); err != nil {
		log.Errorf("error writing XLSX output: %v", err)
	}
}
