package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// SendingProfiles handles requests for the /api/smtp/ endpoint
func (as *Server) SendingProfiles(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ss, err := models.GetSMTPs(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ss, http.StatusOK)
	//POST: Create a new SMTP and return it as JSON
	case r.Method == "POST":
		s := models.SMTP{}
		// Put the request into a page
		err := json.NewDecoder(r.Body).Decode(&s)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		// Check to make sure the name is unique
		_, err = models.GetSMTPByName(s.Name, ctx.Get(r, "user_id").(int64))
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "SMTP name already in use"}, http.StatusConflict)
			log.Error(err)
			return
		}
		s.ModifiedDate = time.Now().UTC()
		s.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PostSMTP(&s)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, s, http.StatusCreated)
	}
}

// SendingProfile contains functions to handle the GET'ing, DELETE'ing, and PUT'ing
// of a SMTP object
func (as *Server) SendingProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	s, err := models.GetSMTP(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "SMTP not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, s, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteSMTP(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting SMTP"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "SMTP Deleted Successfully"}, http.StatusOK)
	case r.Method == "PUT":
		s = models.SMTP{}
		err = json.NewDecoder(r.Body).Decode(&s)
		if err != nil {
			log.Error(err)
		}
		if s.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "/:id and /:smtp_id mismatch"}, http.StatusBadRequest)
			return
		}
		err = s.Validate()
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		s.ModifiedDate = time.Now().UTC()
		s.UserId = ctx.Get(r, "user_id").(int64)
		err = models.PutSMTP(&s)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error updating SMTP"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, s, http.StatusOK)
	}
}

// SMTPUsage returns the current hour usage statistics for an SMTP profile
func (as *Server) SMTPUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid SMTP ID"}, http.StatusBadRequest)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)

	// Verify the SMTP exists and belongs to the user
	s, err := models.GetSMTP(id, uid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "SMTP not found"}, http.StatusNotFound)
			return
		}
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	// Get current usage
	usage, err := models.GetSMTPUsage(id, s.MaxEmailsPerHour)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, usage, http.StatusOK)
}
