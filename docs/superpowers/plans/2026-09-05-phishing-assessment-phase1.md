# Phishing Assessment Enhancement — Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close AwareNow's two biggest competitive gaps as an email-only phishing **assessment** tool — a real reporting loop (native report button + real/unknown-phish intake) and complete analytics export (PDF/XLSX) — without adding SMS or voice/vishing capability, per explicit scope decision on 2026-09-05.

**Architecture:** Two independent tracks that can run in parallel, plus one small sequential chain:
- **Track A (sequential, Tasks 1-4):** a new `ReportedMessage` intake path for *real, non-campaign* suspicious emails, landing on the existing public `PhishingServer` (`controllers/phish.go`), reviewed and converted into a draft `Template` through the admin API and a new web UI page.
- **Track B (independent, Task 5):** an Outlook add-in that calls AwareNow's **existing, already-wired** `/report` endpoint (`ReportHandler` in `controllers/phish.go`, calls `models.Result.HandleEmailReport`) — this endpoint already works and is CORS-open specifically for external reporting clients; nothing on the Go side is required for the known-campaign-report path, only a client.
- **Track C (independent, Tasks 6-7):** PDF and XLSX analytics export, replacing the current `501 Not Implemented` stubs in `controllers/api/analytics.go`.

**Tech Stack:** Go (existing stack: gorilla/mux, gorm/sqlite), vanilla JS + Office.js for the Outlook add-in (no new frontend build tooling), `github.com/xuri/excelize/v2` (XLSX, pure Go) and `github.com/go-pdf/fpdf` (PDF, pure Go) as new Go dependencies.

**Spec:** This plan implements Phase 1 (items A, B, C) of the enhancement roadmap discussed in-session on 2026-09-05, itself derived from a feature-by-feature comparison of AwareNow against KnowBe4, Proofpoint, Cofense, Hoxhunt, and Microsoft Defender ASR (chat record; no separate spec file exists). Scope explicitly excludes SMS/smishing delivery and voice/vishing per the user's instruction.

## Global Constraints

- No new cgo dependencies. This repo already has one cgo pain point (`bitbucket.org/liamstask/goose` → `sqlite3.Error`, see `docs/development.md`) that breaks local builds on machines without a C compiler (this dev machine included). Both new libraries (`excelize`, `go-pdf/fpdf`) are pure Go — verify this holds before adding anything else.
- Follow existing patterns: `models.Response{Success, Message}` + `api.JSONResponse(w, v, status)` for all JSON responses; `db.Save`/`gorm.ErrRecordNotFound` for persistence (see `models/enhanced_tracking.go` for the idiom); `mid.Use(handler, middlewares...)` for chained middleware (see `controllers/api/server.go`).
- The public, unauthenticated intake endpoint (Task 2) must be rate-limited — reuse `middleware/ratelimit.PostLimiter`, the same type already used to rate-limit `/login` in `controllers/route.go`.
- `go test ./...` is known-broken on this Windows dev machine for any package importing `models` (cgo sqlite3 issue, pre-existing, unrelated to this work — see `docs/development.md`). Verify new Go tests via `go vet` and code review locally; treat GitHub Actions (`ubuntu-latest`, has gcc) as the real test gate for anything under `models`/`controllers`.
- Keep this file updated as work ships: check off steps as completed, and append a dated line to the Progress Log below for every task that ships (merged/pushed), including what changed and any deviation from the plan as written.

---

## Progress Log

_Append one line per shipped task. Do not edit history above this point — add new lines below._

- 2026-09-05 — Plan created. No tasks shipped yet.

---

## Task 1: `ReportedMessage` data model

**Files:**
- Create: `models/reported_message.go`
- Test: `models/reported_message_test.go`

**Interfaces:**
- Produces: `models.ReportedMessage` struct; `models.CreateReportedMessage(rm *ReportedMessage) error`; `models.GetReportedMessages(status string) ([]ReportedMessage, error)`; `models.GetReportedMessageByID(id int64) (ReportedMessage, error)`; `models.ErrReportedMessageNotFound`; status constants `models.ReportedMessageStatusPending`, `models.ReportedMessageStatusApproved`, `models.ReportedMessageStatusRejected`.
- Consumes: nothing (foundational task).

- [ ] **Step 1: Write the failing test**

```go
// models/reported_message_test.go
package models

import "testing"

func TestCreateAndGetReportedMessage(t *testing.T) {
	setupTest(t)
	defer tearDown(t)

	rm := &ReportedMessage{
		ReporterEmail: "alice@example.com",
		Subject:       "Your invoice is overdue",
		BodyText:      "Please click here to pay",
		BodyHTML:      "<p>Please <a href=\"http://evil.example\">click here</a> to pay</p>",
	}
	if err := CreateReportedMessage(rm); err != nil {
		t.Fatalf("CreateReportedMessage failed: %v", err)
	}
	if rm.ID == 0 {
		t.Fatal("expected ID to be set after create")
	}
	if rm.Status != ReportedMessageStatusPending {
		t.Fatalf("expected default status %q, got %q", ReportedMessageStatusPending, rm.Status)
	}

	got, err := GetReportedMessageByID(rm.ID)
	if err != nil {
		t.Fatalf("GetReportedMessageByID failed: %v", err)
	}
	if got.Subject != rm.Subject {
		t.Fatalf("expected subject %q, got %q", rm.Subject, got.Subject)
	}

	pending, err := GetReportedMessages(ReportedMessageStatusPending)
	if err != nil {
		t.Fatalf("GetReportedMessages failed: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending message, got %d", len(pending))
	}
}

func TestGetReportedMessageByIDNotFound(t *testing.T) {
	setupTest(t)
	defer tearDown(t)

	_, err := GetReportedMessageByID(999)
	if err != ErrReportedMessageNotFound {
		t.Fatalf("expected ErrReportedMessageNotFound, got %v", err)
	}
}
```

