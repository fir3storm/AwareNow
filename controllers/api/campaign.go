package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// campaignCreateRequest wraps the campaign with additional multi-SMTP fields
type campaignCreateRequest struct {
	models.Campaign
	SMTPIDs        []int64               `json:"smtp_ids"`
	DeliveryConfig models.DeliveryConfig `json:"delivery_config"`
}

// Campaigns returns a list of campaigns if requested via GET.
// If requested via POST, APICampaigns creates a new campaign and returns a reference to it.
func (as *Server) Campaigns(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaigns(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, cs, http.StatusOK)
	//POST: Create a new campaign and return it as JSON
	case r.Method == "POST":
		req := campaignCreateRequest{}
		// Put the request into a campaign
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}

		c := req.Campaign
		uid := ctx.Get(r, "user_id").(int64)

		// Handle multi-SMTP: if smtp_ids provided, use those; otherwise fall back to legacy smtp_id
		hasMultiSMTP := len(req.SMTPIDs) > 0
		hasLegacySMTP := c.SMTP.Name != "" || c.SMTPId != 0

		if !hasMultiSMTP && !hasLegacySMTP {
			JSONResponse(w, models.Response{Success: false, Message: "At least one sending profile must be specified"}, http.StatusBadRequest)
			return
		}

		// Validate multi-SMTP profiles if provided
		if hasMultiSMTP {
			for _, smtpID := range req.SMTPIDs {
				_, err := models.GetSMTP(smtpID, uid)
				if err == gorm.ErrRecordNotFound {
					JSONResponse(w, models.Response{Success: false, Message: "Sending profile not found"}, http.StatusBadRequest)
					return
				} else if err != nil {
					JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
					return
				}
			}
		}

		// For backward compatibility: if using legacy single SMTP, set up the campaign normally
		if hasLegacySMTP {
			err = models.PostCampaign(&c, uid)
			if err != nil {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
				return
			}
			// If smtp_ids also provided alongside legacy, store the multi-SMTP relationships
			if hasMultiSMTP {
				err = storeCampaignSMTPs(c.Id, req.SMTPIDs, uid)
				if err != nil {
					log.WithFields(map[string]interface{}{
						"campaign_id": c.Id,
						"error":       err.Error(),
					}).Error("Failed to store campaign SMTP relationships")
				}
			}
		} else {
			// Multi-SMTP mode: use the first SMTP as the legacy SMTP for validation
			firstSMTP, err := models.GetSMTP(req.SMTPIDs[0], uid)
			if err != nil {
				JSONResponse(w, models.Response{Success: false, Message: "Invalid sending profile"}, http.StatusBadRequest)
				return
			}
			c.SMTP = firstSMTP
			c.SMTPId = firstSMTP.Id

			err = models.PostCampaign(&c, uid)
			if err != nil {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
				return
			}

			// Store all CampaignSMTP relationships
			err = storeCampaignSMTPs(c.Id, req.SMTPIDs, uid)
			if err != nil {
				log.WithFields(map[string]interface{}{
					"campaign_id": c.Id,
					"error":       err.Error(),
				}).Error("Failed to store campaign SMTP relationships")
			}
		}

		// Store delivery config (stored in campaign record)
		if req.DelayBetweenMs > 0 || req.SelectionStrategy != "" || req.MaxEmailsPerProfile > 0 || req.RetryFailedProfiles {
			err = storeDeliveryConfig(c.Id, req.DeliveryConfig)
			if err != nil {
				log.WithFields(map[string]interface{}{
					"campaign_id": c.Id,
					"error":       err.Error(),
				}).Error("Failed to store delivery config")
			}
		}

		// Reload campaign with SMTP details
		c, err = models.GetCampaign(c.Id, uid)
		if err != nil {
			log.Error(err)
		}

		// If the campaign is scheduled to launch immediately, send it to the worker.
		// Otherwise, the worker will pick it up at the scheduled time
		if c.Status == models.CampaignInProgress {
			go as.worker.LaunchCampaign(c)
		}
		JSONResponse(w, c, http.StatusCreated)
	}
}

// storeCampaignSMTPs creates CampaignSMTP relationships for a campaign
func storeCampaignSMTPs(campaignID int64, smtpIDs []int64, uid int64) error {
	// Delete any existing relationships first (for idempotency)
	models.DeleteCampaignSMTPsByCampaign(campaignID)

	for i, smtpID := range smtpIDs {
		cs := &models.CampaignSMTP{
			CampaignId: campaignID,
			SMTPId:     smtpID,
			Priority:   i,
		}
		if err := models.PostCampaignSMTP(cs); err != nil {
			return err
		}
	}
	return nil
}

// storeDeliveryConfig updates the campaign record with delivery config
func storeDeliveryConfig(campaignID int64, dc models.DeliveryConfig) error {
	// Set defaults
	if dc.SelectionStrategy == "" {
		dc.SelectionStrategy = models.DefaultSelectionStrategy
	}
	return models.UpdateDeliveryConfig(campaignID, dc)
}

// CampaignsSummary returns the summary for the current user's campaigns
func (as *Server) CampaignsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// Campaign returns details about the requested campaign. If the campaign is not
// valid, APICampaign returns null.
func (as *Server) Campaign(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	c, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, c, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteCampaign(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign deleted successfully!"}, http.StatusOK)
	}
}

// CampaignResults returns just the results for a given campaign to
// significantly reduce the information returned.
func (as *Server) CampaignResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	cr, err := models.GetCampaignResults(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		JSONResponse(w, cr, http.StatusOK)
		return
	}
}

// CampaignSummary returns the summary for a given campaign.
func (as *Server) CampaignSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummary(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			} else {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			}
			log.Error(err)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// CampaignComplete effectively "ends" a campaign.
// Future phishing emails clicked will return a simple "404" page.
func (as *Server) CampaignComplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		err := models.CompleteCampaign(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error completing campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign completed successfully!"}, http.StatusOK)
	}
}
