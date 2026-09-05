package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	mid "github.com/fir3storm/AwareNow/middleware"
	"github.com/fir3storm/AwareNow/middleware/ratelimit"
	"github.com/fir3storm/AwareNow/models"
	"github.com/fir3storm/AwareNow/worker"
	"github.com/gorilla/mux"
)

// ServerOption is an option to apply to the API server.
type ServerOption func(*Server)

// Server represents the routes and functionality of the Gophish API.
// It's not a server in the traditional sense, in that it isn't started and
// stopped. Rather, it's meant to be used as an http.Handler in the
// AdminServer.
type Server struct {
	handler http.Handler
	worker  worker.Worker
	limiter *ratelimit.PostLimiter
}

// NewServer returns a new instance of the API handler with the provided
// options applied.
func NewServer(options ...ServerOption) *Server {
	defaultWorker, _ := worker.New()
	defaultLimiter := ratelimit.NewPostLimiter()
	as := &Server{
		worker:  defaultWorker,
		limiter: defaultLimiter,
	}
	for _, opt := range options {
		opt(as)
	}
	as.registerRoutes()
	return as
}

// WithWorker is an option that sets the background worker.
func WithWorker(w worker.Worker) ServerOption {
	return func(as *Server) {
		as.worker = w
	}
}

func WithLimiter(limiter *ratelimit.PostLimiter) ServerOption {
	return func(as *Server) {
		as.limiter = limiter
	}
}

func (as *Server) registerRoutes() {
	root := mux.NewRouter()
	root = root.StrictSlash(true)
	router := root.PathPrefix("/api/").Subrouter()

	// JWT login endpoint (no API key required)
	router.HandleFunc("/login", as.login).Methods("POST")

	// Protected routes require API key
	router.Use(mid.RequireAPIKey)
	router.Use(mid.EnforceViewOnly)
	router.HandleFunc("/me", as.profile).Methods("GET")
	router.HandleFunc("/imap/", as.IMAPServer)
	router.HandleFunc("/imap/validate", as.IMAPServerValidate)
	router.HandleFunc("/reset", as.Reset)
	router.HandleFunc("/campaigns/", as.Campaigns)
	router.HandleFunc("/campaigns/summary", as.CampaignsSummary)
	router.HandleFunc("/campaigns/{id:[0-9]+}", as.Campaign)
	router.HandleFunc("/campaigns/{id:[0-9]+}/results", as.CampaignResults)
	router.HandleFunc("/campaigns/{id:[0-9]+}/summary", as.CampaignSummary)
	router.HandleFunc("/campaigns/{id:[0-9]+}/complete", as.CampaignComplete)
	router.HandleFunc("/groups/", as.Groups)
	router.HandleFunc("/groups/summary", as.GroupsSummary)
	router.HandleFunc("/groups/{id:[0-9]+}", as.Group)
	router.HandleFunc("/groups/{id:[0-9]+}/summary", as.GroupSummary)
	router.HandleFunc("/templates/", as.Templates)
	router.HandleFunc("/templates/{id:[0-9]+}", as.Template)
	router.HandleFunc("/pages/", as.Pages)
	router.HandleFunc("/pages/{id:[0-9]+}", as.Page)
	router.HandleFunc("/smtp/", as.SendingProfiles)
	router.HandleFunc("/smtp/{id:[0-9]+}", as.SendingProfile)
	router.HandleFunc("/smtp/{id:[0-9]+}/usage", as.SMTPUsage)
	router.HandleFunc("/users/", mid.Use(as.Users, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/users/{id:[0-9]+}", mid.Use(as.User))
	router.HandleFunc("/util/send_test_email", as.SendTestEmail)
	router.HandleFunc("/import/group", as.ImportGroup)
	router.HandleFunc("/import/email", as.ImportEmail)
	router.HandleFunc("/import/site", as.ImportSite)
	router.HandleFunc("/webhooks/", mid.Use(as.Webhooks, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/webhooks/{id:[0-9]+}/validate", mid.Use(as.ValidateWebhook, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/webhooks/{id:[0-9]+}", mid.Use(as.Webhook, mid.RequirePermission(models.PermissionModifySystem)))
	// Analytics routes
	router.HandleFunc("/analytics/overview", as.AnalyticsOverview).Methods("GET")
	router.HandleFunc("/analytics/campaigns/{id:[0-9]+}/timeline", as.CampaignTimeline).Methods("GET")
	router.HandleFunc("/analytics/timeline", as.OverallTimeline).Methods("GET")
	router.HandleFunc("/analytics/departments", as.DepartmentStats).Methods("GET")
	router.HandleFunc("/analytics/trends", as.Trends).Methods("GET")
	router.HandleFunc("/analytics/risk-score", as.RiskScore).Methods("GET")
	router.HandleFunc("/analytics/export", as.ExportAnalytics).Methods("GET")
	// Tenant management routes (super admin only)
	router.HandleFunc("/admin/tenants/", mid.Use(as.Tenants, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/admin/tenants/{id:[0-9]+}", mid.Use(as.Tenant, mid.RequirePermission(models.PermissionModifySystem)))
	router.HandleFunc("/admin/tenants/{id:[0-9]+}/stats", mid.Use(as.TenantStats, mid.RequirePermission(models.PermissionModifySystem)))
	// Behavior events routes
	router.HandleFunc("/behavior-events", as.BehaviorEvents).Methods("POST", "GET")
	// Reported message admin review routes
	router.HandleFunc("/reported-messages/", mid.Use(as.ReportedMessages, mid.RequirePermission(models.PermissionModifyObjects)))
	router.HandleFunc("/reported-messages/{id:[0-9]+}", mid.Use(as.ReportedMessage, mid.RequirePermission(models.PermissionModifyObjects)))
	router.HandleFunc("/reported-messages/{id:[0-9]+}/approve", mid.Use(as.ReportedMessageApprove, mid.RequirePermission(models.PermissionModifyObjects))).Methods("POST")
	router.HandleFunc("/reported-messages/{id:[0-9]+}/reject", mid.Use(as.ReportedMessageReject, mid.RequirePermission(models.PermissionModifyObjects))).Methods("POST")

	// Add a default handler for unsupported HTTP methods on all routes
	router.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		response := map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Method %s not allowed", r.Method),
		}
		json.NewEncoder(w).Encode(response)
	})

	as.handler = router
}

func (as *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	as.handler.ServeHTTP(w, r)
}