(`setupTest`/`tearDown` are the existing per-test sqlite fixtures already used throughout `models/*_test.go` — check `models/models_test.go` for the exact names in use if these differ.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./models/ -run TestCreateAndGetReportedMessage -v`
Expected: FAIL — `ReportedMessage`, `CreateReportedMessage`, etc. undefined.

- [ ] **Step 3: Write the implementation**

```go
// models/reported_message.go
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
	ID                 int64     `json:"id" gorm:"column:id; primary_key:yes"`
	ReporterEmail      string    `json:"reporter_email" gorm:"column:reporter_email; not null"`
	Subject            string    `json:"subject" gorm:"column:subject"`
	BodyText           string    `json:"body_text" gorm:"column:body_text; sql:type:text"`
	BodyHTML           string    `json:"body_html" gorm:"column:body_html; sql:type:text"`
	Status             string    `json:"status" gorm:"column:status; not null"`
	ConvertedTemplateID int64    `json:"converted_template_id" gorm:"column:converted_template_id"`
	ReviewedBy         string    `json:"reviewed_by" gorm:"column:reviewed_by"`
	CreatedAt          time.Time `json:"created_at" gorm:"column:created_at"`
	ReviewedAt         time.Time `json:"reviewed_at" gorm:"column:reviewed_at"`
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
		"status":               status,
		"reviewed_by":          reviewedBy,
		"reviewed_at":          time.Now().UTC(),
		"converted_template_id": templateID,
	}
	err := db.Model(&ReportedMessage{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		log.Errorf("error updating reported message status: %v", err)
	}
	return err
}
```

- [ ] **Step 4: Register the new table with gorm's auto-migration**

Find where existing models (e.g. `DeviceFingerprint`, `BehaviorEvent` from `models/enhanced_tracking.go`) are passed to `db.AutoMigrate(...)` inside `models/models.go`'s `Setup`/migration function, and add `&ReportedMessage{}` to that list.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./models/ -run TestReportedMessage -v` (or the full `TestCreateAndGetReportedMessage` / `TestGetReportedMessageByIDNotFound` names)
Expected: PASS
Note: per Global Constraints, this may not build locally on a machine without gcc (cgo sqlite3 dependency) — if so, verify via `go vet ./models/` locally and let CI run the actual test.

- [ ] **Step 6: Commit**

```bash
git add models/reported_message.go models/reported_message_test.go models/models.go
git commit -m "feat: add ReportedMessage model for real-phish intake

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 2: Public intake endpoint + shared email-parsing helper

**Files:**
- Modify: `controllers/phish.go` (add `ReportUnknownHandler`, register route, add rate limiter field)
- Modify: `controllers/api/import.go` (extract reusable parsing function)
- Test: `controllers/phish_test.go`

**Interfaces:**
- Consumes: `models.CreateReportedMessage` (Task 1).
- Produces: `func ParseEmailContent(content string, convertLinks bool) (subject, text, html string, err error)` in package `api` (moved out of the `ImportEmail` handler body so Task 3 can reuse it) — **exported** so `controllers` can call `api.ParseEmailContent`. `POST /report-unknown` on `PhishingServer`, body `{"reporter_email": string, "subject": string, "body_text": string, "body_html": string}`, always responds `204 No Content` on success (mirrors `ReportHandler`'s existing style) or `400`/`429` on failure.

- [ ] **Step 1: Extract the shared parser (refactor, no behavior change)**

In `controllers/api/import.go`, pull the body of `ImportEmail` (from `email.NewEmailFromReader` through the `ConvertLinks` goquery rewrite) into:

```go
// ParseEmailContent parses a raw RFC 822 email into its subject, text, and
// HTML parts. When convertLinks is true, all <a href> targets in the HTML
// body are rewritten to "{{.URL}}" so the result is ready to use as a
// phishing template.
func ParseEmailContent(content string, convertLinks bool) (subject, text, html string, err error) {
	e, err := email.NewEmailFromReader(strings.NewReader(content))
	if err != nil {
		return "", "", "", err
	}
	htmlBytes := e.HTML
	if convertLinks {
		d, derr := goquery.NewDocumentFromReader(bytes.NewReader(e.HTML))
		if derr != nil {
			return "", "", "", derr
		}
		d.Find("a").Each(func(i int, a *goquery.Selection) {
			a.SetAttr("href", "{{.URL}}")
		})
		h, herr := d.Html()
		if herr != nil {
			return "", "", "", herr
		}
		htmlBytes = []byte(h)
	}
	return e.Subject, string(e.Text), string(htmlBytes), nil
}
```

Then rewrite `ImportEmail` to call it:

```go
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
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	JSONResponse(w, emailResponse{Subject: subject, Text: text, HTML: html}, http.StatusOK)
}
```

- [ ] **Step 2: Run existing import tests to confirm no regression**

Run: `go test ./controllers/api/ -run TestImportEmail -v`
Expected: PASS (behavior unchanged, only refactored)

- [ ] **Step 3: Add the rate limiter field and route to `PhishingServer`**

In `controllers/phish.go`, add a limiter field and wire the new route:

```go
type PhishingServer struct {
	server         *http.Server
	config         config.PhishServer
	contactAddress string
	limiter        *ratelimit.PostLimiter // add this field
}
```

```go
// in NewPhishingServer, before ps.registerRoutes():
ps.limiter = ratelimit.NewPostLimiter()
```

```go
// in registerRoutes():
router.HandleFunc("/report-unknown", mid.Use(ps.ReportUnknownHandler, ps.limiter.Limit)).Methods("POST")
```

Add imports: `"github.com/fir3storm/AwareNow/middleware/ratelimit"` and confirm `mid` (already imported as `"github.com/fir3storm/AwareNow/middleware"`) covers `mid.Use`.

- [ ] **Step 4: Write the failing test**

```go
// in controllers/phish_test.go — follow the existing suite's setup pattern
// (check_test.go / gocheck style already used in this file)
func (s *ControllersSuite) TestReportUnknownHandler(c *check.C) {
	body := `{"reporter_email":"alice@example.com","subject":"Urgent: verify your account","body_text":"click here","body_html":"<p>click <a href=\"http://evil.example\">here</a></p>"}`
	req, err := http.NewRequest("POST", "/report-unknown", strings.NewReader(body))
	c.Assert(err, check.Equals, nil)
	req.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	s.phishingServer.ServeHTTP(response, req)
	c.Assert(response.Code, check.Equals, http.StatusNoContent)

	msgs, err := models.GetReportedMessages(models.ReportedMessageStatusPending)
	c.Assert(err, check.Equals, nil)
	c.Assert(len(msgs), check.Equals, 1)
	c.Assert(msgs[0].ReporterEmail, check.Equals, "alice@example.com")
}
```

(Match this to whatever the existing `ControllersSuite` fixture/handler-access pattern is in `controllers/phish_test.go` and `controllers/controllers_test.go` — adjust field/method names to what's actually there rather than inventing new suite plumbing.)

- [ ] **Step 5: Run test to verify it fails**

Run: `go test ./controllers/ -run TestReportUnknownHandler -v`
Expected: FAIL — `ReportUnknownHandler` undefined.

- [ ] **Step 6: Implement `ReportUnknownHandler`**

```go
// ReportUnknownHandler accepts a report of a real, non-campaign suspicious
// email (one with no AwareNow tracking rid). It stores the report for admin
// review; it does not touch any Result or campaign.
func (ps *PhishingServer) ReportUnknownHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // same rationale as ReportHandler: external reporting clients
	var payload struct {
		ReporterEmail string `json:"reporter_email"`
		Subject       string `json:"subject"`
		BodyText      string `json:"body_text"`
		BodyHTML      string `json:"body_html"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if payload.ReporterEmail == "" || (payload.BodyText == "" && payload.BodyHTML == "") {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	rm := &models.ReportedMessage{
		ReporterEmail: payload.ReporterEmail,
		Subject:       payload.Subject,
		BodyText:      payload.BodyText,
		BodyHTML:      payload.BodyHTML,
	}
	if err := models.CreateReportedMessage(rm); err != nil {
		log.Error(err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Add `"encoding/json"` to `controllers/phish.go`'s imports if not already present (it is not, per the file as read for this plan).

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./controllers/ -run TestReportUnknownHandler -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add controllers/phish.go controllers/api/import.go controllers/phish_test.go
git commit -m "feat: add public intake endpoint for real (non-campaign) phishing reports

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 3: Admin review API — list / approve / reject

**Files:**
- Create: `controllers/api/reported_message.go`
- Modify: `controllers/api/server.go` (register routes)
- Test: `controllers/api/reported_message_test.go`

**Interfaces:**
- Consumes: `models.GetReportedMessages`, `models.GetReportedMessageByID`, `models.UpdateReportedMessageStatus` (Task 1); `api.ParseEmailContent` (Task 2); existing `models.PostTemplate` / template creation function (confirm exact signature in `models/template.go` before wiring — it takes a `*models.Template` and a user ID in every other handler in `controllers/api/template.go`, follow that exact pattern).
- Produces: `GET /api/reported-messages/`, `GET /api/reported-messages/{id:[0-9]+}`, `POST /api/reported-messages/{id:[0-9]+}/approve`, `POST /api/reported-messages/{id:[0-9]+}/reject` — all behind the existing `mid.RequireAPIKey` + `mid.RequirePermission(models.PermissionModifyObjects)` used elsewhere in `server.go`.

- [ ] **Step 1: Write the failing test for list + approve**

```go
// controllers/api/reported_message_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fir3storm/AwareNow/models"
)

