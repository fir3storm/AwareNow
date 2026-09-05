package models

import (
	"errors"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/jinzhu/gorm"
)

// ReportedMessageStatusPending indicates a reported message awaiting admin review.
const ReportedMessageStatusPending = "pending"

// ReportedMessageStatusApproved indicates a reported message that was converted into a template.
const ReportedMessageStatusApproved = "approved"

// ReportedMessageStatusRejected indicates a reported message an admin dismissed.
const ReportedMessageStatusRejected = "rejected"

// ErrReportedMessageNotFound indicates no reported message was found for the given criteria.
var ErrReportedMessageNotFound = errors.New("reported message not found")

// ReportedMessage stores a real (non-campaign) suspicious email a recipient
// reported through the Outlook add-in or another reporting client, pending
// admin review and optional conversion into a new phishing template.
type ReportedMessage struct {
	ID                  int64     `json:"id" gorm:"column:id; primary_key:yes"`
	ReporterEmail       string    `json:"reporter_email" gorm:"column:reporter_email; not null"`
	Subject             string    `json:"subject" gorm:"column:subject"`
	BodyText            string    `json:"body_text" gorm:"column:body_text; sql:type:text"`
	BodyHTML            string    `json:"body_html" gorm:"column:body_html; sql:type:text"`
	Status              string    `json:"status" gorm:"column:status; not null"`
	ConvertedTemplateID int64     `json:"converted_template_id" gorm:"column:converted_template_id"`
	ReviewedBy          string    `json:"reviewed_by" gorm:"column:reviewed_by"`
	CreatedAt           time.Time `json:"created_at" gorm:"column:created_at"`
	ReviewedAt          time.Time `json:"reviewed_at" gorm:"column:reviewed_at"`
}

// TableName specifies the table name for the ReportedMessage model
func (ReportedMessage) TableName() string {
	return "reported_messages"
}

// CreateReportedMessage saves a new reported message with a default
// "pending" status.
func CreateReportedMessage(rm *ReportedMessage) error {
	rm.Status = ReportedMessageStatusPending
	rm.CreatedAt = time.Now().UTC()
	err := db.Save(rm).Error
	if err != nil {
		log.Errorf("error creating reported message: %v", err)
	}
	return err
}

// GetReportedMessages returns all reported messages with the given status.
// Pass an empty string to return all reported messages regardless of status.
func GetReportedMessages(status string) ([]ReportedMessage, error) {
	rms := []ReportedMessage{}
	query := db.Order("created_at desc")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Find(&rms).Error
	if err != nil {
		log.Errorf("error getting reported messages: %v", err)
	}
	return rms, err
}

// GetReportedMessageByID retrieves a single reported message by its primary key.
func GetReportedMessageByID(id int64) (ReportedMessage, error) {
	rm := ReportedMessage{}
	err := db.Where("id = ?", id).First(&rm).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return rm, ErrReportedMessageNotFound
		}
		log.Errorf("error getting reported message by id: %v", err)
	}
	return rm, err
}

// UpdateReportedMessageStatus transitions a reported message to approved or
// rejected, recording who reviewed it and when. If approved, templateID
// should be the ID of the template created from this message (0 otherwise).
func UpdateReportedMessageStatus(id int64, status string, reviewedBy string, templateID int64) error {
	updates := map[string]interface{}{
		"status":                status,
		"reviewed_by":           reviewedBy,
		"reviewed_at":           time.Now().UTC(),
		"converted_template_id": templateID,
	}
	err := db.Model(&ReportedMessage{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		log.Errorf("error updating reported message status: %v", err)
	}
	return err
}
