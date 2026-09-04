package middleware

import (
	"net/http"
	"os"
)

// allowedOrigins returns the list of allowed origins for CORS.
// In development (when AWARENOW_ENV=development), localhost origins are allowed.
// Otherwise, the AWARENOW_ALLOWED_ORIGINS environment variable is used (comma-separated).
func allowedOrigins() []string {
	env := os.Getenv("AWARENOW_ENV")
	if env == "development" {
		return []string{
			"http://localhost",
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
	}
	// In production, check for configured origins
	origins := os.Getenv("AWARENOW_ALLOWED_ORIGINS")
	if origins != "" {
		// Parse comma-separated list
		var result []string
		current := ""
		for _, c := range origins {
			if c == ',' {
				if current != "" {
					result = append(result, current)
					current = ""
				}
			} else {
				current += string(c)
			}
		}
		if current != "" {
			result = append(result, current)
		}
		return result
	}
	return []string{}
}

// isOriginAllowed checks if the given origin is in the allowed list
func isOriginAllowed(origin string, allowed []string) bool {
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	return false
}

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins := allowedOrigins()
		origin := r.Header.Get("Origin")

		// Only set the Allow-Origin header if the origin is in the allowed list
		if origin != "" && isOriginAllowed(origin, origins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