func TestReportedMessagesApprove(t *testing.T) {
	// follow the existing setup used in api_test.go (setupTest/apiKey helpers)
	rm := &models.ReportedMessage{
		ReporterEmail: "bob@example.com",
		Subject:       "Test",
		BodyHTML:      "<p><a href=\"http://evil.example\">link</a></p>",
	}
	if err := models.CreateReportedMessage(rm); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/reported-messages/"+itoa(rm.ID)+"/approve", strings.NewReader(`{"name":"From report: Test"}`))
	// attach API key header the same way other tests in this package do
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req) // apiServer: reuse whatever package-level test server api_test.go already builds

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, err := models.GetReportedMessageByID(rm.ID)
	if err != nil {
		t.Fatalf("GetReportedMessageByID failed: %v", err)
	}
	if got.Status != models.ReportedMessageStatusApproved {
		t.Fatalf("expected approved, got %s", got.Status)
	}
	if got.ConvertedTemplateID == 0 {
		t.Fatal("expected ConvertedTemplateID to be set")
	}
}
```

Adjust the request-construction/auth boilerplate to match whatever `controllers/api/api_test.go` already sets up for other authenticated-endpoint tests (there is an existing pattern there — do not invent a second one).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./controllers/api/ -run TestReportedMessagesApprove -v`
Expected: FAIL — route not registered / 404.

- [ ] **Step 3: Implement the handlers**

