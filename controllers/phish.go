package controllers

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/fir3storm/AwareNow/config"
	ctx "github.com/fir3storm/AwareNow/context"
	"github.com/fir3storm/AwareNow/controllers/api"
	log "github.com/fir3storm/AwareNow/logger"
	mid "github.com/fir3storm/AwareNow/middleware"
	"github.com/fir3storm/AwareNow/middleware/ratelimit"
	"github.com/fir3storm/AwareNow/models"
	"github.com/fir3storm/AwareNow/util"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jordan-wright/unindexed"
)

// ErrInvalidRequest is thrown when a request with an invalid structure is
// received
var ErrInvalidRequest = errors.New("Invalid request")

// ErrCampaignComplete is thrown when an event is received for a campaign that
// has already been marked as complete.
var ErrCampaignComplete = errors.New("Event received on completed campaign")

// TransparencyResponse is the JSON response provided when a third-party
// makes a request to the transparency handler.
type TransparencyResponse struct {
	Server         string    `json:"server"`
	ContactAddress string    `json:"contact_address"`
	SendDate       time.Time `json:"send_date"`
}

// TransparencySuffix (when appended to a valid result ID), will cause Gophish
// to return a transparency response.
const TransparencySuffix = "+"

// PhishingServerOption is a functional option that is used to configure the
// the phishing server
type PhishingServerOption func(*PhishingServer)

// PhishingServer is an HTTP server that implements the campaign event
// handlers, such as email open tracking, click tracking, and more.
type PhishingServer struct {
	server         *http.Server
	config         config.PhishServer
	contactAddress string
	limiter        *ratelimit.PostLimiter
}

// NewPhishingServer returns a new instance of the phishing server with
// provided options applied.
func NewPhishingServer(config config.PhishServer, options ...PhishingServerOption) *PhishingServer {
	defaultServer := &http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         config.ListenURL,
	}
	ps := &PhishingServer{
		server:  defaultServer,
		config:  config,
		limiter: ratelimit.NewPostLimiter(),
	}
	for _, opt := range options {
		opt(ps)
	}
	ps.registerRoutes()
	return ps
}

// WithContactAddress sets the contact address used by the transparency
// handlers
func WithContactAddress(addr string) PhishingServerOption {
	return func(ps *PhishingServer) {
		ps.contactAddress = addr
	}
}

