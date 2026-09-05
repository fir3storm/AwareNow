package models

import (
	"gopkg.in/check.v1"
)

func (s *ModelsSuite) TestCreateAndGetReportedMessage(c *check.C) {
	rm := ReportedMessage{
		ReporterEmail: "alice@example.com",
		Subject:       "Your invoice is overdue",
		BodyText:      "Please click here to pay",
		BodyHTML:      "<p>Please <a href=\"http://evil.example\">click here</a> to pay</p>",
	}
	c.Assert(CreateReportedMessage(&rm), check.Equals, nil)
	c.Assert(rm.ID, check.Not(check.Equals), int64(0))
	c.Assert(rm.Status, check.Equals, ReportedMessageStatusPending)

	got, err := GetReportedMessageByID(rm.ID)
	c.Assert(err, check.Equals, nil)
	c.Assert(got.Subject, check.Equals, rm.Subject)

	rms, err := GetReportedMessages(ReportedMessageStatusPending)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(rms), check.Equals, 1)
}

func (s *ModelsSuite) TestGetReportedMessageByIDNotFound(c *check.C) {
	_, err := GetReportedMessageByID(999)
	c.Assert(err, check.Equals, ErrReportedMessageNotFound)
}
