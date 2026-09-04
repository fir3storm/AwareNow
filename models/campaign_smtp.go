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
func AddSMTPToCampaign(campaignID, smtpID uint) error {
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
		CampaignID: int64(campaignID),
		SMTPID:     int64(smtpID),
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
func RemoveSMTPFromCampaign(campaignID, smtpID uint) error {
	err := db.Where("campaign_id = ? AND smtp_id = ?", campaignID, smtpID).Delete(&CampaignSMTP{}).Error
	if err != nil {
		log.Errorf("error removing SMTP from campaign: %v", err)
		return err
	}
	return nil
}

// GetCampaignSMTPs returns all SMTP profiles associated with a campaign.
func GetCampaignSMTPs(campaignID uint) ([]SMTP, error) {
	smtps := []SMTP{}
	err := db.Table("smtp").
		Select("smtp.*").
		Joins("INNER JOIN campaign_smtps ON campaign_smtps.smtp_id = smtp.id").
		Where("campaign_smtps.campaign_id = ?", campaignID).
		Find(&smtps).Error
	if err != nil {
		log.Errorf("error getting campaign SMTPs: %v", err)
		return smtps, err
	}

	// Load headers for each SMTP
	for i := range smtps {
		hErr := db.Where("smtp_id=?", smtps[i].Id).Find(&smtps[i].Headers).Error
		if hErr != nil && hErr != gorm.ErrRecordNotFound {
			log.Errorf("error loading headers for SMTP %d: %v", smtps[i].Id, hErr)
			return smtps, hErr
		}
	}

	return smtps, nil
}

// IncrementSMTPUsage increments the email sent counter for a specific
// campaign-SMTP association.
func IncrementSMTPUsage(campaignID, smtpID uint) error {
	err := db.Model(&CampaignSMTP{}).
		Where("campaign_id = ? AND smtp_id = ?", campaignID, smtpID).
		UpdateColumn("emails_sent", gorm.Expr("emails_sent + ?", 1)).Error
	if err != nil {
		log.Errorf("error incrementing SMTP usage: %v", err)
		return err
	}
	return nil
}

// GetSMTPUsage returns the usage tracking record for a specific
// campaign-SMTP association.
func GetSMTPUsage(campaignID, smtpID uint) (*CampaignSMTP, error) {
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
