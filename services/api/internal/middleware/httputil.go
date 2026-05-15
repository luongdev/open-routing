// Package middleware provides chi-compatible HTTP middleware and shared
// response helpers for the API server.
//
// httputil.go defines the canonical JSON error response shape (Shared Pattern
// S3 from PATTERNS.md) and a generic JSON success writer. Every error path in
// every middleware and handler in services/api MUST emit responses through
// WriteError / WriteJSON so the on-the-wire shape stays uniform.
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// errorBody is the canonical error response shape (Shared Pattern S3, D-35).
//
// Three fields, lowercase JSON tags:
//   - Error     — stable machine-readable code (e.g. "invalid_org_id", "not_found")
//   - Reason    — human-readable / debugging detail (e.g. "missing_header")
//   - RequestID — UUIDv7 from ctx, embedded for support DX (D-35); omitted
//     when ctx has no request id (bypass routes, pre-middleware paths).
//
// Never include stack traces, internal paths, goroutine numbers, or any
// uncontrolled error.Error() string in this body. Per ASVS V7.4 /
// V14.4 / V7.1.2: server-side error details stay in slog, never in
// the HTTP response.
//
// Future error shapes (validation field errors, bulk import row errors) are
// added via SEPARATE response structs in a non-breaking way (e.g. Phase 5's
// {failed: [{row, field, message}]} bulk import shape) — never by extending
// errorBody with optional fields.
type errorBody struct {
	Error     string `json:"error"`
	Reason    string `json:"reason"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteError writes a JSON error response with the canonical shape.
//
// Parameters:
//   - ctx:    request context — used to extract the per-request UUIDv7 id
//     minted by the RequestID middleware (D-28, D-35). When the ctx does not
//     carry a request id (bypass routes, test code that bypasses middleware),
//     the "request_id" field is omitted from the response body (omitempty).
//   - w:      the HTTP response writer
//   - status: HTTP status code (4xx / 5xx)
//   - code:   stable error identifier — e.g. "invalid_org_id", "not_found".
//     Clients (admin UI, future SDKs) branch on this value.
//   - reason: contextual detail — e.g. "missing_header", "malformed_uuid".
//     Suitable for log lines and developer debugging.
//
// If JSON encoding fails (programming error / OOM), the failure is logged via
// slog.Error and the client receives an empty body after the already-sent
// headers. Per ASVS, we do not surface the underlying encode error.
func WriteError(ctx context.Context, w http.ResponseWriter, status int, code, reason string) {
	body := errorBody{Error: code, Reason: reason}
	if id, ok := RequestIDFromContext(ctx); ok {
		body.RequestID = id.String()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("httputil: encode error response", "err", err, "code", code)
	}
}

// WriteJSON writes a JSON success response with the given body and status.
//
// Used by handlers (Plan 06 and later) for 200/201/204 responses. The body
// is encoded via the stdlib JSON encoder; callers are responsible for shape.
//
// If encoding fails (programming error / OOM), the failure is logged via
// slog.Error and the client receives a partial body. Per ASVS, we do not
// surface the underlying encode error.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("httputil: encode json response", "err", err)
	}
}
