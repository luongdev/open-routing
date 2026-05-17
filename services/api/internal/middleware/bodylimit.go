// bodylimit.go — path-scoped HTTP body-size enforcement for the Phase 5
// bulk-import endpoint (D5-21).
//
// Phase 5 ships a single synchronous bulk-import endpoint capped at 50 MB
// per request body. Larger bodies must be rejected BEFORE any parser
// touches the bytes — both to bound memory (D5-21) and to keep the
// 50 MB hard cap honest against:
//
//  1. Malicious clients that send a giant body (multi-GB POST). The
//     Content-Length pre-flight short-circuits these.
//  2. Clients that lie about Content-Length OR send chunked transfer
//     encoding (no Content-Length at all). The http.MaxBytesReader wrap
//     surfaces a typed *http.MaxBytesError on mid-stream over-limit,
//     which the downstream handler maps to 413 via errors.As.
//  3. Generated strict-server JSON eager-decode (RESEARCH §F1). The
//     handler-level decode reads through the wrapped Body so eager decode
//     of an oversized JSON also hits the MaxBytesError path.
//
// Pre-flight is BEST-EFFORT (Content-Length is client-controlled and
// optional under chunked transfer-encoding); the wrap is the actual
// guard. See Pitfall 5 in 05-RESEARCH.md.
//
// Wire shape on 413 (locked — Wave 4 integration tests grep for it):
//
//	{"error": "invalid_body",
//	 "reason": "request_too_large_use_async_pathway",
//	 "request_id": "<uuid>"}
//
// The reason string deliberately points admins at IMP-09 (the deferred
// async pathway). Mid-stream over-limit emits the same reason from the
// handler — middleware can't peek inside io.Reader use, so the wire shape
// for that branch is owned by the handler that reads r.Body.
//
// Chain ordering (Plan 05-06 wires this): RequestID → OrgContext →
// BodyLimit("/v1/orgs/", 50<<20) → strict-server. Cheap header rejection
// (400 invalid X-Org-Id) fires BEFORE any body bytes are wrapped (Q7).
package middleware

import (
	"net/http"
	"strings"
)

// BodyLimit returns a chi MiddlewareFunc that rejects requests whose
// declared Content-Length exceeds maxBytes and wraps r.Body in
// http.MaxBytesReader so mid-stream over-limit also surfaces as
// *http.MaxBytesError (D5-21). pathPrefix scopes the middleware to a
// single URL prefix so other endpoints retain unlimited bodies.
//
// Inputs:
//   - maxBytes:   the cap; Phase 5 wires 50<<20 (50 MB).
//   - pathPrefix: only requests whose URL.Path starts with this prefix
//     are enforced; everything else passes through untouched. Use
//     "/v1/orgs/" to catch the entire org-scoped surface; use a more
//     specific prefix like "/v1/orgs/{org_id}/catalog/import" to limit
//     scope to the import endpoint only (path templates with placeholders
//     are matched lexically, not by chi route — be deliberate).
//
// The factory shape (function returning a middleware) matches chi
// MiddlewareFunc and the existing requestid.go / uuidv7path.go shapes.
//
// 413 emission uses the canonical WriteError from httputil.go — do NOT
// invent a parallel error emitter; the {error, reason, request_id}
// envelope contract (D-35) must stay uniform across all 4xx paths.
func BodyLimit(maxBytes int64, pathPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, pathPrefix) {
				next.ServeHTTP(w, r)
				return
			}
			// (a) Content-Length pre-flight. r.ContentLength is the
			//     stdlib's parsed Content-Length value: >0 when the
			//     header was present and well-formed, 0 for an empty
			//     body, and -1 for chunked transfer encoding / unknown
			//     length. Only the >maxBytes branch matters for the
			//     short-circuit; the chunked path falls through to the
			//     wrap below which is the actual mid-stream guard.
			//
			//     Reading r.ContentLength (the parsed long, not the
			//     header string) avoids the Content-Length-header-not-
			//     yet-projected pitfall seen with httptest.NewRequest
			//     and aligns with Go stdlib idiom (net/http internally
			//     populates r.ContentLength from the header at request
			//     parse time).
			if r.ContentLength > maxBytes {
				WriteError(r.Context(), w, http.StatusRequestEntityTooLarge,
					"invalid_body", "request_too_large_use_async_pathway")
				return
			}
			// (b) Wrap r.Body so a mid-stream over-limit (chunked transfer
			//     encoding, missing/lying Content-Length) also fails. The
			//     downstream handler receives an *http.MaxBytesError on
			//     Read and is responsible for mapping that to 413 via
			//     errors.As — the middleware cannot peek inside io.Reader
			//     use to detect mid-stream failures itself.
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