// Start launches the phishing server, listening on the configured address.
func (ps *PhishingServer) Start() {
	if ps.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		ps.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(ps.config.CertPath, ps.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting phishing server at https://%s", ps.config.ListenURL)
		log.Fatal(ps.server.ListenAndServeTLS(ps.config.CertPath, ps.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting phishing server at http://%s", ps.config.ListenURL)
	log.Fatal(ps.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (ps *PhishingServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return ps.server.Shutdown(ctx)
}

// CreatePhishingRouter creates the router that handles phishing connections.
func (ps *PhishingServer) registerRoutes() {
	router := mux.NewRouter()
	fileServer := http.FileServer(unindexed.Dir("./static/endpoint/"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileServer))
	router.HandleFunc("/track", ps.TrackHandler)
	router.HandleFunc("/track-details", ps.TrackDetailsHandler).Methods("POST")
	router.HandleFunc("/robots.txt", ps.RobotsHandler)
	router.HandleFunc("/{path:.*}/track", ps.TrackHandler)
	router.HandleFunc("/{path:.*}/report", ps.ReportHandler)
	router.HandleFunc("/report", ps.ReportHandler)
	router.HandleFunc("/report-unknown", mid.Use(ps.ReportUnknownHandler, ps.limiter.Limit, reportIntakeCORS)).Methods("POST", "OPTIONS")
	// Behavior events endpoint on phishing server
	router.HandleFunc("/api/behavior-events", ps.BehaviorEventsHandler).Methods("POST")
	router.HandleFunc("/{path:.*}/api/behavior-events", ps.BehaviorEventsHandler).Methods("POST")
	router.HandleFunc("/{path:.*}", ps.PhishHandler)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	phishHandler := gzipWrapper(router)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	phishHandler = handlers.ProxyHeaders(phishHandler)

	// Setup logging
	phishHandler = handlers.CombinedLoggingHandler(log.Writer(), phishHandler)
	ps.server.Handler = phishHandler
}

// TrackHandler tracks emails as they are opened, updating the status for the given Result.
// Enhanced to collect additional metadata including email_client, device_type, referrer, and tls_version.
func (ps *PhishingServer) TrackHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		http.ServeFile(w, r, "static/images/pixel.png")
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	// Enhance tracking metadata with email client, device type, referrer, and TLS info
	userAgent := r.Header.Get("User-Agent")
	referrer := r.Header.Get("Referer")
	emailClient := util.GetEmailClient(userAgent)
	deviceType := util.GetDeviceType(userAgent)

	// Extract TLS version if available
	tlsVersion := ""
	if r.TLS != nil {
		tlsVersion = getTLSVersionName(r.TLS.Version)
	}

	// Update browser details with enhanced metadata
	d.Browser["email_client"] = emailClient
	d.Browser["device_type"] = deviceType
	d.Browser["referrer"] = referrer
	d.Browser["tls_version"] = tlsVersion

	err = rs.HandleEmailOpened(d)
	if err != nil {
		log.Error(err)
	}

	// Update total_opens counter and last_activity
	rs.TotalOpens++
	rs.LastActivity = time.Now().UTC()
	err = models.SaveResult(&rs)
	if err != nil {
		log.Errorf("error updating result opens counter: %v", err)
	}

	http.ServeFile(w, r, "static/images/pixel.png")
}

// TrackDetailsHandler receives device fingerprint data from the client-side
// tracking script and updates the result with enhanced metadata.
// POST /track-details
func (ps *PhishingServer) TrackDetailsHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}

	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	// Parse fingerprint data from request body
	var fingerprint struct {
		ScreenWidth   int    `json:"screen_width"`
		ScreenHeight  int    `json:"screen_height"`
		ColorDepth    int    `json:"color_depth"`
		Timezone      string `json:"timezone"`
		Language      string `json:"language"`
		Platform      string `json:"platform"`
		Concurrency   int    `json:"hardware_concurrency"`
		DeviceMemory  int    `json:"device_memory"`
		TouchSupport  bool   `json:"touch_support"`
		WebGLVendor   string `json:"webgl_vendor"`
		WebGLRenderer string `json:"webgl_renderer"`
		CanvasFP      string `json:"canvas_fingerprint"`
		Fonts         string `json:"fonts"`
		Plugins       string `json:"plugins"`
		AudioFP       string `json:"audio_fingerprint"`
	}

	if r.Method == "POST" && r.Body != nil {
		err := json.NewDecoder(r.Body).Decode(&fingerprint)
		if err == nil {
			// Store fingerprint data in browser details
			d.Browser["screen"] = fmt.Sprintf("%dx%d", fingerprint.ScreenWidth, fingerprint.ScreenHeight)
			d.Browser["color_depth"] = fmt.Sprintf("%d", fingerprint.ColorDepth)
			d.Browser["timezone"] = fingerprint.Timezone
			d.Browser["language"] = fingerprint.Language
			d.Browser["platform"] = fingerprint.Platform
			d.Browser["hardware_concurrency"] = fmt.Sprintf("%d", fingerprint.Concurrency)
			d.Browser["device_memory"] = fmt.Sprintf("%d", fingerprint.DeviceMemory)
			d.Browser["touch_support"] = fmt.Sprintf("%v", fingerprint.TouchSupport)
			d.Browser["webgl_vendor"] = fingerprint.WebGLVendor
			d.Browser["webgl_renderer"] = fingerprint.WebGLRenderer
			d.Browser["canvas_fingerprint"] = fingerprint.CanvasFP
			d.Browser["fonts"] = fingerprint.Fonts
			d.Browser["plugins"] = fingerprint.Plugins
			d.Browser["audio_fingerprint"] = fingerprint.AudioFP
		}
	}

	// Update the result with fingerprint data
	err = rs.HandleBehaviorBatch(d)
	if err != nil {
		log.Error(err)
	}

	// Update last_activity timestamp
	rs.LastActivity = time.Now().UTC()
	models.SaveResult(&rs)

	w.WriteHeader(http.StatusNoContent)
}

// BehaviorEventsHandler wraps the behavior API handler to receive
// client-side behavior event batches on the phishing server.
// POST /api/behavior-events (via phishing server)
func (ps *PhishingServer) BehaviorEventsHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}

	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		api.JSONResponse(w, models.Response{Success: false, Message: "Preview mode - no events recorded"}, http.StatusBadRequest)
		return
	}

	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	// Parse the behavior event batch
	var batch struct {
		SessionID   string                   `json:"session_id"`
		Events      []map[string]interface{} `json:"events"`
		TimeOnPage  int64                    `json:"time_on_page"`
		EmailClient string                   `json:"email_client"`
		DeviceType  string                   `json:"device_type"`
		Referrer    string                   `json:"referrer"`
		TLSVersion  string                   `json:"tls_version"`
	}

	if r.Method == "POST" && r.Body != nil {
		err := json.NewDecoder(r.Body).Decode(&batch)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return
		}
	}

	// Enhance batch metadata from the request if not provided
	userAgent := r.Header.Get("User-Agent")
	if batch.EmailClient == "" {
		batch.EmailClient = util.GetEmailClient(userAgent)
	}
	if batch.DeviceType == "" {
		batch.DeviceType = util.GetDeviceType(userAgent)
	}
	if batch.Referrer == "" {
		batch.Referrer = r.Header.Get("Referer")
	}
	if batch.TLSVersion == "" && r.TLS != nil {
		batch.TLSVersion = getTLSVersionName(r.TLS.Version)
	}

	// Create behavior events for each event in the batch
	for _, evt := range batch.Events {
		eventType, _ := evt["type"].(string)
		eventTime := time.Now().UTC()
		if ts, ok := evt["timestamp"].(float64); ok {
			eventTime = time.Unix(0, int64(ts)*int64(time.Millisecond)).UTC()
		}

		details := models.EventDetails{
			Payload: map[string][]string{},
			Browser: map[string]string{
				"rid":          rid,
				"session_id":   batch.SessionID,
				"event_type":   eventType,
				"email_client": batch.EmailClient,
				"device_type":  batch.DeviceType,
				"referrer":     batch.Referrer,
				"tls_version":  batch.TLSVersion,
			},
		}

		if data, ok := evt["data"].(map[string]interface{}); ok {
			dataJSON, _ := json.Marshal(data)
			details.Browser["event_data"] = string(dataJSON)
		}

		be := &models.BehaviorEvent{
			Rid:         rid,
			CampaignId:  rs.CampaignId,
			SessionId:   batch.SessionID,
			EventType:   eventType,
			EventTime:   eventTime,
			TimeOnPage:  batch.TimeOnPage,
			EmailClient: batch.EmailClient,
			DeviceType:  batch.DeviceType,
			Referrer:    batch.Referrer,
			TLSCipher:   batch.TLSVersion,
			Details:     details,
		}

		err = models.AddBehaviorEvent(be)
		if err != nil {
			log.Errorf("error saving behavior event: %v", err)
		}
	}

	// Update result tracking metadata
	rs.LastActivity = time.Now().UTC()
	models.SaveResult(&rs)

	api.JSONResponse(w, models.Response{
		Success: true,
		Message: "Behavior events recorded",
		Data: map[string]interface{}{
			"events_processed": len(batch.Events),
		},
	}, http.StatusCreated)
}

