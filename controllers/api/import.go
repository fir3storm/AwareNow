package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/fir3storm/AwareNow/dialer"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/fir3storm/AwareNow/util"
	"github.com/jordan-wright/email"
)

type cloneRequest struct {
	URL              string `json:"url"`
	IncludeResources bool   `json:"include_resources"`
}

func (cr *cloneRequest) validate() error {
	if cr.URL == "" {
		return errors.New("No URL Specified")
	}
	return nil
}

type cloneResponse struct {
	HTML string `json:"html"`
}

type emailResponse struct {
	Text    string `json:"text"`
	HTML    string `json:"html"`
	Subject string `json:"subject"`
}

// ImportGroup imports a CSV of group members
func (as *Server) ImportGroup(w http.ResponseWriter, r *http.Request) {
	ts, err := util.ParseCSV(r)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error parsing CSV"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, ts, http.StatusOK)
}

// htmlRenderError marks a failure that occurred while re-serializing a
// goquery document back to HTML, as opposed to a failure parsing it. It
// lets callers preserve a distinct status code for that failure mode, as
// ImportEmail did before this logic was extracted into ParseEmailContent.
type htmlRenderError struct {
	err error
}

func (e *htmlRenderError) Error() string { return e.err.Error() }
func (e *htmlRenderError) Unwrap() error { return e.err }

// ParseEmailContent parses a raw RFC 822 email into its subject, text, and
// HTML parts. When convertLinks is true, all <a href> targets in the HTML
// body are rewritten to "{{.URL}}" so the result is ready to use as a
// phishing template.
func ParseEmailContent(content string, convertLinks bool) (subject, text, html string, err error) {
	e, perr := email.NewEmailFromReader(strings.NewReader(content))
	if perr != nil {
		// This failure is logged, not returned: parsing continues below
		// with whatever (possibly empty) *email.Email NewEmailFromReader
		// still returned, matching the pre-refactor ImportEmail behavior.
		log.Error(perr)
	}
	htmlBytes := e.HTML
	// If the caller wants to convert links to point to
	// the landing page, let's make it happen by changing up
	// the HTML body.
	if convertLinks {
		d, derr := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
		if derr != nil {
			return "", "", "", derr
		}
		d.Find("a").Each(func(i int, a *goquery.Selection) {
			a.SetAttr("href", "{{.URL}}")
		})
		h, herr := d.Html()
		if herr != nil {
			return "", "", "", &htmlRenderError{herr}
		}
		htmlBytes = []byte(h)
	}
	return e.Subject, string(e.Text), string(htmlBytes), nil
}

// ImportEmail allows for the importing of email.
// Returns a Message object
func (as *Server) ImportEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusBadRequest)
		return
	}
	ir := struct {
		Content      string `json:"content"`
		ConvertLinks bool   `json:"convert_links"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&ir)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error decoding JSON Request"}, http.StatusBadRequest)
		return
	}
	subject, text, html, err := ParseEmailContent(ir.Content, ir.ConvertLinks)
	if err != nil {
		status := http.StatusBadRequest
		var hre *htmlRenderError
		if errors.As(err, &hre) {
			status = http.StatusInternalServerError
			err = hre.err
		}
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, status)
		return
	}
	er := emailResponse{
		Subject: subject,
		Text:    text,
		HTML:    html,
	}
	JSONResponse(w, er, http.StatusOK)
}

// ImportSite allows for the importing of HTML from a website
// Without "include_resources" set, it will merely place a "base" tag
// so that all resources can be loaded relative to the given URL.
func (as *Server) ImportSite(w http.ResponseWriter, r *http.Request) {
	cr := cloneRequest{}
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusBadRequest)
		return
	}
	err := json.NewDecoder(r.Body).Decode(&cr)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Error decoding JSON Request"}, http.StatusBadRequest)
		return
	}
	if err = cr.validate(); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	restrictedDialer := dialer.Dialer()
	tr := &http.Transport{
		DialContext: restrictedDialer.DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(cr.URL)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	// Insert the base href tag to better handle relative resources
	d, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	// Assuming we don't want to include resources, we'll need a base href
	if d.Find("head base").Length() == 0 {
		d.Find("head").PrependHtml(fmt.Sprintf("<base href=\"%s\">", cr.URL))
	}
	forms := d.Find("form")
	forms.Each(func(i int, f *goquery.Selection) {
		// We'll want to store where we got the form from
		// (the current URL)
		url := f.AttrOr("action", cr.URL)
		if !strings.HasPrefix(url, "http") {
			url = fmt.Sprintf("%s%s", cr.URL, url)
		}
		f.PrependHtml(fmt.Sprintf("<input type=\"hidden\" name=\"__original_url\" value=\"%s\"/>", url))
	})
	h, err := d.Html()
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	cs := cloneResponse{HTML: h}
	JSONResponse(w, cs, http.StatusOK)
}
