package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/fir3storm/AwareNow/assessment"
	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/go-pdf/fpdf"
	"github.com/gorilla/mux"
	"github.com/xuri/excelize/v2"
)

// Assessments handles the functionality for the /api/assessments/ endpoint,
// dispatching on method the same way as Templates.
func (as *Server) Assessments(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		assessments, err := models.GetAssessments(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error retrieving assessments"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, assessments, http.StatusOK)
	case r.Method == "POST":
		as.CreateAssessment(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// CreateAssessment creates a new assessment definition.
// POST /api/assessments/
// Body: {"name": string, "skill_tag": string, "baseline_scenario_id": int64,
//
//	"followup_scenario_id": int64 (optional), "benign_scenario_id": int64 (optional),
//	"observation_window_hours": int64 (optional)}
func (as *Server) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	var body struct {
		Name                   string `json:"name"`
		SkillTag               string `json:"skill_tag"`
		BaselineScenarioID     int64  `json:"baseline_scenario_id"`
		FollowupScenarioID     int64  `json:"followup_scenario_id"`
		BenignScenarioID       int64  `json:"benign_scenario_id"`
		ObservationWindowHours int64  `json:"observation_window_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
		return
	}

	a := models.Assessment{
		OwnerID:                uid,
		Name:                   body.Name,
		SkillTag:               body.SkillTag,
		BaselineScenarioID:     body.BaselineScenarioID,
		FollowupScenarioID:     body.FollowupScenarioID,
		BenignScenarioID:       body.BenignScenarioID,
		ObservationWindowHours: body.ObservationWindowHours,
	}
	if err := models.CreateAssessment(&a); err != nil {
		assessmentCreateError(w, err)
		return
	}
	JSONResponse(w, a, http.StatusCreated)
}

// AssessmentByID returns a single assessment by ID.
// GET /api/assessments/{id}
func (as *Server) AssessmentByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	a, err := models.GetAssessmentByID(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		if errors.Is(err, models.ErrAssessmentNotFound) {
			JSONResponse(w, models.Response{Success: false, Message: "Assessment not found"}, http.StatusNotFound)
			return
		}
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error processing assessment"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, a, http.StatusOK)
}

// AssessmentPhaseHandler handles the functionality for the
// /api/assessments/{id}/phases endpoint, dispatching on method the same way
// as Assessments and Templates.
func (as *Server) AssessmentPhaseHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		as.AssessmentPhases(w, r)
	case r.Method == "POST":
		as.LinkAssessmentPhase(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// LinkAssessmentPhase records that the given campaign delivers the given
// phase of the given assessment. Calling it again for the same
// (assessment, phase) pair updates the CampaignID rather than erroring or
// duplicating.
//
// POST /api/assessments/{id}/phases
// Body: {"phase": string, "campaign_id": int64}
func (as *Server) LinkAssessmentPhase(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}

	var body struct {
		Phase      string `json:"phase"`
		CampaignID int64  `json:"campaign_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
		return
	}

	p, err := models.LinkAssessmentPhase(uid, id, body.Phase, body.CampaignID)
	if err != nil {
		assessmentPhaseLinkError(w, err)
		return
	}
	JSONResponse(w, p, http.StatusOK)
}

// AssessmentPhases returns every phase linked so far for the given
// assessment.
// GET /api/assessments/{id}/phases
func (as *Server) AssessmentPhases(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	phases, err := models.GetAssessmentPhases(id, uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving assessment phases"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, phases, http.StatusOK)
}

// assessmentPhaseLinkError maps each of LinkAssessmentPhase's specific error
// sentinels to a distinct, clear response, matching assessmentCreateError's
// style rather than collapsing them into one generic message.
func assessmentPhaseLinkError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, models.ErrAssessmentPhaseInvalid):
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
	case errors.Is(err, models.ErrAssessmentNotFound):
		JSONResponse(w, models.Response{Success: false, Message: "Assessment not found"}, http.StatusNotFound)
	case errors.Is(err, models.ErrAssessmentPhaseScenarioNotConfigured):
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
	case errors.Is(err, models.ErrAssessmentPhaseCampaignNotFound):
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
	default:
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error linking assessment phase"}, http.StatusInternalServerError)
	}
}

// assessmentCreateError maps each of CreateAssessment's specific error
// sentinels to a distinct, clear message so a reviewer can tell exactly
// which scenario reference is the problem, rather than a generic
// "invalid scenario" message.
func assessmentCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, models.ErrAssessmentScenarioRequired):
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
	case errors.Is(err, models.ErrAssessmentScenarioNotApproved):
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
	case errors.Is(err, models.ErrAssessmentScenarioKindMismatch):
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
	case errors.Is(err, models.ErrScenarioNotFound):
		JSONResponse(w, models.Response{Success: false, Message: "Referenced scenario not found"}, http.StatusBadRequest)
	case errors.Is(err, models.ErrAssessmentOwnerRequired):
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
	default:
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error creating assessment"}, http.StatusInternalServerError)
	}
}

