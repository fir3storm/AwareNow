package models

import (
	"errors"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/jinzhu/gorm"
)

// CampaignSMTP represents the many-to-many relationship between campaigns
// and sending profiles, tracking per-campaign usage of each SMTP.
type CampaignSMTP struct {
	ID         int64     `json:"id"`
	CampaignID int64     `json:"campaign_id"`
	SMTPID     int64     `json:"smtp_id"`
	SMTP       SMTP      `json:"smtp,omitempty"`
	EmailsSent int64     `json:"emails_sent"`
	CreatedAt  time.Time `json:"created_at"`
}

// ErrCampaignSMTPNotFound indicates a campaign-SMTP association was not found
var ErrCampaignSMTPNotFound = errors.New("campaign SMTP association not found")

// AddSMTPToCampaign adds a sending profile to a campaign.
// It creates a new campaign_smtps record if one doesn't already exist.
func AddSMTPToCampaign(campaignID, smtpID int64) error {
	existing := CampaignSMTP{}
	err := db.Where("campaign_id = ? AND smtp_id = ?", campaignID, smtpID).First(&existing).Error
	if err == nil {
		// Already exists, nothing to do
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		log.Errorf("error checking existing campaign SMTP: %v", err)
		return err
	}

	cs := CampaignSMTP{
		CampaignID: campaignID,
		SMTPID:     smtpID,
		EmailsSent: 0,
		CreatedAt:  time.Now().UTC(),
	}
	err = db.Save(&cs).Error
	if err != nil {
		log.Errorf("error adding SMTP to campaign: %v", err)
		return err
	}
	return nil
}

// RemoveSMTPFromCampaign removes a sending profile from a campaign.
func RemoveSMTPFromCampaign(campaignID, smtpID int64) error {
	err := db.Where("campaign_id = ? AND smtp_id = ?", campaignID, smtpID).Delete(&CampaignSMTP{}).Error
	if err != nil {
		log.Errorf("error removing SMTP from campaign: %v", err)
		return err
	}
	return nil
}

// GetCampaignSMTPs returns all CampaignSMTP relationships for a campaign.
func GetCampaignSMTPs(campaignID int64) ([]CampaignSMTP, error) {
	cs := []CampaignSMTP{}
	err := db.Where("campaign_id = ?", campaignID).Find(&cs).Error
	if err != nil {
		log.Errorf("error getting campaign SMTPs: %v", err)
		return cs, err
	}

	// Load the associated SMTP profiles
	for i := range cs {
		s, err := GetSMTP(cs[i].SMTPID, 0)
		if err != nil {
			log.Warnf("SMTP %d not found for campaign %d: %v", cs[i].SMTPID, campaignID, err)
			continue
		}
		cs[i].SMTP = s
	}

	return cs, nil
}

// IncrementCampaignSMTPUsage increments the email sent counter for a specific
// campaign-SMTP association. Distinct from IncrementSMTPUsage in smtp.go,
// which tracks a sending profile's global hourly usage.
func IncrementCampaignSMTPUsage(campaignID, smtpID int64) error {
	err := db.Model(&CampaignSMTP{}).
		Where("campaign_id = ? AND smtp_id = ?", campaignID, smtpID).
		UpdateColumn("emails_sent", gorm.Expr("emails_sent + ?", 1)).Error
	if err != nil {
		log.Errorf("error incrementing SMTP usage: %v", err)
		return err
	}
	return nil
}

// GetCampaignSMTPUsage returns the usage tracking record for a specific
// campaign-SMTP association.
func GetCampaignSMTPUsage(campaignID, smtpID int64) (*CampaignSMTP, error) {
	cs := &CampaignSMTP{}
	err := db.Where("campaign_id = ? AND smtp_id = ?", campaignID, smtpID).First(cs).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrCampaignSMTPNotFound
		}
		log.Errorf("error getting SMTP usage: %v", err)
		return nil, err
	}
	return cs, nil
}
