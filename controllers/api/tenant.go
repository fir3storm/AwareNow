package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	log "github.com/fir3storm/AwareNow/logger"
	"github.com/fir3storm/AwareNow/models"
	"github.com/gorilla/mux"
)

// Tenant-related errors
var (
	// ErrEmptyTenantName is thrown when a tenant name is not provided
	ErrEmptyTenantName = errors.New("tenant name is required")
	// ErrEmptyTenantDomain is thrown when a tenant domain is not provided
	ErrEmptyTenantDomain = errors.New("tenant domain is required")
)

// tenantRequest represents the payload for creating a new tenant
type tenantRequest struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// tenantUpdateRequest represents the payload for updating a tenant
type tenantUpdateRequest struct {
	Name     string `json:"name,omitempty"`
	Domain   string `json:"domain,omitempty"`
	IsActive *bool  `json:"is_active,omitempty"`
}

// validate ensures the tenant request has all required fields
func (tr *tenantRequest) Validate() error {
	tr.Name = strings.TrimSpace(tr.Name)
	tr.Domain = strings.TrimSpace(tr.Domain)

	if tr.Name == "" {
		return ErrEmptyTenantName
	}
	if tr.Domain == "" {
		return ErrEmptyTenantDomain
	}
	return nil
}

// Tenants handles listing all tenants (GET) and creating new tenants (POST).
// These operations require super admin privileges.
func (as *Server) Tenants(w http.ResponseWriter, r *http.Request) {
	tm := models.GetTenantManager()

	switch {
	case r.Method == "GET":
		tenants, err := tm.ListTenants()
		if err != nil {
			log.Errorf("Error listing tenants: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, tenants, http.StatusOK)

	case r.Method == "POST":
		tr := &tenantRequest{}
		if err := json.NewDecoder(r.Body).Decode(tr); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}

		if err := tr.Validate(); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}

		tenant, err := tm.CreateTenant(tr.Name, tr.Domain)
		if err != nil {
			if err == models.ErrTenantExists {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusConflict)
				return
			}
			log.Errorf("Error creating tenant: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		log.Infof("Created new tenant: %s (ID: %d, Domain: %s)", tenant.Name, tenant.ID, tenant.Domain)
		JSONResponse(w, tenant, http.StatusCreated)
	}
}

// Tenant handles operations on a single tenant: GET, PUT, DELETE.
func (as *Server) Tenant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid tenant ID"}, http.StatusBadRequest)
		return
	}

	tm := models.GetTenantManager()

	switch {
	case r.Method == "GET":
		tenant, err := tm.GetTenant(uint(id))
		if err != nil {
			if err == models.ErrTenantNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Tenant not found"}, http.StatusNotFound)
				return
			}
			log.Errorf("Error getting tenant: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, tenant, http.StatusOK)

	case r.Method == "PUT":
		tur := &tenantUpdateRequest{}
		if err := json.NewDecoder(r.Body).Decode(tur); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}

		updates := make(map[string]interface{})
		if tur.Name != "" {
			updates["name"] = strings.TrimSpace(tur.Name)
		}
		if tur.Domain != "" {
			updates["domain"] = strings.TrimSpace(tur.Domain)
		}
		if tur.IsActive != nil {
			updates["is_active"] = *tur.IsActive
		}

		tenant, err := tm.UpdateTenant(uint(id), updates)
		if err != nil {
			if err == models.ErrTenantNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Tenant not found"}, http.StatusNotFound)
				return
			}
			if err == models.ErrTenantExists {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusConflict)
				return
			}
			log.Errorf("Error updating tenant: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		log.Infof("Updated tenant: %s (ID: %d)", tenant.Name, tenant.ID)
		JSONResponse(w, tenant, http.StatusOK)

	case r.Method == "DELETE":
		err := tm.DeleteTenant(uint(id))
		if err != nil {
			if err == models.ErrTenantNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Tenant not found"}, http.StatusNotFound)
				return
			}
			log.Errorf("Error deleting tenant: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		log.Infof("Deleted tenant with ID: %d", id)
		JSONResponse(w, models.Response{Success: true, Message: "Tenant deleted successfully"}, http.StatusOK)
	}
}

// TenantStats returns statistics for a specific tenant.
func (as *Server) TenantStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid tenant ID"}, http.StatusBadRequest)
		return
	}

	tm := models.GetTenantManager()

	switch {
	case r.Method == "GET":
		stats, err := tm.GetTenantStats(uint(id))
		if err != nil {
			if err == models.ErrTenantNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Tenant not found"}, http.StatusNotFound)
				return
			}
			if err == models.ErrTenantInactive {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusForbidden)
				return
			}
			log.Errorf("Error getting tenant stats: %v", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, stats, http.StatusOK)
	}
}
