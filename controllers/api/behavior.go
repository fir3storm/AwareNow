/*
gophish - Open-Source Phishing Framework

The MIT License (MIT)

Copyright (c) 2013 Jordan Wright

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
)

// BehaviorEventPayload represents a single behavior event sent from the
// client-side tracking script. Each event captures a specific user action
// such as mouse movement, scrolling, key presses, or focus changes.
type BehaviorEventPayload struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// BehaviorEventBatch represents a batch of behavior events sent from the
// client-side tracking script for a specific recipient session.
type BehaviorEventBatch struct {
	Rid         string                 `json:"rid"`
	SessionID   string                 `json:"session_id"`
	Events      []BehaviorEventPayload `json:"events"`
	TimeOnPage  int64                  `json:"time_on_page"` // in seconds
	EmailClient string                 `json:"email_client"`
	DeviceType  string                 `json:"device_type"`
	Referrer    string                 `json:"referrer"`
	TLSVersion  string                 `json:"tls_version"`
}

// BehaviorEventResponse represents the response returned for behavior event
// API endpoints.
type BehaviorEventResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// BehaviorEvents handles POST requests to create new behavior event batches.
// POST /api/behavior-events
func (as *Server) BehaviorEvents(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "POST":
		as.createBehaviorEvents(w, r)
	case r.Method == "GET":
		as.getBehaviorEvents(w, r)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// createBehaviorEvents processes a batch of behavior events submitted by the
// client-side tracking script.
func (as *Server) createBehaviorEvents(w http.ResponseWriter, r *http.Request) {
	uid, ok := ctx.Get(r, "user_id").(int64)
	if !ok {
		JSONResponse(w, models.Response{Success: false, Message: "Unable to identify user"}, http.StatusUnauthorized)
		return
	}

	// Decode the event batch
	batch := BehaviorEventBatch{}
	err := json.NewDecoder(r.Body).Decode(&batch)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Invalid request body"}, http.StatusBadRequest)
		return
	}

	// Validate required fields
	if batch.Rid == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Missing recipient ID (rid)"}, http.StatusBadRequest)
		return
	}

	// Verify the result exists and belongs to the user
	rs, err := models.GetResult(batch.Rid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Recipient not found"}, http.StatusNotFound)
		return
	}

	// Verify campaign ownership
	c, err := models.GetCampaign(rs.CampaignId, uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Access denied"}, http.StatusForbidden)
		return
	}

	// Process each event in the batch
	eventCount := len(batch.Events)
	for _, event := range batch.Events {
		details := models.EventDetails{
			Payload: map[string][]string{},
			Browser: map[string]string{
				"rid":          batch.Rid,
				"session_id":   batch.SessionID,
				"event_type":   event.Type,
				"campaign_id":  strconv.FormatInt(c.Id, 10),
				"email_client": batch.EmailClient,
				"device_type":  batch.DeviceType,
				"referrer":     batch.Referrer,
			},
		}

		// Store event data
		if event.Data != nil {
			dataJSON, err := json.Marshal(event.Data)
			if err == nil {
				details.Browser["event_data"] = string(dataJSON)
			}
		}

		// Create behavior event
		be := &models.BehaviorEvent{
			Rid:         batch.Rid,
			CampaignId:  rs.CampaignId,
			UserId:      uid,
			SessionId:   batch.SessionID,
			EventType:   event.Type,
			EventTime:   event.Timestamp,
			TimeOnPage:  batch.TimeOnPage,
			EmailClient: batch.EmailClient,
			DeviceType:  batch.DeviceType,
			Referrer:    batch.Referrer,
			TLSCipher:   batch.TLSVersion,
			Details:     details,
		}

		err = models.AddBehaviorEvent(be)
		if err != nil {
			log.Errorf("error saving behavior event: %v", err)
		}
	}

	// Update the result with enhanced tracking metadata
	err = rs.HandleBehaviorBatch(models.EventDetails{
		Payload: map[string][]string{},
		Browser: map[string]string{
			"email_client": batch.EmailClient,
			"device_type":  batch.DeviceType,
			"referrer":     batch.Referrer,
			"session_id":   batch.SessionID,
		},
	})
	if err != nil {
		log.Errorf("error updating result with behavior batch: %v", err)
	}

	log.Infof("Processed %d behavior events for recipient %s in campaign %d", eventCount, batch.Rid, c.Id)

	JSONResponse(w, BehaviorEventResponse{
		Success: true,
		Message: "Behavior events recorded successfully",
		Data: map[string]interface{}{
			"events_processed": eventCount,
		},
	}, http.StatusCreated)
}

// getBehaviorEvents retrieves behavior events for a specific recipient.
// GET /api/behavior-events?rid=xxx
func (as *Server) getBehaviorEvents(w http.ResponseWriter, r *http.Request) {
	uid, ok := ctx.Get(r, "user_id").(int64)
	if !ok {
		JSONResponse(w, models.Response{Success: false, Message: "Unable to identify user"}, http.StatusUnauthorized)
		return
	}

	rid := r.URL.Query().Get("rid")
	if rid == "" {
		JSONResponse(w, models.Response{Success: false, Message: "Missing recipient ID (rid) parameter"}, http.StatusBadRequest)
		return
	}

	// Verify the result exists and belongs to the user
	rs, err := models.GetResult(rid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Recipient not found"}, http.StatusNotFound)
		return
	}

	// Verify campaign ownership
	_, err = models.GetCampaign(rs.CampaignId, uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Access denied"}, http.StatusForbidden)
		return
	}

	// Retrieve behavior events for this recipient
	events, err := models.GetBehaviorEventsByRid(rid, uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving behavior events"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, BehaviorEventResponse{
		Success: true,
		Message: "Behavior events retrieved successfully",
		Data:    events,
	}, http.StatusOK)
}