// getTLSVersionName returns a human-readable TLS version string from the
// TLS version constant.
func getTLSVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

// ReportHandler tracks emails as they are reported, updating the status for the given Result
func (ps *PhishingServer) ReportHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	w.Header().Set("Access-Control-Allow-Origin", "*") // To allow Chrome extensions (or other pages) to report a campaign without violating CORS
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	err = rs.HandleEmailReport(d)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// PhishHandler handles incoming client connections and registers the associated actions performed
// (such as clicked link, etc.)
func (ps *PhishingServer) PhishHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Server", config.ServerName) // Useful for checking if this is a GoPhish server (e.g. for campaign reporting plugins)
	var ptx models.PhishingTemplateContext
	// Check for a preview
	if preview, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		ptx, err = models.NewPhishingTemplateContext(&preview, preview.BaseRecipient, preview.RId)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return
		}
		p, err := models.GetPage(preview.PageId, preview.UserId)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return
		}
		renderPhishResponse(w, r, ptx, p)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	c := ctx.Get(r, "campaign").(models.Campaign)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	p, err := models.GetPage(c.PageId, c.UserId)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
		return
	}
	switch {
	case r.Method == "GET":
		err = rs.HandleClickedLink(d)
		if err != nil {
			log.Error(err)
		}
		// Calculate time_to_click on first click
		if rs.TimeToClickMs == 0 && !rs.SendDate.IsZero() {
			rs.TimeToClickMs = time.Now().UTC().Sub(rs.SendDate).Milliseconds()
		}
		rs.LastActivity = time.Now().UTC()
		rs.TotalClicks++
		models.SaveResult(&rs)
	case r.Method == "POST":
		err = rs.HandleFormSubmit(d)
		if err != nil {
			log.Error(err)
		}
		rs.LastActivity = time.Now().UTC()
		models.SaveResult(&rs)
	}
	ptx, err = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
		return
	}
	renderPhishResponse(w, r, ptx, p)
}

