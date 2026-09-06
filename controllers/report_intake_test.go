package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fir3storm/AwareNow/config"
	"github.com/fir3storm/AwareNow/models"
)

func TestReportIntakeValidation(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	valid := `{"reporter_email":"alice@example.com","subject":"Suspicious","body_text":"Inspect this"}`
	cases := []struct {
		name, body, contentType, key string
		status                       int
	}{
		{"valid", valid, "application/json", "", 204},
		{"wrong media type", valid, "text/plain", "", 415},
		{"trailing object", valid + `{}`, "application/json", "", 400},
		{"submitted owner", `{"reporter_email":"alice@example.com","body_text":"hello","owner_id":2}`, "application/json", "", 400},
		{"invalid email", `{"reporter_email":"invalid","body_text":"hello"}`, "application/json", "", 400},
		{"display name", `{"reporter_email":"Alice <alice@example.com>","body_text":"hello"}`, "application/json", "", 400},
		{"blank body", `{"reporter_email":"alice@example.com","body_text":"  "}`, "application/json", "", 400},
		{"subject newline", `{"reporter_email":"alice@example.com","subject":"hi\nthere","body_text":"hello"}`, "application/json", "", 400},
		{"large key", valid, "application/json", strings.Repeat("k", 129), 400},
		{"padded key", valid, "application/json", " padded", 400},
		{"long subject", `{"reporter_email":"alice@example.com","body_text":"hello","subject":"` + strings.Repeat("s", 999) + `"}`, "application/json", "", 400},
		{"long email", `{"reporter_email":"` + strings.Repeat("a", 243) + `@example.com","body_text":"hello"}`, "application/json", "", 400},
		{"oversize whitespace", valid + strings.Repeat(" ", maxReportBodyBytes), "application/json", "", 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh router isolates validation from the per-IP request budget.
			ps := NewPhishingServer(ctx.config.PhishConf)
			req := httptest.NewRequest(http.MethodPost, "/report-unknown", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			ps.server.Handler.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestReportIntakeCORSAndRateLimit(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	for i := 0; i < 7; i++ {
		req, _ := http.NewRequest(http.MethodOptions, ctx.phishServer.URL+"/report-unknown", nil)
		req.Header.Set("Origin", "https://reporter.example")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "content-type,idempotency-key")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 204 || resp.Header.Get("Access-Control-Allow-Origin") != "*" || !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Idempotency-Key") {
			t.Fatalf("invalid preflight: %d %v", resp.StatusCode, resp.Header)
		}
	}
	for i := 0; i < 6; i++ {
		resp, err := http.Post(ctx.phishServer.URL+"/report-unknown", "application/json", strings.NewReader(`{"reporter_email":"a@example.com","body_text":"hello"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		want := 204
		if i == 5 {
			want = 429
		}
		if resp.StatusCode != want || resp.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("request %d: status %d, headers %v", i, resp.StatusCode, resp.Header)
		}
	}
}

func TestReportIntakeIdempotency(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	cases := []struct {
		email, body string
		status      int
	}{
		{"alice@example.com", "hello", 204},
		{"alice@example.com", "hello", 204},
		{"alice@example.com", "changed", 409},
		{"bob@example.com", "hello", 204},
	}
	for _, tc := range cases {
		body, _ := json.Marshal(map[string]string{"reporter_email": tc.email, "body_text": tc.body})
		req, _ := http.NewRequest(http.MethodPost, ctx.phishServer.URL+"/report-unknown", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "retry-123")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tc.status {
			t.Fatalf("status %d, want %d", resp.StatusCode, tc.status)
		}
	}
	reports, err := models.GetReportedMessages(1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want two distinct reporters", len(reports))
	}
}

func TestReportIntakeRequiresConfiguredOwner(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	for _, owner := range []int64{0, -1, 999999} {
		ps := NewPhishingServer(config.PhishServer{ReportOwnerID: owner})
		w := httptest.NewRecorder()
		ps.server.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/report-unknown", strings.NewReader(`{}`)))
		if w.Code != 503 {
			t.Fatalf("owner %d: status %d, want 503", owner, w.Code)
		}
	}
}
