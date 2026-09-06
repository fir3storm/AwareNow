package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"strings"

	"github.com/fir3storm/AwareNow/models"
)

const maxReportBodyBytes = 1 << 20

// Reporting clients are public clients: CORS grants no identity or ownership.
// Wrap the limiter so browsers can read its error responses as well.
func reportIntakeCORS(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// ReportUnknownHandler stores unverified reported mail in the configured owner's
// inbox. A client-supplied identity is never used to authorize access.
func (ps *PhishingServer) ReportUnknownHandler(w http.ResponseWriter, r *http.Request) {
	if ps.config.ReportOwnerID <= 0 {
		http.Error(w, "report intake is not configured", http.StatusServiceUnavailable)
		return
	}
	if _, err := models.GetUser(ps.config.ReportOwnerID); err != nil {
		http.Error(w, "report intake is unavailable", http.StatusServiceUnavailable)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	// Read through MaxBytesReader before decoding, bounding even whitespace and
	// trailing payloads. No email content is written to logs on failures.
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxReportBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "report exceeds 1 MiB", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "invalid request", http.StatusBadRequest)
		}
		return
	}
	var payload struct {
		ReporterEmail string `json:"reporter_email"`
		Subject       string `json:"subject"`
		BodyText      string `json:"body_text"`
		BodyHTML      string `json:"body_html"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || decoder.Decode(new(interface{})) != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	payload.ReporterEmail = strings.TrimSpace(payload.ReporterEmail)
	address, err := mail.ParseAddress(payload.ReporterEmail)
	if err != nil || address.Address != payload.ReporterEmail || len(payload.ReporterEmail) > 254 ||
		len(payload.Subject) > 998 || strings.ContainsAny(payload.Subject, "\r\n") ||
		(strings.TrimSpace(payload.BodyText) == "" && strings.TrimSpace(payload.BodyHTML) == "") {
		http.Error(w, "invalid report fields", http.StatusBadRequest)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) > 128 || (key != "" && strings.TrimSpace(key) != key) {
		http.Error(w, "invalid Idempotency-Key", http.StatusBadRequest)
		return
	}
	rm := &models.ReportedMessage{
		OwnerID: ps.config.ReportOwnerID, ReporterEmail: payload.ReporterEmail,
		Subject: payload.Subject, BodyText: payload.BodyText, BodyHTML: payload.BodyHTML,
	}
	if key != "" {
		// Length-delimited components prevent ambiguous concatenation. Preserve
		// local-part casing: distinct reporters must not be merged implicitly.
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s:%s", rm.OwnerID, len(rm.ReporterEmail), rm.ReporterEmail, key)))
		hash := hex.EncodeToString(sum[:])
		rm.IdempotencyKeyHash = &hash
	}
	if err := models.CreateReportedMessage(rm); err != nil {
		if errors.Is(err, models.ErrReportedMessageIdempotencyConflict) {
			http.Error(w, "idempotency key already used for a different report", http.StatusConflict)
		} else {
			http.Error(w, "unable to store report", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
