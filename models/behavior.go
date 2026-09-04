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

package models

import (
	"encoding/json"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
)

// BehaviorEvent stores client-side behavioral data captured by the
// awarenow-track.js tracking script. Each event represents a batch of
// user interactions (mouse movements, scrolls, keypresses, etc.) for
// a specific recipient during a specific session.
type BehaviorEvent struct {
	Id          int64        `json:"id" gorm:"column:id; primary_key:yes"`
	Rid         string       `json:"rid" gorm:"column:r_id"`
	CampaignId  int64        `json:"campaign_id" gorm:"column:campaign_id"`
	UserId      int64        `json:"user_id" gorm:"column:user_id"`
	SessionId   string       `json:"session_id" gorm:"column:session_id"`
	EventType   string       `json:"event_type" gorm:"column:event_type"`
	EventTime   time.Time    `json:"event_time" gorm:"column:event_time"`
	TimeOnPage  int64        `json:"time_on_page" gorm:"column:time_on_page"`
	EmailClient string       `json:"email_client" gorm:"column:email_client"`
	DeviceType  string       `json:"device_type" gorm:"column:device_type"`
	Referrer    string       `json:"referrer" gorm:"column:referrer"`
	TLSCipher   string       `json:"tls_cipher" gorm:"column:tls_cipher"`
	Details     EventDetails `json:"-" gorm:"-"`
	DetailsJSON string       `json:"details" gorm:"column:details"`
	CreatedDate time.Time    `json:"created_date" gorm:"column:created_date"`
}

// BehaviorEventSummary provides aggregated behavior event data for a recipient.
type BehaviorEventSummary struct {
	Rid             string    `json:"rid"`
	TotalSessions   int       `json:"total_sessions"`
	TotalEvents     int       `json:"total_events"`
	TotalTimeOnPage int64     `json:"total_time_on_page"`
	FirstActivity   time.Time `json:"first_activity"`
	LastActivity    time.Time `json:"last_activity"`
	EmailClient     string    `json:"email_client"`
	DeviceType      string    `json:"device_type"`
	Referrer        string    `json:"referrer"`
}

// AddBehaviorEvent saves a behavior event to the database.
func AddBehaviorEvent(be *BehaviorEvent) error {
	// Marshal details to JSON for storage
	if be.Details.Browser != nil || be.Details.Payload != nil {
		dj, err := json.Marshal(be.Details)
		if err != nil {
			log.Errorf("error marshaling behavior event details: %v", err)
			return err
		}
		be.DetailsJSON = string(dj)
	}

	if be.CreatedDate.IsZero() {
		be.CreatedDate = time.Now().UTC()
	}

	err := db.Save(be).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// GetBehaviorEventsByRid retrieves all behavior events for a specific recipient
// that belong to campaigns owned by the given user.
func GetBehaviorEventsByRid(rid string, uid int64) ([]BehaviorEvent, error) {
	events := []BehaviorEvent{}
	err := db.Table("behavior_events").
		Joins("JOIN campaigns ON behavior_events.campaign_id = campaigns.id").
		Where("behavior_events.r_id = ? AND campaigns.user_id = ?", rid, uid).
		Order("behavior_events.event_time ASC").
		Find(&events).Error
	if err != nil {
		log.Error(err)
	}
	return events, err
}

// GetBehaviorEventSummary retrieves aggregated behavior event data for a
// specific recipient.
func GetBehaviorEventSummary(rid string, uid int64) (*BehaviorEventSummary, error) {
	summary := &BehaviorEventSummary{Rid: rid}

	// Get aggregated stats
	type aggResult struct {
		TotalSessions   int
		TotalEvents     int
		TotalTimeOnPage int64
		FirstActivity   time.Time
		LastActivity    time.Time
	}
	var agg aggResult

	err := db.Table("behavior_events").
		Joins("JOIN campaigns ON behavior_events.campaign_id = campaigns.id").
		Select("COUNT(DISTINCT session_id) as total_sessions, COUNT(*) as total_events, COALESCE(SUM(time_on_page), 0) as total_time_on_page, MIN(event_time) as first_activity, MAX(event_time) as last_activity").
		Where("behavior_events.r_id = ? AND campaigns.user_id = ?", rid, uid).
		Scan(&agg).Error
	if err != nil {
		log.Error(err)
		return summary, err
	}

	summary.TotalSessions = agg.TotalSessions
	summary.TotalEvents = agg.TotalEvents
	summary.TotalTimeOnPage = agg.TotalTimeOnPage
	summary.FirstActivity = agg.FirstActivity
	summary.LastActivity = agg.LastActivity

	// Get the most recent email client and device type
	var latestEvent BehaviorEvent
	err = db.Table("behavior_events").
		Joins("JOIN campaigns ON behavior_events.campaign_id = campaigns.id").
		Where("behavior_events.r_id = ? AND campaigns.user_id = ?", rid, uid).
		Order("behavior_events.event_time DESC").
		First(&latestEvent).Error
	if err == nil {
		summary.EmailClient = latestEvent.EmailClient
		summary.DeviceType = latestEvent.DeviceType
		summary.Referrer = latestEvent.Referrer
	}

	return summary, nil
}

// HandleBehaviorBatch updates a Result to reflect that behavior events were
// received for this recipient. This updates the tracking metadata with
// enhanced information about the recipient's environment.
func (r *Result) HandleBehaviorBatch(details EventDetails) error {
	// Create a behavior event entry
	eventDetails := EventDetails{
		Payload: details.Payload,
		Browser: make(map[string]string),
	}

	// Copy browser info from details
	for k, v := range details.Browser {
		eventDetails.Browser[k] = v
	}

	// Add timestamp
	eventDetails.Browser["behavior_recorded_at"] = time.Now().UTC().Format(time.RFC3339)

	event, err := r.createEvent(EventBehaviorBatch, eventDetails)
	if err != nil {
		return err
	}

	r.ModifiedDate = event.Time
	return db.Save(r).Error
}