// renderPhishResponse handles rendering the correct response to the phishing
// connection. This usually involves writing out the page HTML or redirecting
// the user to the correct URL.
func renderPhishResponse(w http.ResponseWriter, r *http.Request, ptx models.PhishingTemplateContext, p models.Page) {
	// If the request was a form submit and a redirect URL was specified, we
	// should send the user to that URL
	if r.Method == "POST" {
		if p.RedirectURL != "" {
			redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
			if err != nil {
				log.Error(err)
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
	}
	// Otherwise, we just need to write out the templated HTML
	html, err := models.ExecuteTemplate(p.HTML, ptx)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
		return
	}

	// Inject the tracking script with the recipient ID before </head>
	html, err = models.InjectTrackingScriptToHTML(html, ptx.RId)
	if err != nil {
		log.Errorf("error injecting tracking script: %v", err)
		// Continue without tracking script - don't fail the request
	}

	w.Write([]byte(html))
}

// RobotsHandler prevents search engines, etc. from indexing phishing materials
func (ps *PhishingServer) RobotsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "User-agent: *\nDisallow: /")
}

// TransparencyHandler returns a TransparencyResponse for the provided result
// and campaign.
func (ps *PhishingServer) TransparencyHandler(w http.ResponseWriter, r *http.Request) {
	rs := ctx.Get(r, "result").(models.Result)
	tr := &TransparencyResponse{
		Server:         config.ServerName,
		SendDate:       rs.SendDate,
		ContactAddress: ps.contactAddress,
	}
	api.JSONResponse(w, tr, http.StatusOK)
}

// setupContext handles some of the administrative work around receiving a new
// request, such as checking the result ID, the campaign, etc.
func setupContext(r *http.Request) (*http.Request, error) {
	err := r.ParseForm()
	if err != nil {
		log.Error(err)
		return r, err
	}
	rid := r.Form.Get(models.RecipientParameter)
	if rid == "" {
		return r, ErrInvalidRequest
	}
	// Since we want to support the common case of adding a "+" to indicate a
	// transparency request, we need to take care to handle the case where the
	// request ends with a space, since a "+" is technically reserved for use
	// as a URL encoding of a space.
	if strings.HasSuffix(rid, " ") {
		// We'll trim off the space
		rid = strings.TrimRight(rid, " ")
		// Then we'll add the transparency suffix
		rid = fmt.Sprintf("%s%s", rid, TransparencySuffix)
	}
	// Finally, if this is a transparency request, we'll need to verify that
	// a valid rid has been provided, so we'll look up the result with a
	// trimmed parameter.
	id := strings.TrimSuffix(rid, TransparencySuffix)
	// Check to see if this is a preview or a real result
	if strings.HasPrefix(id, models.PreviewPrefix) {
		rs, err := models.GetEmailRequestByResultId(id)
		if err != nil {
			return r, err
		}
		r = ctx.Set(r, "result", rs)
		return r, nil
	}
	rs, err := models.GetResult(id)
	if err != nil {
		return r, err
	}
	c, err := models.GetCampaign(rs.CampaignId, rs.UserId)
	if err != nil {
		log.Error(err)
		return r, err
	}
	// Don't process events for completed campaigns
	if c.Status == models.CampaignComplete {
		return r, ErrCampaignComplete
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// Handle post processing such as GeoIP
	err = rs.UpdateGeo(ip)
	if err != nil {
		log.Error(err)
	}
	d := models.EventDetails{
		Payload: r.Form,
		Browser: make(map[string]string),
	}
	d.Browser["address"] = ip
	d.Browser["user-agent"] = r.Header.Get("User-Agent")

	// Enhance tracking with email client, device type, referrer, and TLS info
	userAgent := r.Header.Get("User-Agent")
	d.Browser["email_client"] = util.GetEmailClient(userAgent)
	d.Browser["device_type"] = util.GetDeviceType(userAgent)
	d.Browser["referrer"] = r.Header.Get("Referer")
	if r.TLS != nil {
		d.Browser["tls_version"] = getTLSVersionName(r.TLS.Version)
	}

	r = ctx.Set(r, "rid", rid)
	r = ctx.Set(r, "result", rs)
	r = ctx.Set(r, "campaign", c)
	r = ctx.Set(r, "details", d)
	return r, nil
}
