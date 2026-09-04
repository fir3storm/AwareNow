package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/fir3storm/AwareNow/auth"
	ctx "github.com/fir3storm/AwareNow/context"
)

func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"success":false,"message":"Authorization header required"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, `{"success":false,"message":"Invalid authorization format"}`, http.StatusUnauthorized)
			return
		}

		claims, err := auth.ValidateJWT(parts[1])
		if err != nil {
			http.Error(w, `{"success":false,"message":"Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		context := r.Context()
		context = ctx.Set(context, "user_id", claims.UserID)
		context = ctx.Set(context, "username", claims.Username)
		context = ctx.Set(context, "role", claims.Role)

		next.ServeHTTP(w, r.WithContext(context))
	})
}
