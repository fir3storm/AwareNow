package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/gorilla/mux"
)

// ReportedMessages returns all reported messages, optionally filtered by
// the ?status= query parameter.
// GET /api/reported-messages/
func (as *Server) ReportedMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	status := r.URL.Query().Get("status")
	msgs, err := models.GetReportedMessages(ctx.Get(r, "user_id").(int64), status)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving reported messages"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, msgs, http.StatusOK)
}

// ReportedMessage returns a single reported message by ID.
// GET /api/reported-messages/{id}
func (as *Server) ReportedMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	rm, err := models.GetReportedMessageByID(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		reportedMessageError(w, err)
		return
	}
	JSONResponse(w, rm, http.StatusOK)
}

// ReportedMessageApprove converts a reported message into a new draft
// template and marks it approved.
// POST /api/reported-messages/{id}/approve
// Body: {"name": "<template name>"}
func (as *Server) ReportedMessageApprove(w http.ResponseWriter, r *http.Request) {
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
	if rm.Status != models.ReportedMessageStatusPending {
		JSONResponse(w, models.Response{Success: false, Message: "Reported message already reviewed"}, http.StatusConflict)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		body.Name = "From report: " + rm.Subject
	}

	convertLinks := rm.BodyHTML != ""
	_, text, html, err := ParseEmailContent(rawEmailFromParts(rm.Subject, rm.BodyText, rm.BodyHTML), convertLinks)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error converting message to template"}, http.StatusInternalServerError)
		return
	}

	tmpl := models.Template{
		UserId:  uid,
		Name:    body.Name,
		Subject: rm.Subject,
		Text:    text,
		HTML:    html,
	}
	if err := tmpl.Validate(); err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}

	reviewedBy := ctx.Get(r, "user").(models.User).Username
	if err := models.ApproveReportedMessage(id, uid, reviewedBy, &tmpl); err != nil {
		reportedMessageError(w, err)
		return
	}
	JSONResponse(w, tmpl, http.StatusOK)
}

// ReportedMessageReject dismisses a reported message without creating a template.
// POST /api/reported-messages/{id}/reject
func (as *Server) ReportedMessageReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	reviewedBy := ctx.Get(r, "user").(models.User).Username
	if err := models.RejectReportedMessage(id, ctx.Get(r, "user_id").(int64), reviewedBy); err != nil {
		reportedMessageError(w, err)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Reported message rejected"}, http.StatusOK)
}

func reportedMessageError(w http.ResponseWriter, err error) {
	switch err {
	case models.ErrReportedMessageNotFound:
		JSONResponse(w, models.Response{Success: false, Message: "Reported message not found"}, http.StatusNotFound)
	case models.ErrReportedMessageReviewed:
		JSONResponse(w, models.Response{Success: false, Message: "Reported message already reviewed"}, http.StatusConflict)
	default:
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error processing reported message"}, http.StatusInternalServerError)
	}
}

// rawEmailFromParts builds a minimal raw RFC 822 message from already-split
// fields so it can be run back through ParseEmailContent's link-rewriting
// logic without duplicating that logic.
func rawEmailFromParts(subject, text, html string) string {
	if html != "" {
		return "Subject: " + subject + "\r\nContent-Type: text/html\r\n\r\n" + html
	}
	return "Subject: " + subject + "\r\nContent-Type: text/plain\r\n\r\n" + text
}
