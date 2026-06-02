package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
)

// AllowedOriginsFromEnv parses CORS_ALLOWED_ORIGINS into a list of origins.
// Empty input returns empty list (reject all); dev sets "*".
func AllowedOriginsFromEnv(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// NewCORS returns a chi MiddlewareFunc that handles CORS preflight + headers
// per D7-16. cors.Handler short-circuits OPTIONS preflight automatically
// (returns 200 OK + headers, does NOT call next.ServeHTTP) — verified in
// github.com/go-chi/cors source. This is why ordering in the chain is safe
// BEFORE OrgContext.
func NewCORS(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		// go-chi/cors treats an empty AllowedOrigins list as wildcard.
		allowedOrigins = []string{"https://cors-deny.open-routing.invalid"}
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"X-Org-Id", "Content-Type", "Idempotency-Key", "Accept-Language"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	})
}
