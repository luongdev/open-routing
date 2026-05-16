package middleware

// UUIDv7PathParams validates all {id} path parameters in a chi route context
// are UUIDv7 (version 7 or higher). Non-v7 UUIDs receive a 400 invalid_id
// response before the handler runs.
//
// Locked contract (REVIEWS HIGH #3, D-19):
//
//	The spec documents that UUIDv4 and lower are rejected at path level. Without
//	this middleware a UUIDv4 path param silently reaches the handler, which then
//	performs a DB lookup — the lookup returns no row (because ids are v7) and
//	the response is a 404 "not found". This is indistinguishable from a valid
//	cross-org probe (FOUND-08 disposition), confusing incident response.
//
//	With this middleware, a UUIDv4 path param returns 400 invalid_id BEFORE any
//	DB access, which is the correct disposition (D-19).
//
// Scope: only `id` path parameters are validated here. The `org_id` path
// parameter is already validated by OrgContext middleware (orgcontext.go) and
// excluded from this check to avoid double-validation.
//
// Mount: wire via chi.Route("/v1", func(r chi.Router) { r.Use(UUIDv7PathParams) })
// or in the ChiServerOptions.Middlewares slice applied to /v1/* routes. The
// middleware is a no-op on requests without an `id` path parameter.
//
// Note: chi.RouteContext(ctx).URLParams contains the matched params ONLY when
// the middleware runs INSIDE a chi sub-router (after chi route matching). It is
// safe to apply this inside the api.ChiServerOptions.Middlewares slice because
// that slice runs after HandlerWithOptions resolves the route.
import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// UUIDv7PathParams is a chi MiddlewareFunc that inspects all `id` path
// parameters matched by the router and rejects any that are not UUIDv7+.
// Parameters named `org_id` are excluded (handled by OrgContext).
func UUIDv7PathParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		rctx := chi.RouteContext(ctx)
		if rctx == nil {
			// No route context — not inside a chi router; skip validation.
			next.ServeHTTP(w, r)
			return
		}

		keys := rctx.URLParams.Keys
		values := rctx.URLParams.Values
		for i, key := range keys {
			// Only validate parameters named "id"; skip "org_id" (OrgContext owns it)
			// and any other params like "cursor", "status" which are not UUIDs.
			if key != "id" {
				continue
			}
			if i >= len(values) {
				continue
			}
			raw := values[i]
			id, err := uuid.Parse(raw)
			if err != nil {
				WriteError(ctx, w, http.StatusBadRequest, "invalid_id", "malformed_uuid")
				return
			}
			if id.Version() < 7 {
				WriteError(ctx, w, http.StatusBadRequest, "invalid_id", "uuidv7_required")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