// AssessmentEvidence returns the versioned evidence bundle for an
// assessment: its definition plus computed USP-1 metrics for every linked
// phase. GET /api/assessments/{id}/evidence?format=json|pdf|xlsx
// (default: json).
func (as *Server) AssessmentEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}

	// Parse format parameter (default: json), matching ExportAnalytics's
	// own default-handling idiom.
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	uid := ctx.Get(r, "user_id").(int64)
	bundle, err := models.GetAssessmentEvidence(id, uid)
	if err != nil {
		if errors.Is(err, models.ErrAssessmentNotFound) {
			JSONResponse(w, models.Response{Success: false, Message: "Assessment not found"}, http.StatusNotFound)
			return
		}
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving assessment evidence"}, http.StatusInternalServerError)
		return
	}

	switch format {
	case "json":
		JSONResponse(w, bundle, http.StatusOK)
	case "pdf":
		evidencePDF(w, bundle)
	case "xlsx":
		evidenceXLSX(w, bundle)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Unsupported format. Use 'json', 'pdf', or 'xlsx'."}, http.StatusBadRequest)
	}
}

// proportionLine formats an assessment.Proportion for display, printing
// "insufficient data (n=<Denominator>)" instead of a percentage when the
// proportion is Suppressed rather than silently showing a rate computed
// from too small a cohort.
func proportionLine(p *assessment.Proportion) string {
	if p == nil {
		return ""
	}
	if p.Suppressed {
		return fmt.Sprintf("insufficient data (n=%d)", p.Denominator)
	}
	return fmt.Sprintf("%.2f%% (n=%d/%d, 95%% CI %.2f%%-%.2f%%)", p.Rate*100, p.Numerator, p.Denominator, p.CILow*100, p.CIHigh*100)
}

// evidencePDF renders bundle as a PDF summary, mirroring export_pdf.go's
// structure and style.
func evidencePDF(w http.ResponseWriter, bundle models.EvidenceBundle) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "AwareNow Assessment Evidence Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Assessment: "+bundle.Assessment.Name, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Skill Tag: "+bundle.Assessment.SkillTag, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Generated "+bundle.GeneratedAt.Format(time.RFC1123), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	for _, phase := range bundle.Phases {
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(0, 8, "Phase: "+phase.Phase, "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		if phase.Recognition != nil {
			pdf.CellFormat(50, 6, "Recognition", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 6, proportionLine(phase.Recognition), "", 1, "L", false, 0, "")
		}
		if phase.Recovery != nil {
			pdf.CellFormat(50, 6, "Recovery", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 6, proportionLine(phase.Recovery), "", 1, "L", false, 0, "")
		}
		if phase.Speed != nil {
			pdf.CellFormat(50, 6, "Speed (eligible)", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 6, fmt.Sprintf("%d (%d reported)", phase.Speed.Eligible, phase.Speed.AnyReportCount), "", 1, "L", false, 0, "")
			pdf.CellFormat(50, 6, "Speed (median)", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 6, phase.Speed.MedianNs.String(), "", 1, "L", false, 0, "")
			pdf.CellFormat(50, 6, "Speed (P25 / P75)", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 6, phase.Speed.P25Ns.String()+" / "+phase.Speed.P75Ns.String(), "", 1, "L", false, 0, "")
			pdf.CellFormat(50, 6, "Nonreporting", "", 0, "L", false, 0, "")
			nr := phase.Speed.Nonreporting
			pdf.CellFormat(0, 6, proportionLine(&nr), "", 1, "L", false, 0, "")
		}
		if phase.Discrimination != nil {
			pdf.CellFormat(50, 6, "Discrimination", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 6, proportionLine(phase.Discrimination), "", 1, "L", false, 0, "")
		}
		pdf.Ln(2)
	}

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Limitations", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	for _, limitation := range bundle.Limitations {
		pdf.MultiCell(0, 5, "- "+limitation, "", "L", false)
	}

	filename := fmt.Sprintf("assessment_evidence_%d_%s.pdf", bundle.Assessment.ID, time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if err := pdf.Output(w); err != nil {
		log.Errorf("error writing PDF output: %v", err)
	}
}

