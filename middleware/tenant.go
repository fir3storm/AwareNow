package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	ctx "github.com/fir3storm/AwareNow/context"
	"github.com/fir3storm/AwareNow/models"
)

// Context keys for storing tenant information in the request context
type tenantContextKey string

const (
	// TenantIDKey is the context key for the tenant ID
	TenantIDKey tenantContextKey = "tenant_id"
	// TenantKey is the context key for the full tenant object
	TenantKey tenantContextKey = "tenant"
)

// JSONError returns an error in JSON format with the given status code and message
func tenantJSONError(w http.ResponseWriter, c int, m string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(c)
	response := map[string]interface{}{
		"success": false,
		"message": m,
	}
	json.NewEncoder(w).Encode(response)
}

// TenantResolver is a middleware that resolves the current tenant from the request.
// It supports three resolution methods (in order of priority):
// 1. X-Tenant-ID header
// 2. JWT claim (if Authorization header is present)
// 3. Subdomain extraction from the Host header (tenant.awarenow.com)
//
// Once resolved, the tenant ID and tenant object are stored in the request context.
func TenantResolver(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tenant *models.Tenant
		var err error

		// Get the tenant manager
		tm := models.GetTenantManager()

		// Method 1: Check X-Tenant-ID header
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID != "" {
			var id uint
			if _, err := fmt.Sscanf(tenantID, "%d", &id); err == nil {
				tenant, err = tm.GetTenant(id)
				if err != nil {
					tenantJSONError(w, http.StatusUnauthorized, "Invalid tenant ID")
					return
				}
			}
		}

		// Method 2: Check JWT claim (if Authorization header is present)
		if tenant == nil {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				tenantID, err := extractTenantFromJWT(authHeader)
				if err == nil && tenantID > 0 {
					tenant, err = tm.GetTenant(tenantID)
					if err != nil {
						tenantJSONError(w, http.StatusUnauthorized, "Invalid tenant in token")
						return
					}
				}
			}
		}

		// Method 3: Extract from subdomain
		if tenant == nil {
			host := r.Host
			// Remove port if present
			if idx := strings.Index(host, ":"); idx != -1 {
				host = host[:idx]
			}

			// Extract subdomain (e.g., "tenant" from "tenant.awarenow.com")
			subdomain := extractSubdomain(host)
			if subdomain != "" && subdomain != "www" {
				tenant, err = tm.GetTenantByDomain(subdomain)
				if err != nil {
					tenantJSONError(w, http.StatusUnauthorized, "Tenant not found for subdomain")
					return
				}
			}
		}

		// If no tenant was resolved, continue without one
		// (some routes may not require a tenant)
		if tenant != nil {
			// Check if tenant is active
			if !tenant.IsActive {
				tenantJSONError(w, http.StatusForbidden, "Tenant account is inactive")
				return
			}

			// Store tenant information in request context
			r = ctx.Set(r, TenantIDKey, tenant.ID)
			r = ctx.Set(r, TenantKey, tenant)
		}

		next.ServeHTTP(w, r)
	})
}

// RequireTenant is a middleware that requires a valid tenant to be resolved.
// If no tenant is found, it returns an error.
func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, ok := ctx.Get(r, TenantIDKey).(uint)
		if !ok || tenantID == 0 {
			tenantJSONError(w, http.StatusUnauthorized, "Tenant context required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetTenantID retrieves the tenant ID from the request context.
// Returns 0 if no tenant is set.
func GetTenantID(r *http.Request) uint {
	tenantID, ok := ctx.Get(r, TenantIDKey).(uint)
	if !ok {
		return 0
	}
	return tenantID
}

// GetTenant retrieves the full tenant object from the request context.
// Returns nil if no tenant is set.
func GetTenant(r *http.Request) *models.Tenant {
	tenant, ok := ctx.Get(r, TenantKey).(*models.Tenant)
	if !ok {
		return nil
	}
	return tenant
}

// GetTenantDB retrieves the database connection for the current tenant.
// Returns an error if no tenant is resolved or the database cannot be accessed.
func GetTenantDB(r *http.Request) (interface{}, error) {
	tenantID := GetTenantID(r)
	if tenantID == 0 {
		return nil, fmt.Errorf("no tenant context available")
	}

	tm := models.GetTenantManager()
	return tm.GetConnection(tenantID)
}

// extractSubdomain extracts the subdomain from a hostname.
// For example, "tenant.awarenow.com" returns "tenant".
func extractSubdomain(host string) string {
	// Handle localhost and IP addresses
	if strings.Contains(host, "localhost") || strings.Contains(host, ":") {
		return ""
	}

	parts := strings.Split(host, ".")
	// If we have at least 3 parts (subdomain.domain.tld), return the first part
	if len(parts) >= 3 {
		return parts[0]
	}

	return ""
}

// extractTenantFromJWT extracts the tenant ID from the JWT token.
// This is a placeholder that would need to be implemented based on
// the actual JWT structure used in the application.
func extractTenantFromJWT(authHeader string) (uint, error) {
	// Remove "Bearer " prefix
	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimSpace(token)

	if token == "" {
		return 0, fmt.Errorf("empty token")
	}

	// In a real implementation, you would:
	// 1. Parse the JWT token
	// 2. Extract the "tenant_id" claim
	// 3. Return the tenant ID
	//
	// For now, return an error to indicate this method is not available
	return 0, fmt.Errorf("JWT tenant extraction not implemented")
}

// SetTenantContext is a helper function to manually set the tenant context.
// This is useful for testing or when the tenant is determined through other means.
func SetTenantContext(r *http.Request, tenant *models.Tenant) *http.Request {
	r = ctx.Set(r, TenantIDKey, tenant.ID)
	r = ctx.Set(r, TenantKey, tenant)
	return r
}

// WithTenantContext returns a new context with the tenant information set.
// This can be used outside of HTTP handlers.
func WithTenantContext(parent context.Context, tenantID uint) context.Context {
	return context.WithValue(parent, TenantIDKey, tenantID)
}

// TenantIDFromContext retrieves the tenant ID from a context.
// Returns 0 if no tenant is set.
func TenantIDFromContext(ctx context.Context) uint {
	tenantID, ok := ctx.Value(TenantIDKey).(uint)
	if !ok {
		return 0
	}
	return tenantID
}
