package models

import (
	"github.com/jinzhu/gorm"
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestCreateAndGetReportedMessage(c *check.C) {
	rm := ReportedMessage{
		OwnerID:       1,
		ReporterEmail: "alice@example.com",
		Subject:       "Your invoice is overdue",
		BodyText:      "Please click here to pay",
		BodyHTML:      "<p>Please <a href=\"http://evil.example\">click here</a> to pay</p>",
	}
	c.Assert(CreateReportedMessage(&rm), check.Equals, nil)
	c.Assert(rm.ID, check.Not(check.Equals), int64(0))
	c.Assert(rm.Status, check.Equals, ReportedMessageStatusPending)

	got, err := GetReportedMessageByID(rm.ID, 1)
	c.Assert(err, check.Equals, nil)
	c.Assert(got.Subject, check.Equals, rm.Subject)

	rms, _, err := GetReportedMessages(1, ReportedMessageFilter{Status: ReportedMessageStatusPending})
	c.Assert(err, check.Equals, nil)
	c.Assert(len(rms), check.Equals, 1)
}

func (s *ModelsSuite) TestGetReportedMessageByIDNotFound(c *check.C) {
	_, err := GetReportedMessageByID(999, 1)
	c.Assert(err, check.Equals, ErrReportedMessageNotFound)
}

func (s *ModelsSuite) TestReportedMessageOwnership(c *check.C) {
	rm := ReportedMessage{OwnerID: 1, ReporterEmail: "alice@example.com", BodyText: "Test"}
	c.Assert(CreateReportedMessage(&rm), check.IsNil)
	_, err := GetReportedMessageByID(rm.ID, 2)
	c.Assert(err, check.Equals, ErrReportedMessageNotFound)
	rms, _, err := GetReportedMessages(2, ReportedMessageFilter{})
	c.Assert(err, check.IsNil)
	c.Assert(rms, check.HasLen, 0)
	c.Assert(RejectReportedMessage(rm.ID, 2, "other"), check.Equals, ErrReportedMessageNotFound)
	tmpl := Template{Name: "Foreign", Text: "Test"}
	c.Assert(ApproveReportedMessage(rm.ID, 2, "other", &tmpl), check.Equals, ErrReportedMessageNotFound)

	// Simulate an existing report created before ownership was introduced.
	c.Assert(db.Model(&rm).Update("owner_id", 0).Error, check.IsNil)
	_, err = GetReportedMessageByID(rm.ID, 0)
	c.Assert(err, check.Equals, ErrReportedMessageNotFound)
	rms, _, err = GetReportedMessages(0, ReportedMessageFilter{})
	c.Assert(err, check.IsNil)
	c.Assert(rms, check.HasLen, 0)
	c.Assert(RejectReportedMessage(rm.ID, 0, "admin"), check.Equals, ErrReportedMessageNotFound)
	c.Assert(CreateReportedMessage(&ReportedMessage{}), check.Equals, ErrReportedMessageOwnerRequired)
	c.Assert(CreateReportedMessage(&ReportedMessage{OwnerID: 99999}), check.Equals, ErrReportedMessageOwnerRequired)
}

func (s *ModelsSuite) TestReportedMessageReviewTransitions(c *check.C) {
	for _, approve := range []bool{true, false} {
		rm := ReportedMessage{OwnerID: 1, ReporterEmail: "alice@example.com", BodyText: "Test"}
		c.Assert(CreateReportedMessage(&rm), check.IsNil)
		tmpl := Template{Name: "Review transition", Text: "Test", UserId: 999}
		if approve {
			c.Assert(ApproveReportedMessage(rm.ID, 1, "admin", &tmpl), check.IsNil)
			defer db.Delete(&tmpl)
			c.Assert(tmpl.UserId, check.Equals, int64(1))
			stored, err := GetReportedMessageByID(rm.ID, 1)
			c.Assert(err, check.IsNil)
			c.Assert(stored.ConvertedTemplateID, check.Equals, tmpl.Id)
		} else {
			c.Assert(RejectReportedMessage(rm.ID, 1, "admin"), check.IsNil)
		}
		c.Assert(RejectReportedMessage(rm.ID, 1, "admin"), check.Equals, ErrReportedMessageReviewed)
		c.Assert(ApproveReportedMessage(rm.ID, 1, "admin", &tmpl), check.Equals, ErrReportedMessageReviewed)
	}
	c.Assert(RejectReportedMessage(99999, 1, "admin"), check.Equals, ErrReportedMessageNotFound)
}

func (s *ModelsSuite) TestReportedMessageApprovalRollback(c *check.C) {
	rm := ReportedMessage{OwnerID: 1, ReporterEmail: "alice@example.com", BodyText: "Test"}
	c.Assert(CreateReportedMessage(&rm), check.IsNil)
	tmpl := Template{Name: "Rollback", Text: "Test"}
	// Fail after template insertion, when the report receives its template ID.
	c.Assert(db.Exec("CREATE TRIGGER fail_report_conversion BEFORE UPDATE OF converted_template_id ON reported_messages WHEN NEW.converted_template_id > 0 BEGIN SELECT RAISE(FAIL, 'injected failure'); END").Error, check.IsNil)
	defer db.Exec("DROP TRIGGER fail_report_conversion")
	c.Assert(ApproveReportedMessage(rm.ID, 1, "admin", &tmpl), check.NotNil)
	stored, err := GetReportedMessageByID(rm.ID, 1)
	c.Assert(err, check.IsNil)
	c.Assert(stored.Status, check.Equals, ReportedMessageStatusPending)
	c.Assert(stored.ConvertedTemplateID, check.Equals, int64(0))
	c.Assert(stored.ReviewedBy, check.Equals, "")
	c.Assert(stored.ReviewedAt.IsZero(), check.Equals, true)
	var count int
	c.Assert(db.Model(&Template{}).Where("name = ?", "Rollback").Count(&count).Error, check.IsNil)
	c.Assert(count, check.Equals, 0)
}

func (s *ModelsSuite) TestReportedMessageIdempotency(c *check.C) {
	key := "hashed-retry-key"
	original := ReportedMessage{OwnerID: 1, ReporterEmail: "alice@example.com", BodyText: "Test", IdempotencyKeyHash: &key}
	first := original
	c.Assert(CreateReportedMessage(&first), check.IsNil)
	retry := original
	c.Assert(CreateReportedMessage(&retry), check.IsNil)
	c.Assert(retry.ID, check.Equals, first.ID)
	changed := original
	changed.Subject = "Different message"
	c.Assert(CreateReportedMessage(&changed), check.Equals, ErrReportedMessageIdempotencyConflict)
	for i := 0; i < 2; i++ {
		withoutKey := original
		withoutKey.IdempotencyKeyHash = nil
		c.Assert(CreateReportedMessage(&withoutKey), check.IsNil)
	}
	rms, _, err := GetReportedMessages(1, ReportedMessageFilter{})
	c.Assert(err, check.IsNil)
	c.Assert(rms, check.HasLen, 3)
}

func (s *ModelsSuite) TestReportedMessagesFilterBySearch(c *check.C) {
	c.Assert(CreateReportedMessage(&ReportedMessage{OwnerID: 1, ReporterEmail: "alice@example.com", Subject: "Invoice overdue", BodyText: "Test"}), check.IsNil)
	c.Assert(CreateReportedMessage(&ReportedMessage{OwnerID: 1, ReporterEmail: "bob@example.com", Subject: "Password reset", BodyText: "Test"}), check.IsNil)
	c.Assert(CreateReportedMessage(&ReportedMessage{OwnerID: 1, ReporterEmail: "carol@example.com", Subject: "Meeting notes", BodyText: "Test"}), check.IsNil)

	// Search matches only the reporter email.
	rms, total, err := GetReportedMessages(1, ReportedMessageFilter{Search: "bob@"})
	c.Assert(err, check.IsNil)
	c.Assert(total, check.Equals, int64(1))
	c.Assert(rms, check.HasLen, 1)
	c.Assert(rms[0].ReporterEmail, check.Equals, "bob@example.com")

	// Search matches only the subject.
	rms, total, err = GetReportedMessages(1, ReportedMessageFilter{Search: "Invoice"})
	c.Assert(err, check.IsNil)
	c.Assert(total, check.Equals, int64(1))
	c.Assert(rms, check.HasLen, 1)
	c.Assert(rms[0].Subject, check.Equals, "Invoice overdue")

	// No match.
	rms, total, err = GetReportedMessages(1, ReportedMessageFilter{Search: "nonexistent-term"})
	c.Assert(err, check.IsNil)
	c.Assert(total, check.Equals, int64(0))
	c.Assert(rms, check.HasLen, 0)
}

func (s *ModelsSuite) TestReportedMessagesPagination(c *check.C) {
	for i := 0; i < 5; i++ {
		c.Assert(CreateReportedMessage(&ReportedMessage{OwnerID: 1, ReporterEmail: "page@example.com", Subject: "Page test", BodyText: "Test"}), check.IsNil)
	}

	rms, total, err := GetReportedMessages(1, ReportedMessageFilter{Page: 1, PerPage: 2})
	c.Assert(err, check.IsNil)
	c.Assert(total, check.Equals, int64(5))
	c.Assert(rms, check.HasLen, 2)

	rms, total, err = GetReportedMessages(1, ReportedMessageFilter{Page: 2, PerPage: 2})
	c.Assert(err, check.IsNil)
	c.Assert(total, check.Equals, int64(5))
	c.Assert(rms, check.HasLen, 2)

	rms, total, err = GetReportedMessages(1, ReportedMessageFilter{Page: 3, PerPage: 2})
	c.Assert(err, check.IsNil)
	c.Assert(total, check.Equals, int64(5))
	c.Assert(rms, check.HasLen, 1)
}

// Existing records have no trustworthy owner to infer. Schema upgrades must
// preserve their contents while excluding them from every normal review scope.
func (s *ModelsSuite) TestReportedMessageLegacySchemaQuarantine(c *check.C) {
	legacy, err := gorm.Open("sqlite3", ":memory:")
	c.Assert(err, check.IsNil)
	defer legacy.Close()
	legacy.DB().SetMaxOpenConns(1)
	c.Assert(legacy.Exec("CREATE TABLE reported_messages (id INTEGER PRIMARY KEY, reporter_email TEXT NOT NULL, status TEXT NOT NULL)").Error, check.IsNil)
	c.Assert(legacy.Exec("INSERT INTO reported_messages (id, reporter_email, status) VALUES (1, 'legacy@example.com', 'pending')").Error, check.IsNil)
	c.Assert(legacy.AutoMigrate(&ReportedMessage{}).Error, check.IsNil)
	originalDB := db
	db = legacy
	defer func() { db = originalDB }()
	for _, owner := range []int64{0, 1, 2} {
		reports, _, err := GetReportedMessages(owner, ReportedMessageFilter{})
		c.Assert(err, check.IsNil)
		c.Assert(reports, check.HasLen, 0)
		_, err = GetReportedMessageByID(1, owner)
		c.Assert(err, check.Equals, ErrReportedMessageNotFound)
		c.Assert(RejectReportedMessage(1, owner, "reviewer"), check.Equals, ErrReportedMessageNotFound)
		c.Assert(ApproveReportedMessage(1, owner, "reviewer", &Template{Name: "Legacy", Text: "Test"}), check.Equals, ErrReportedMessageNotFound)
	}
	var preserved ReportedMessage
	c.Assert(legacy.First(&preserved, 1).Error, check.IsNil)
	c.Assert(preserved.ReporterEmail, check.Equals, "legacy@example.com")
	c.Assert(preserved.Status, check.Equals, ReportedMessageStatusPending)
}
