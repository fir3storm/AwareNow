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

// CreateScenarioFromReportedMessage builds a new sanitized Scenario from an
// existing reported message. This is a separate action from the Task-3
// approve-to-template flow: it does not require (or change) the reported
// message's Status.
//
// POST /api/reported-messages/{id}/create-scenario
// Body: {"name": string, "skill_tag": string, "kind": "threat"|"benign"}
func (as *Server) CreateScenarioFromReportedMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)
	rm, err := models.GetReportedMessageByID(id, uid)
	if err != nil {
		reportedMessageError(w, err)
		return
	}

	var body struct {
		Name     string `json:"name"`
		SkillTag string `json:"skill_tag"`
		Kind     string `json:"kind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
		return
	}

	subject, text, html, err := ParseEmailContent(rawEmailFromParts(rm.Subject, rm.BodyText, rm.BodyHTML), true)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error converting message to scenario"}, http.StatusInternalServerError)
		return
	}

	s := models.Scenario{
		OwnerID:                 uid,
		SourceReportedMessageID: rm.ID,
		Name:                    body.Name,
		SkillTag:                body.SkillTag,
		Kind:                    body.Kind,
		Subject:                 subject,
		HTML:                    html,
		Text:                    text,
	}
	if err := models.CreateScenario(&s); err != nil {
		scenarioCreateError(w, err)
		return
	}
	JSONResponse(w, s, http.StatusCreated)
}

// Scenarios returns every scenario owned by the caller.
// GET /api/scenarios/
func (as *Server) Scenarios(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	scenarios, err := models.GetScenarios(ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving scenarios"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, scenarios, http.StatusOK)
}

// ScenarioByID returns a single scenario by ID.
// GET /api/scenarios/{id}
func (as *Server) ScenarioByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	s, err := models.GetScenarioByID(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		scenarioError(w, err)
		return
	}
	JSONResponse(w, s, http.StatusOK)
}

// ApproveScenario records a reviewer's sign-off on a scenario.
// POST /api/scenarios/{id}/approve
func (as *Server) ApproveScenario(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)
	reviewedBy := ctx.Get(r, "user").(models.User).Username
	if err := models.ApproveScenario(id, uid, reviewedBy); err != nil {
		scenarioError(w, err)
		return
	}
	s, err := models.GetScenarioByID(id, uid)
	if err != nil {
		scenarioError(w, err)
		return
	}
	JSONResponse(w, s, http.StatusOK)
}

func scenarioCreateError(w http.ResponseWriter, err error) {
	if errors.Is(err, models.ErrScenarioInvalidKind) || errors.Is(err, models.ErrScenarioUnsafeContent) {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	log.Error(err)
	JSONResponse(w, models.Response{Success: false, Message: "Error creating scenario"}, http.StatusInternalServerError)
}

func scenarioError(w http.ResponseWriter, err error) {
	if errors.Is(err, models.ErrScenarioNotFound) {
		JSONResponse(w, models.Response{Success: false, Message: "Scenario not found"}, http.StatusNotFound)
		return
	}
	log.Error(err)
	JSONResponse(w, models.Response{Success: false, Message: "Error processing scenario"}, http.StatusInternalServerError)
}
