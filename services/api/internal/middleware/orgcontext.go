package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// OrgContext is the trust boundary middleware that parses the X-Org-Id
// request header, validates it as UUIDv7+, stores the parsed uuid.UUID in
// ctx via orgkey.SetOrgID, and injects an org_id attribute on the active
// OTel span.
//
// Locked contract (FOUND-03, D-19, D-20):
//
//  1. Missing or empty X-Org-Id           → HTTP 400 {"error":"invalid_org_id","reason":"missing_header"}
//  2. Header not parseable as UUID        → HTTP 400 {"error":"invalid_org_id","reason":"malformed_uuid"}
//  3. UUID present but Version() < 7      → HTTP 400 {"error":"invalid_org_id","reason":"uuidv7_required"}
//  4. Valid UUIDv7+                       → SetOrgID + span attr + next.ServeHTTP
//
// The three rejection reason strings are LOCKED — clients (admin UI, future
// SDKs) branch on them. Do NOT consolidate into a generic "bad_request"; the
// stable reason is the entire point of the structured shape (Pattern S3).
//
// Scope (D-21): this middleware runs ONLY inside the /v1 sub-router. The
// bypass routes /healthz, /readyz, /metrics MUST be mounted outside the
// chi.Route("/v1", ...) closure so they never reach this code path.
//
// Trust model:
//   - Phase 1 stub-auth: X-Org-Id is taken directly from the request header
//     (T-1-03 accepted disposition). Threat: a hostile client can claim any
//     org_id; we are not yet enforcing authentication.
//   - JWT-swap-ready: the middleware shape (parse external input → validate →
//     SetOrgID → call next) is identical for both header-source and
//     JWT-claim-source. When AUTH-01 (v1) replaces the header with a JWT,
//     only this file changes; downstream consumers (orgDB, handlers,
//     TracingHandler) are untouched.
//
// FOUND-07 OTel injection: after SetOrgID, we inject the org_id attribute
// onto the active span via trace.SpanFromContext(ctx).SetAttributes(...).
// Per PATTERNS.md Pitfall 5, this works as long as otelhttp.NewHandler wraps
// the chi mux (so the span exists when we run) and the attribute is set
// BEFORE next.ServeHTTP (so it lands on the right span).
func OrgContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		raw := r.Header.Get("X-Org-Id")
		if raw == "" {
			WriteError(ctx, w, http.StatusBadRequest, "invalid_org_id", "missing_header")
			return
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			WriteError(ctx, w, http.StatusBadRequest, "invalid_org_id", "malformed_uuid")
			return
		}
		if id.Version() < 7 {
			WriteError(ctx, w, http.StatusBadRequest, "invalid_org_id", "uuidv7_required")
			return
		}

		ctx = orgkey.SetOrgID(r.Context(), id)

		// FOUND-07: tag the active OTel span with org_id so every downstream
		// span and log line in the request can be filtered by tenant. The
		// span is created by otelhttp.NewHandler wrapping the chi mux (Plan
		// 06); SpanFromContext returns a non-recording no-op span if no
		// tracer is configured (e.g. in tests), and SetAttributes is a safe
		// no-op on that — so the call is always safe.
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("org_id", id.String()))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