```go
// controllers/api/reported_message.go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/fir3storm/AwareNow/context"
	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/gorilla/mux"
)

// ReportedMessages returns all reported messages, optionally filtered by
// the ?status= query parameter.
// GET /api/reported-messages/
func (as *Server) ReportedMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	status := r.URL.Query().Get("status")
	msgs, err := models.GetReportedMessages(status)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error retrieving reported messages"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, msgs, http.StatusOK)
}

// ReportedMessage returns a single reported message by ID.
// GET /api/reported-messages/{id}
func (as *Server) ReportedMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	rm, err := models.GetReportedMessageByID(id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Reported message not found"}, http.StatusNotFound)
		return
	}
	JSONResponse(w, rm, http.StatusOK)
}

// ReportedMessageApprove converts a reported message into a new draft
// template and marks it approved.
// POST /api/reported-messages/{id}/approve
// Body: {"name": "<template name>"}
func (as *Server) ReportedMessageApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	rm, err := models.GetReportedMessageByID(id)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Reported message not found"}, http.StatusNotFound)
		return
	}
	if rm.Status != models.ReportedMessageStatusPending {
		JSONResponse(w, models.Response{Success: false, Message: "Reported message already reviewed"}, http.StatusConflict)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		body.Name = "From report: " + rm.Subject
	}

	source := rm.BodyHTML
	convertLinks := rm.BodyHTML != ""
	_, text, html, err := ParseEmailContent(rawEmailFromParts(rm.Subject, rm.BodyText, source), convertLinks)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error converting message to template"}, http.StatusInternalServerError)
		return
	}

	uid := ctx.Get(r, "user_id").(int64)
	tmpl := models.Template{
		UserId: uid,
		Name:   body.Name,
		Subject: rm.Subject,
		Text:   text,
		HTML:   html,
	}
	// Follow whatever models.PostTemplate(&tmpl) / models.Template validation
	// signature controllers/api/template.go already uses for POST /templates/ —
	// call the same function here instead of duplicating validation logic.
	if err := models.PostTemplate(&tmpl); err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}

	reviewedBy := ctx.Get(r, "user").(models.User).Username
	if err := models.UpdateReportedMessageStatus(id, models.ReportedMessageStatusApproved, reviewedBy, tmpl.Id); err != nil {
		log.Error(err)
	}
	JSONResponse(w, tmpl, http.StatusOK)
}

// ReportedMessageReject dismisses a reported message without creating a template.
// POST /api/reported-messages/{id}/reject
func (as *Server) ReportedMessageReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid ID"}, http.StatusBadRequest)
		return
	}
	reviewedBy := ctx.Get(r, "user").(models.User).Username
	if err := models.UpdateReportedMessageStatus(id, models.ReportedMessageStatusRejected, reviewedBy, 0); err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error updating reported message"}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Reported message rejected"}, http.StatusOK)
}
```

`rawEmailFromParts` is a small local helper needed because `ParseEmailContent` takes a raw RFC 822 message, but a `ReportedMessage` stores already-split subject/text/html. Add it in the same file:

```go
// rawEmailFromParts builds a minimal raw RFC 822 message from already-split
// fields so it can be run back through ParseEmailContent's link-rewriting
// logic without duplicating that logic.
func rawEmailFromParts(subject, text, html string) string {
	if html != "" {
		return "Subject: " + subject + "\r\nContent-Type: text/html\r\n\r\n" + html
	}
	return "Subject: " + subject + "\r\nContent-Type: text/plain\r\n\r\n" + text
}
```

**Before writing this file for real:** open `models/template.go` and `controllers/api/template.go` to confirm the exact `Template` struct field names and the exact template-creation function signature (`PostTemplate` is a guess based on the `PostCampaign`/`PostCampaignSMTP` naming convention seen elsewhere in `models/`) — adjust the calls above to match what's actually there rather than what's assumed here.

- [ ] **Step 4: Register routes**

In `controllers/api/server.go`, inside `registerRoutes()`, add alongside the other `RequirePermission` routes:

```go
router.HandleFunc("/reported-messages/", mid.Use(as.ReportedMessages, mid.RequirePermission(models.PermissionModifyObjects)))
router.HandleFunc("/reported-messages/{id:[0-9]+}", mid.Use(as.ReportedMessage, mid.RequirePermission(models.PermissionModifyObjects)))
router.HandleFunc("/reported-messages/{id:[0-9]+}/approve", mid.Use(as.ReportedMessageApprove, mid.RequirePermission(models.PermissionModifyObjects))).Methods("POST")
router.HandleFunc("/reported-messages/{id:[0-9]+}/reject", mid.Use(as.ReportedMessageReject, mid.RequirePermission(models.PermissionModifyObjects))).Methods("POST")
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./controllers/api/ -run TestReportedMessages -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add controllers/api/reported_message.go controllers/api/server.go controllers/api/reported_message_test.go
git commit -m "feat: add admin review API for reported phishing messages

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 4: Frontend — Reported Messages review page

**Files:**
- Create: `web/src/api/reportedMessages.ts`
- Create: `web/src/pages/ReportedMessages/ReportedMessageList.tsx`
- Modify: wherever the sidebar/route table lives (check `web/src/App.tsx` for the existing route list and nav — follow its exact pattern, e.g. how `Groups`/`Templates` are wired)
- Test: `web/src/pages/ReportedMessages/ReportedMessageList.test.tsx`

**Interfaces:**
- Consumes: `GET/POST /api/reported-messages/...` (Task 3). Follow `web/src/api/client.ts`'s existing axios instance and `web/src/api/templates.ts`'s existing call style exactly — read both before writing this task's code.
- Produces: a route (path TBD to match existing convention, e.g. `/reported-messages`) reachable from the nav.

- [ ] **Step 1: Read the existing patterns first**

Open `web/src/api/templates.ts`, `web/src/pages/Templates/TemplateList.tsx`, and `web/src/App.tsx` in full. This task's list/detail/approve/reject page should structurally mirror `TemplateList.tsx` (same data-fetching hook style — likely `@tanstack/react-query`, same table component, same loading/error states) rather than introducing a new pattern.

- [ ] **Step 2: Write the API client**

```ts
// web/src/api/reportedMessages.ts
import { apiClient } from './client'; // match the actual export name in client.ts

export type ReportedMessageStatus = 'pending' | 'approved' | 'rejected';

export interface ReportedMessage {
  id: number;
  reporter_email: string;
  subject: string;
  body_text: string;
  body_html: string;
  status: ReportedMessageStatus;
  converted_template_id: number;
  reviewed_by: string;
  created_at: string;
  reviewed_at: string;
}

export async function listReportedMessages(status?: ReportedMessageStatus) {
  const params = status ? { status } : undefined;
  const { data } = await apiClient.get<ReportedMessage[]>('/reported-messages/', { params });
  return data;
}

export async function approveReportedMessage(id: number, name: string) {
  const { data } = await apiClient.post(`/reported-messages/${id}/approve`, { name });
  return data;
}

