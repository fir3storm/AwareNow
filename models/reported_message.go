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

// ErrReportedMessageReviewed indicates a conflicting review of a terminal report.
var ErrReportedMessageReviewed = errors.New("reported message already reviewed")

// ErrReportedMessageOwnerRequired prevents intake without a trusted owner.
var ErrReportedMessageOwnerRequired = errors.New("reported message owner required")

// ErrReportedMessageIdempotencyConflict means a retry key was reused for other content.
var ErrReportedMessageIdempotencyConflict = errors.New("reported message idempotency key reused with different content")

// ReportedMessage stores a real (non-campaign) suspicious email a recipient
// reported through the Outlook add-in or another reporting client, pending
// admin review and optional conversion into a new phishing template.
type ReportedMessage struct {
	ID                  int64     `json:"id" gorm:"column:id; primary_key:yes"`
	OwnerID             int64     `json:"-" gorm:"column:owner_id; index"`
	IdempotencyKeyHash  *string   `json:"-" gorm:"column:idempotency_key_hash; type:varchar(64); unique_index"`
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
	if rm.OwnerID <= 0 {
		return ErrReportedMessageOwnerRequired
	}
	if _, err := GetUser(rm.OwnerID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return ErrReportedMessageOwnerRequired
		}
		return err
	}
	rm.Status = ReportedMessageStatusPending
	rm.CreatedAt = time.Now().UTC()
	err := db.Create(rm).Error
	if err != nil && rm.IdempotencyKeyHash != nil {
		var existing ReportedMessage
		if lookupErr := db.Where("owner_id = ? AND idempotency_key_hash = ?", rm.OwnerID, *rm.IdempotencyKeyHash).First(&existing).Error; lookupErr == nil {
			if existing.ReporterEmail != rm.ReporterEmail || existing.Subject != rm.Subject || existing.BodyText != rm.BodyText || existing.BodyHTML != rm.BodyHTML {
				return ErrReportedMessageIdempotencyConflict
			}
			*rm = existing
			return nil
		}
	}
	if err != nil {
		log.Errorf("error creating reported message: %v", err)
	}
	return err
}

// GetReportedMessages returns all reported messages with the given status.
// Pass an empty string to return all reported messages regardless of status.
func GetReportedMessages(ownerID int64, status string) ([]ReportedMessage, error) {
	rms := []ReportedMessage{}
	query := db.Where("owner_id = ? AND owner_id > 0", ownerID).Order("created_at desc")
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
func GetReportedMessageByID(id, ownerID int64) (ReportedMessage, error) {
	rm := ReportedMessage{}
	err := db.Where("id = ? AND owner_id = ? AND owner_id > 0", id, ownerID).First(&rm).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return rm, ErrReportedMessageNotFound
		}
		log.Errorf("error getting reported message by id: %v", err)
	}
	return rm, err
}

// reviewReportedMessage claims a pending report before performing any template
// writes. The conditional update serializes competing reviewers in the database.
func reviewReportedMessage(id, ownerID int64, status, reviewedBy string, template *Template) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	updates := map[string]interface{}{
		"status":                status,
		"reviewed_by":           reviewedBy,
		"reviewed_at":           time.Now().UTC(),
		"converted_template_id": int64(0),
	}
	result := tx.Model(&ReportedMessage{}).Where("id = ? AND owner_id = ? AND owner_id > 0 AND status = ?", id, ownerID, ReportedMessageStatusPending).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var report ReportedMessage
		err := tx.Where("id = ? AND owner_id = ? AND owner_id > 0", id, ownerID).First(&report).Error
		if err == gorm.ErrRecordNotFound {
			return ErrReportedMessageNotFound
		}
		if err != nil {
			return err
		}
		return ErrReportedMessageReviewed
	}
	if template != nil {
		template.Id = 0
		template.UserId = ownerID
		template.ModifiedDate = time.Now().UTC()
		if err := template.Validate(); err != nil {
			return err
		}
		if err := tx.Save(template).Error; err != nil {
			return err
		}
		for i := range template.Attachments {
			template.Attachments[i].TemplateId = template.Id
			if err := tx.Save(&template.Attachments[i]).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&ReportedMessage{}).Where("id = ? AND owner_id = ?", id, ownerID).Update("converted_template_id", template.Id).Error; err != nil {
			return err
		}
	}
	return tx.Commit().Error
}

// ApproveReportedMessage atomically creates a template and approves its report.
func ApproveReportedMessage(id, ownerID int64, reviewedBy string, template *Template) error {
	if template == nil {
		return ErrTemplateMissingParameter
	}
	return reviewReportedMessage(id, ownerID, ReportedMessageStatusApproved, reviewedBy, template)
}

// RejectReportedMessage dismisses only a pending report owned by the reviewer.
func RejectReportedMessage(id, ownerID int64, reviewedBy string) error {
	return reviewReportedMessage(id, ownerID, ReportedMessageStatusRejected, reviewedBy, nil)
}
