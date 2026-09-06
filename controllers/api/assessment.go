package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/gorilla/mux"
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