export async function rejectReportedMessage(id: number) {
  const { data } = await apiClient.post(`/reported-messages/${id}/reject`);
  return data;
}
```

(Adjust the exact axios wrapper import/usage to whatever `web/src/api/templates.ts` actually does — this is illustrative of the shape, not a guaranteed match to the real client helper's name.)

- [ ] **Step 3: Write the failing component test**

```tsx
// web/src/pages/ReportedMessages/ReportedMessageList.test.tsx
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ReportedMessageList } from './ReportedMessageList';
import * as api from '../../api/reportedMessages';

vi.spyOn(api, 'listReportedMessages').mockResolvedValue([
  {
    id: 1,
    reporter_email: 'alice@example.com',
    subject: 'Urgent: verify your account',
    body_text: '',
    body_html: '',
    status: 'pending',
    converted_template_id: 0,
    reviewed_by: '',
    created_at: '2026-09-05T00:00:00Z',
    reviewed_at: '',
  },
]);

describe('ReportedMessageList', () => {
  it('renders a pending reported message', async () => {
    render(<ReportedMessageList />);
    expect(await screen.findByText('Urgent: verify your account')).toBeInTheDocument();
    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run test to verify it fails**

Run: `cd web && npx vitest run src/pages/ReportedMessages/ReportedMessageList.test.tsx`
Expected: FAIL — module does not exist.

- [ ] **Step 5: Implement the component**

Build `ReportedMessageList.tsx` following `TemplateList.tsx`'s exact structure (query hook, table, loading/error states), with columns: reporter email, subject, reported date, status, and Approve/Reject action buttons that call `approveReportedMessage`/`rejectReportedMessage` and invalidate the list query on success. Do not invent a new table/loading pattern — copy the established one.

- [ ] **Step 6: Wire the route and nav entry**

Add the route/nav entry in `web/src/App.tsx` following the exact pattern used for the existing pages there.

- [ ] **Step 7: Run tests to verify they pass**

Run: `cd web && npx vitest run src/pages/ReportedMessages/ReportedMessageList.test.tsx`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add web/src/api/reportedMessages.ts web/src/pages/ReportedMessages/ web/src/App.tsx
git commit -m "feat: add Reported Messages review page to web UI

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 5: Outlook add-in — one-click report button (independent of Tasks 1-4)

**Files:**
- Create: `addins/outlook-report-button/manifest.xml`
- Create: `addins/outlook-report-button/commands.html`
- Create: `addins/outlook-report-button/src/extractRid.js`
- Create: `addins/outlook-report-button/src/commands.js`
- Create: `addins/outlook-report-button/src/extractRid.test.js`
- Create: `addins/outlook-report-button/package.json`
- Create: `addins/outlook-report-button/README.md`

**Interfaces:**
- Consumes: the **existing** `GET/POST /report?rid=<7-char-id>` endpoint (`controllers/phish.go`'s `ReportHandler`, already implemented, already CORS-open) for known campaign emails; the new `POST /report-unknown` endpoint (Task 2) for anything else.
- Produces: a side-loadable Office Add-in manifest an admin can deploy org-wide via Microsoft 365 admin center (Integrated Apps) or side-load individually for testing.

- [ ] **Step 1: Write the failing unit test for rid extraction**

```js
// addins/outlook-report-button/src/extractRid.test.js
import { describe, it, expect } from 'vitest';
import { extractRid } from './extractRid.js';

describe('extractRid', () => {
  it('extracts a 7-char rid from a query-string-style link', () => {
    const body = 'Please review your invoice: https://itsupport.insec.in/invoice?rid=AbC1234';
    expect(extractRid(body)).toBe('AbC1234');
  });

  it('extracts a rid joined with &', () => {
    const body = 'Click here: https://example.com/x?foo=bar&rid=Zz98765';
    expect(extractRid(body)).toBe('Zz98765');
  });

  it('returns null when no rid is present', () => {
    const body = 'This is a completely unrelated real email with no tracking link.';
    expect(extractRid(body)).toBeNull();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd addins/outlook-report-button && npx vitest run src/extractRid.test.js`
Expected: FAIL — module does not exist.

- [ ] **Step 3: Implement `extractRid`**

```js
// addins/outlook-report-button/src/extractRid.js
// Mirrors the rid pattern imap/monitor.go already looks for
// (7-character alphanumeric AwareNow tracking ID, ?rid= or &rid=).
const RID_PATTERN = /[?&]rid=([A-Za-z0-9]{7})\b/;

export function extractRid(text) {
  if (!text) return null;
  const match = RID_PATTERN.exec(text);
  return match ? match[1] : null;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd addins/outlook-report-button && npx vitest run src/extractRid.test.js`
Expected: PASS

- [ ] **Step 5: Add minimal package.json for the test runner**

```json
{
  "name": "@awarenow/outlook-report-button",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "test": "vitest run"
  },
  "devDependencies": {
    "vitest": "^3.2.4"
  }
}
```

- [ ] **Step 6: Write `commands.js`, the add-in's actual behavior**

```js
// addins/outlook-report-button/src/commands.js
import { extractRid } from './extractRid.js';

// Admin sets this once per deployment (see README) — the org's AwareNow
// phishing-server base URL, e.g. "https://itsupport.insec.in:8082".
const SERVER_URL_SETTING = 'awarenowServerUrl';

Office.onReady(() => {
  // Office.js requires this call before any Office API is used, even
  // though this add-in has no UI beyond a ribbon button.
});

function getServerUrl() {
  return Office.context.roamingSettings.get(SERVER_URL_SETTING) || '';
}

function reportPhishing(event) {
  const item = Office.context.mailbox.item;
  const serverUrl = getServerUrl();

  if (!serverUrl) {
    Office.context.mailbox.item.notificationMessages.replaceAsync('awarenow-config', {
      type: 'errorMessage',
      message: 'AwareNow server URL is not configured. Ask your admin to set it via Outlook add-in settings.',
    });
    event.completed();
    return;
  }

  item.body.getAsync(Office.CoercionType.Text, (bodyResult) => {
    const bodyText = bodyResult.status === Office.AsyncResultStatus.Succeeded ? bodyResult.value : '';
    const rid = extractRid(bodyText);

    const done = (message) => {
      Office.context.mailbox.item.notificationMessages.replaceAsync('awarenow-report', {
        type: 'informationalMessage',
        message,
        icon: 'icon1',
        persistent: false,
      });
      event.completed();
    };

    if (rid) {
      fetch(`${serverUrl}/report?rid=${encodeURIComponent(rid)}`, { method: 'POST', mode: 'cors' })
        .then(() => done('Thanks — this simulated phishing email was reported.'))
        .catch(() => done('Could not reach the AwareNow server. Try again later.'));
      return;
    }

    item.subject.getAsync((subjectResult) => {
      const subject = subjectResult.status === Office.AsyncResultStatus.Succeeded ? subjectResult.value : '';
      fetch(`${serverUrl}/report-unknown`, {
        method: 'POST',
        mode: 'cors',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          reporter_email: Office.context.mailbox.userProfile.emailAddress,
          subject,
          body_text: bodyText,
        }),
      })
        .then(() => done('Thanks — this email was reported to your security team for review.'))
        .catch(() => done('Could not reach the AwareNow server. Try again later.'));
    });
  });
}

// Register the function so the ribbon button (declared in manifest.xml) can invoke it.
Office.actions = Office.actions || {};
Office.actions.associate('reportPhishing', reportPhishing);
```

- [ ] **Step 7: Write `commands.html`, the function-file host page**

```html
<!doctype html>
<html>
<head>
  <meta charset="UTF-8" />
  <title>AwareNow Report Button Commands</title>
  <script src="https://appsforoffice.microsoft.com/lib/1/hosted/office.js"></script>
  <script type="module" src="./src/commands.js"></script>
</head>
<body></body>
</html>
```

- [ ] **Step 8: Write `manifest.xml`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<OfficeApp
    xmlns="http://schemas.microsoft.com/office/appforoffice/1.1"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xmlns:bt="http://schemas.microsoft.com/office/officeappbasictypes/1.0"
    xmlns:mailappor="http://schemas.microsoft.com/office/mailappversionoverrides/1.0"
    xsi:type="MailApp">
  <Id>a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c</Id>
  <Version>1.0.0.0</Version>
  <ProviderName>AwareNow</ProviderName>
  <DefaultLocale>en-US</DefaultLocale>
  <DisplayName DefaultValue="Report Phishing" />
  <Description DefaultValue="Report a suspicious or simulated phishing email to AwareNow with one click." />
  <IconUrl DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-32.png" />
  <HighResolutionIconUrl DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-80.png" />
  <SupportUrl DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST" />
  <AppDomains>
    <AppDomain>https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST</AppDomain>
  </AppDomains>
  <Hosts>
    <Host Name="Mailbox" />
  </Hosts>
  <Requirements>
    <Sets>
      <Set Name="Mailbox" MinVersion="1.5" />
    </Sets>
  </Requirements>
  <FormSettings>
    <Form xsi:type="ItemRead">
      <DesktopSettings>
        <SourceLocation DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/commands.html" />
      </DesktopSettings>
    </Form>
  </FormSettings>
  <Permissions>ReadItem</Permissions>
  <Rule xsi:type="RuleCollection" Mode="Or">
    <Rule xsi:type="ItemIs" ItemType="Message" FormType="Read" />
  </Rule>
  <VersionOverrides xmlns="http://schemas.microsoft.com/office/mailappversionoverrides" xsi:type="VersionOverridesV1_0">
    <Requirements>
      <bt:Sets DefaultMinVersion="1.3">
        <bt:Set Name="Mailbox" />
      </bt:Sets>
    </Requirements>
    <Hosts>
      <Host xsi:type="MailHost">
        <DesktopFormFactor>
          <FunctionFile resid="commands.url" />
          <ExtensionPoint xsi:type="MessageReadCommandSurface">
            <OfficeTab id="TabDefault">
              <Group id="awarenowGroup">
                <Label resid="groupLabel" />
                <Control xsi:type="Button" id="reportPhishingButton">
                  <Label resid="buttonLabel" />
                  <Supertip>
                    <Title resid="buttonLabel" />
                    <Description resid="buttonTooltip" />
                  </Supertip>
                  <Icon>
                    <bt:Image size="16" resid="icon16" />
                    <bt:Image size="32" resid="icon32" />
                    <bt:Image size="80" resid="icon80" />
                  </Icon>
                  <Action xsi:type="ExecuteFunction">
                    <FunctionName>reportPhishing</FunctionName>
                  </Action>
                </Control>
              </Group>
            </OfficeTab>
          </ExtensionPoint>
        </DesktopFormFactor>
      </Host>
    </Hosts>
    <Resources>
      <bt:Images>
        <bt:Image id="icon16" DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-16.png" />
        <bt:Image id="icon32" DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-32.png" />
        <bt:Image id="icon80" DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/icon-80.png" />
      </bt:Images>
      <bt:Urls>
        <bt:Url id="commands.url" DefaultValue="https://REPLACE_WITH_YOUR_DEPLOYMENT_HOST/addins/outlook-report-button/commands.html" />
      </bt:Urls>
      <bt:ShortStrings>
        <bt:String id="groupLabel" DefaultValue="AwareNow" />
        <bt:String id="buttonLabel" DefaultValue="Report Phishing" />
      </bt:ShortStrings>
      <bt:LongStrings>
        <bt:String id="buttonTooltip" DefaultValue="Report this email as suspicious or as a simulated phishing test." />
      </bt:LongStrings>
    </Resources>
  </VersionOverrides>
</OfficeApp>
```

- [ ] **Step 9: Write the README covering the two things this plan cannot automate**

```markdown
# AwareNow Outlook Report Button

## Deploy checklist (manual — cannot be scripted from this repo)

1. Replace every `REPLACE_WITH_YOUR_DEPLOYMENT_HOST` in `manifest.xml` with
   this deployment's actual public HTTPS host serving this add-in's static
   files (commands.html, src/, icons). Serve them from the admin server's
   existing static-file path or a small dedicated host — this add-in has
   no server-side component beyond the two endpoints it calls.
2. Validate the manifest: `npx office-addin-manifest validate manifest.xml`
3. Side-load for testing: Outlook desktop → Get Add-ins → My add-ins →
   Add a custom add-in → Add from file → select `manifest.xml`.
4. For org-wide rollout: Microsoft 365 admin center → Settings →
   Integrated apps → Upload custom apps.
5. Each end user (or an admin, via Office.js roaming settings pushed at
   deploy time) must set the AwareNow server URL once — there is
   currently no settings UI for this in v1; the quickest path is a
   one-time `Office.context.roamingSettings.set('awarenowServerUrl', '<url>')`
   run from the browser console while the add-in is loaded, or add a
   proper settings dialog as a fast-follow.
```

- [ ] **Step 10: Commit**

```bash
git add addins/outlook-report-button/
git commit -m "feat: add Outlook one-click report button add-in

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 6: PDF analytics export (independent)

**Files:**
- Modify: `go.mod`, `go.sum` (add `github.com/go-pdf/fpdf`)
- Create: `controllers/api/export_pdf.go`
- Modify: `controllers/api/analytics.go` (`ExportAnalytics`'s `case "pdf"`)
- Test: `controllers/api/export_pdf_test.go`

**Interfaces:**
- Consumes: `models.GetAnalyticsOverview`, `models.GetOverallTimeline`, `models.GetDepartmentStats`, `models.GetRiskScore` (all already exist, used identically in the existing `exportCSV`/`generateCSVFromAnalytics` in `controllers/api/analytics.go` — reuse those exact calls).
- Produces: `func exportPDF(w http.ResponseWriter, r *http.Request, uid int64)` matching the existing `exportCSV`/`exportJSON` signature pattern.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/go-pdf/fpdf@latest
```

- [ ] **Step 2: Write the failing test**

```go
// controllers/api/export_pdf_test.go
package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExportAnalyticsPDF(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/analytics/export?format=pdf", nil)
	// attach auth the same way other tests in this package do
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf, got %s", ct)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF")) {
		t.Fatal("expected response body to start with the PDF magic bytes")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./controllers/api/ -run TestExportAnalyticsPDF -v`
Expected: FAIL — still returns 501 per the current `case "pdf", "xlsx":` branch.

- [ ] **Step 4: Implement `exportPDF`**

```go
// controllers/api/export_pdf.go
package api

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/go-pdf/fpdf"
)

func exportPDF(w http.ResponseWriter, r *http.Request, uid int64) {
	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
		return
	}
	depts, err := models.GetDepartmentStats(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting department stats"}, http.StatusInternalServerError)
		return
	}
	risk, err := models.GetRiskScore(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting risk score"}, http.StatusInternalServerError)
		return
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "AwareNow Analytics Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Generated "+time.Now().UTC().Format(time.RFC1123), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Overview", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	rows := [][2]string{
		{"Total Campaigns", fmt.Sprintf("%d", overview.TotalCampaigns)},
		{"Emails Sent", fmt.Sprintf("%d", overview.EmailsSent)},
		{"Open Rate", fmt.Sprintf("%.2f%%", overview.OpenRate)},
		{"Click Rate", fmt.Sprintf("%.2f%%", overview.ClickRate)},
		{"Submit Rate", fmt.Sprintf("%.2f%%", overview.SubmitRate)},
		{"Report Rate", fmt.Sprintf("%.2f%%", overview.ReportRate)},
		{"Risk Score", fmt.Sprintf("%d (%s)", risk.Score, risk.Level)},
	}
	for _, row := range rows {
		pdf.CellFormat(60, 6, row[0], "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, row[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Department Statistics", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(70, 6, "Department", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Users", "1", 0, "L", false, 0, "")
	pdf.CellFormat(40, 6, "Click Rate", "1", 0, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Submit Rate", "1", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	for _, d := range depts {
		pdf.CellFormat(70, 6, d.Department, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, fmt.Sprintf("%d", d.UsersCount), "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 6, fmt.Sprintf("%.2f%%", d.ClickRate), "1", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("%.2f%%", d.SubmitRate), "1", 1, "L", false, 0, "")
	}

	filename := fmt.Sprintf("analytics_export_%s.pdf", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if err := pdf.Output(w); err != nil {
		log.Errorf("error writing PDF output: %v", err)
	}
}
```

- [ ] **Step 5: Wire it into `ExportAnalytics`**

In `controllers/api/analytics.go`, change:

```go
case "pdf", "xlsx":
	JSONResponse(w, models.Response{Success: false, Message: fmt.Sprintf("Format '%s' export not yet implemented. Use 'csv' or 'json'.", format)}, http.StatusNotImplemented)
```

to:

```go
case "pdf":
	exportPDF(w, r, uid)
case "xlsx":
	exportXLSX(w, r, uid) // implemented in Task 7 — until that task lands, leave this line out and keep xlsx routed to the NotImplemented branch
```

(If Task 7 hasn't shipped yet when this task ships, keep `"xlsx"` in the not-implemented branch and only pull `"pdf"` out of it — do not reference `exportXLSX` before it exists.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./controllers/api/ -run TestExportAnalyticsPDF -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum controllers/api/export_pdf.go controllers/api/analytics.go controllers/api/export_pdf_test.go
git commit -m "feat: implement PDF analytics export

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Task 7: XLSX analytics export (independent)

**Files:**
- Modify: `go.mod`, `go.sum` (add `github.com/xuri/excelize/v2`)
- Create: `controllers/api/export_xlsx.go`
- Modify: `controllers/api/analytics.go` (`ExportAnalytics`'s `case "xlsx"`, same edit point as Task 6 — coordinate order of landing with whichever of Task 6/7 merges second)
- Test: `controllers/api/export_xlsx_test.go`

**Interfaces:**
- Consumes: same analytics model functions as Task 6, plus `models.GetOverallTimeline`.
- Produces: `func exportXLSX(w http.ResponseWriter, r *http.Request, uid int64)`.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/xuri/excelize/v2@latest
```

- [ ] **Step 2: Write the failing test**

```go
// controllers/api/export_xlsx_test.go
package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExportAnalyticsXLSX(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/analytics/export?format=xlsx", nil)
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	wantCT := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if ct := w.Header().Get("Content-Type"); ct != wantCT {
		t.Fatalf("expected Content-Type %s, got %s", wantCT, ct)
	}
	// XLSX files are zip archives; PK\x03\x04 is the zip local-file-header magic.
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Fatal("expected response body to start with the zip/xlsx magic bytes")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./controllers/api/ -run TestExportAnalyticsXLSX -v`
Expected: FAIL

- [ ] **Step 4: Implement `exportXLSX`**

```go
// controllers/api/export_xlsx.go
package api

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/xuri/excelize/v2"
)

func exportXLSX(w http.ResponseWriter, r *http.Request, uid int64) {
	overview, err := models.GetAnalyticsOverview(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting data"}, http.StatusInternalServerError)
		return
	}
	timeline, err := models.GetOverallTimeline(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting timeline"}, http.StatusInternalServerError)
		return
	}
	depts, err := models.GetDepartmentStats(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting department stats"}, http.StatusInternalServerError)
		return
	}
	risk, err := models.GetRiskScore(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Error exporting risk score"}, http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	const overviewSheet = "Overview"
	f.SetSheetName("Sheet1", overviewSheet)
	overviewRows := [][2]interface{}{
		{"Total Campaigns", overview.TotalCampaigns},
		{"Emails Sent", overview.EmailsSent},
		{"Open Rate (%)", overview.OpenRate},
		{"Click Rate (%)", overview.ClickRate},
		{"Submit Rate (%)", overview.SubmitRate},
		{"Report Rate (%)", overview.ReportRate},
		{"Risk Score", risk.Score},
		{"Risk Level", risk.Level},
	}
	for i, row := range overviewRows {
		f.SetCellValue(overviewSheet, fmt.Sprintf("A%d", i+1), row[0])
		f.SetCellValue(overviewSheet, fmt.Sprintf("B%d", i+1), row[1])
	}

	const timelineSheet = "Timeline"
	f.NewSheet(timelineSheet)
	f.SetCellValue(timelineSheet, "A1", "Date")
	f.SetCellValue(timelineSheet, "B1", "Opens")
	f.SetCellValue(timelineSheet, "C1", "Clicks")
	f.SetCellValue(timelineSheet, "D1", "Submits")
	for i, t := range timeline {
		row := i + 2
		f.SetCellValue(timelineSheet, fmt.Sprintf("A%d", row), t.Date)
		f.SetCellValue(timelineSheet, fmt.Sprintf("B%d", row), t.Opens)
		f.SetCellValue(timelineSheet, fmt.Sprintf("C%d", row), t.Clicks)
		f.SetCellValue(timelineSheet, fmt.Sprintf("D%d", row), t.Submits)
	}

	const deptSheet = "Departments"
	f.NewSheet(deptSheet)
	f.SetCellValue(deptSheet, "A1", "Department")
	f.SetCellValue(deptSheet, "B1", "Users")
	f.SetCellValue(deptSheet, "C1", "Click Rate (%)")
	f.SetCellValue(deptSheet, "D1", "Submit Rate (%)")
	for i, d := range depts {
		row := i + 2
		f.SetCellValue(deptSheet, fmt.Sprintf("A%d", row), d.Department)
		f.SetCellValue(deptSheet, fmt.Sprintf("B%d", row), d.UsersCount)
		f.SetCellValue(deptSheet, fmt.Sprintf("C%d", row), d.ClickRate)
		f.SetCellValue(deptSheet, fmt.Sprintf("D%d", row), d.SubmitRate)
	}

	f.SetActiveSheet(0)

	filename := fmt.Sprintf("analytics_export_%s.xlsx", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if err := f.Write(w); err != nil {
		log.Errorf("error writing XLSX output: %v", err)
	}
}
```

- [ ] **Step 5: Wire it into `ExportAnalytics`**

Same edit point as Task 6, Step 5 — add the `case "xlsx": exportXLSX(w, r, uid)` arm. If Task 6 already landed and left `case "pdf": exportPDF(...)` in place with `"xlsx"` still falling into the not-implemented branch, split that branch's `case` line to remove `"xlsx"` from it.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./controllers/api/ -run TestExportAnalyticsXLSX -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum controllers/api/export_xlsx.go controllers/api/analytics.go controllers/api/export_xlsx_test.go
git commit -m "feat: implement XLSX analytics export

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Self-Review Notes (from initial authoring, 2026-09-05)

- **Spec coverage:** Task 5 covers roadmap item A (client half — the server half already existed pre-plan and is documented as such rather than re-built). Tasks 1-4 cover item B (report → template). Tasks 6-7 cover item C (PDF/XLSX export). All three Phase 1 items from the 2026-09-05 roadmap discussion are covered.
- **Known unknowns flagged inline rather than guessed silently:** the exact `models.Template`/`PostTemplate` signature (Task 3), the exact `setupTest`/`tearDown` test fixture names (Task 1), the exact `ControllersSuite` test plumbing (Task 2), and the exact axios client export shape (Task 4) are all called out as "confirm before writing" rather than invented — the assigned implementer (subagent or otherwise) must read the named files first.
- **Task 5 has two manual, non-scriptable steps** (hosting the static files at a real HTTPS URL, and the org rolling the manifest out via M365 admin center or side-loading) — these are called out explicitly in the add-in's own README rather than glossed over, since no amount of code changes this repo's engine makes can automate a Microsoft 365 tenant admin action.
- **Tasks 6 and 7 share one edit point** in `analytics.go`'s `ExportAnalytics` switch — flagged explicitly in both tasks so whichever lands second doesn't silently clobber the other's case arm.
