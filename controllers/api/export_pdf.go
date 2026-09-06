package api

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/go-pdf/fpdf"
)

func exportPDF(w http.ResponseWriter, r *http.Request, uid int64) {
	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
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

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "AwareNow Analytics Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Generated "+time.Now().UTC().Format(time.RFC1123), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Overview", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	rows := [][2]string{
		{"Total Campaigns", fmt.Sprintf("%d", overview.TotalCampaigns)},
		{"Emails Sent", fmt.Sprintf("%d", overview.EmailsSent)},
		{"Open Rate", fmt.Sprintf("%.2f%%", overview.OpenRate)},
		{"Click Rate", fmt.Sprintf("%.2f%%", overview.ClickRate)},
		{"Submit Rate", fmt.Sprintf("%.2f%%", overview.SubmitRate)},
		{"Report Rate", fmt.Sprintf("%.2f%%", overview.ReportRate)},
		{"Risk Score", fmt.Sprintf("%d (%s)", risk.Score, risk.Level)},
	}
	for _, row := range rows {
		pdf.CellFormat(60, 6, row[0], "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, row[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Department Statistics", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(70, 6, "Department", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Users", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Click Rate", "1", 0, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Submit Rate", "1", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	for _, d := range depts {
		pdf.CellFormat(70, 6, d.Department, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, fmt.Sprintf("%d", d.UsersCount), "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, fmt.Sprintf("%.2f%%", d.ClickRate), "1", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("%.2f%%", d.SubmitRate), "1", 1, "L", false, 0, "")
	}

	filename := fmt.Sprintf("analytics_export_%s.pdf", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if err := pdf.Output(w); err != nil {
		log.Errorf("error writing PDF output: %v", err)
	}
}