// evidenceXLSX renders bundle as a multi-sheet workbook, mirroring
// export_xlsx.go's structure and style.
func evidenceXLSX(w http.ResponseWriter, bundle models.EvidenceBundle) {
	f := excelize.NewFile()
	defer f.Close()

	const overviewSheet = "Overview"
	f.SetSheetName("Sheet1", overviewSheet)
	overviewRows := [][2]interface{}{
		{"Assessment Name", sanitizeSpreadsheetCell(bundle.Assessment.Name)},
		{"Skill Tag", sanitizeSpreadsheetCell(bundle.Assessment.SkillTag)},
		{"Generated At", bundle.GeneratedAt.Format(time.RFC3339)},
		{"Bundle Version", bundle.BundleVersion},
	}
	for i, row := range overviewRows {
		f.SetCellValue(overviewSheet, fmt.Sprintf("A%d", i+1), row[0])
		f.SetCellValue(overviewSheet, fmt.Sprintf("B%d", i+1), row[1])
	}

	const phasesSheet = "Phases"
	f.NewSheet(phasesSheet)
	headers := []string{"Phase", "Metric", "Numerator", "Denominator", "Rate", "CI Low", "CI High", "Suppressed"}
	for i, h := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetCellValue(phasesSheet, fmt.Sprintf("%s1", col), h)
	}
	row := 2
	writeMetricRow := func(phase, metric string, p *assessment.Proportion) {
		if p == nil {
			return
		}
		f.SetCellValue(phasesSheet, fmt.Sprintf("A%d", row), sanitizeSpreadsheetCell(phase))
		f.SetCellValue(phasesSheet, fmt.Sprintf("B%d", row), metric)
		f.SetCellValue(phasesSheet, fmt.Sprintf("C%d", row), p.Numerator)
		f.SetCellValue(phasesSheet, fmt.Sprintf("D%d", row), p.Denominator)
		f.SetCellValue(phasesSheet, fmt.Sprintf("E%d", row), p.Rate)
		f.SetCellValue(phasesSheet, fmt.Sprintf("F%d", row), p.CILow)
		f.SetCellValue(phasesSheet, fmt.Sprintf("G%d", row), p.CIHigh)
		f.SetCellValue(phasesSheet, fmt.Sprintf("H%d", row), p.Suppressed)
		row++
	}
	for _, phase := range bundle.Phases {
		writeMetricRow(phase.Phase, "Recognition", phase.Recognition)
		writeMetricRow(phase.Phase, "Recovery", phase.Recovery)
		if phase.Speed != nil {
			nr := phase.Speed.Nonreporting
			f.SetCellValue(phasesSheet, fmt.Sprintf("A%d", row), sanitizeSpreadsheetCell(phase.Phase))
			f.SetCellValue(phasesSheet, fmt.Sprintf("B%d", row), "Speed (Nonreporting)")
			f.SetCellValue(phasesSheet, fmt.Sprintf("C%d", row), nr.Numerator)
			f.SetCellValue(phasesSheet, fmt.Sprintf("D%d", row), nr.Denominator)
			f.SetCellValue(phasesSheet, fmt.Sprintf("E%d", row), nr.Rate)
			f.SetCellValue(phasesSheet, fmt.Sprintf("F%d", row), nr.CILow)
			f.SetCellValue(phasesSheet, fmt.Sprintf("G%d", row), nr.CIHigh)
			f.SetCellValue(phasesSheet, fmt.Sprintf("H%d", row), nr.Suppressed)
			row++
		}
		writeMetricRow(phase.Phase, "Discrimination", phase.Discrimination)
	}

	// The Nonreporting sub-proportion above fits the Phases sheet's
	// proportion-shaped columns, but Speed's other fields (eligible count,
	// any-report count, and the median/p25/p75 time-to-report durations)
	// don't fit that shape — they get their own sheet instead of being
	// silently dropped from the export.
	const speedSheet = "Speed"
	f.NewSheet(speedSheet)
	speedHeaders := []string{"Phase", "Eligible", "Any Report Count", "Median (hours)", "P25 (hours)", "P75 (hours)"}
	for i, h := range speedHeaders {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetCellValue(speedSheet, fmt.Sprintf("%s1", col), h)
	}
	speedRow := 2
	for _, phase := range bundle.Phases {
		if phase.Speed == nil {
			continue
		}
		toHours := func(d time.Duration) float64 { return d.Hours() }
		f.SetCellValue(speedSheet, fmt.Sprintf("A%d", speedRow), sanitizeSpreadsheetCell(phase.Phase))
		f.SetCellValue(speedSheet, fmt.Sprintf("B%d", speedRow), phase.Speed.Eligible)
		f.SetCellValue(speedSheet, fmt.Sprintf("C%d", speedRow), phase.Speed.AnyReportCount)
		f.SetCellValue(speedSheet, fmt.Sprintf("D%d", speedRow), toHours(phase.Speed.MedianNs))
		f.SetCellValue(speedSheet, fmt.Sprintf("E%d", speedRow), toHours(phase.Speed.P25Ns))
		f.SetCellValue(speedSheet, fmt.Sprintf("F%d", speedRow), toHours(phase.Speed.P75Ns))
		speedRow++
	}

	const limitationsSheet = "Limitations"
	f.NewSheet(limitationsSheet)
	f.SetCellValue(limitationsSheet, "A1", "Limitation")
	for i, l := range bundle.Limitations {
		f.SetCellValue(limitationsSheet, fmt.Sprintf("A%d", i+2), l)
	}

	f.SetActiveSheet(0)

	filename := fmt.Sprintf("assessment_evidence_%d_%s.xlsx", bundle.Assessment.ID, time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if err := f.Write(w); err != nil {
		log.Errorf("error writing XLSX output: %v", err)
	}
}
