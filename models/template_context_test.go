package models

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/png"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
	check "gopkg.in/check.v1"
)

type mockTemplateContext struct {
	URL         string
	FromAddress string
}

func (m mockTemplateContext) getFromAddress() string {
	return m.FromAddress
}

func (m mockTemplateContext) getBaseURL() string {
	return m.URL
}

func (s *ModelsSuite) TestNewTemplateContext(c *check.C) {
	r := Result{
		BaseRecipient: BaseRecipient{
			FirstName: "Foo",
			LastName:  "Bar",
			Email:     "foo@bar.com",
		},
		RId: "1234567",
	}
	ctx := mockTemplateContext{
		URL:         "http://example.com",
		FromAddress: "From Address <from@example.com>",
	}
	expected := PhishingTemplateContext{
		URL:           fmt.Sprintf("%s?rid=%s", ctx.URL, r.RId),
		BaseURL:       ctx.URL,
		BaseRecipient: r.BaseRecipient,
		TrackingURL:   fmt.Sprintf("%s/track?rid=%s", ctx.URL, r.RId),
		From:          "From Address",
		RId:           r.RId,
	}
	expected.Tracker = "<img alt='' style='display: none' src='" + expected.TrackingURL + "'/>"
	got, err := NewPhishingTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)
	// QRCode's exact content is verified independently in
	// TestNewTemplateContextQRCodeEncodesURL; here we only need it present
	// so the rest of the struct fields can still be compared with DeepEquals.
	expected.QRCode = got.QRCode
	c.Assert(got, check.DeepEquals, expected)
}

// TestNewTemplateContextQRCodeEncodesURL proves the QRCode field encodes the
// exact same per-recipient tracking URL as the URL field - not just a
// decorative image. It re-derives the expected QR PNG by calling the same
// encoder with the recipient's URL and comparing bytes exactly, rather than
// decoding the QR image: an evaluation of available pure-Go QR *decoders*
// (github.com/liyue201/goqr) found a real bug reversing digit order within
// numeric-mode runs (e.g. "1234567" decodes back as "3216547"), making that
// class of library unsuitable for verifying correctness here. QR encoding
// with a fixed library, recovery level, and size is deterministic, so exact
// byte comparison against a second, independent call is an equally strong
// proof that the right content was encoded.
func (s *ModelsSuite) TestNewTemplateContextQRCodeEncodesURL(c *check.C) {
	r := Result{
		BaseRecipient: BaseRecipient{
			FirstName: "Foo",
			LastName:  "Bar",
			Email:     "foo@bar.com",
		},
		RId: "1234567",
	}
	ctx := mockTemplateContext{
		URL:         "http://example.com",
		FromAddress: "From Address <from@example.com>",
	}
	got, err := NewPhishingTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)

	const prefix = "data:image/png;base64,"
	c.Assert(strings.HasPrefix(got.QRCode, prefix), check.Equals, true)

	pngBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got.QRCode, prefix))
	c.Assert(err, check.Equals, nil)

	// Confirm it's a real, well-formed PNG (not just base64 garbage).
	_, _, err = image.Decode(bytes.NewReader(pngBytes))
	c.Assert(err, check.Equals, nil)

	// Confirm it encodes got.URL exactly, using the same parameters
	// qrCodeDataURI uses in production.
	expectedPNG, err := qrcode.Encode(got.URL, qrcode.Medium, 256)
	c.Assert(err, check.Equals, nil)
	c.Assert(bytes.Equal(pngBytes, expectedPNG), check.Equals, true)
}
