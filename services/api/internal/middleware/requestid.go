package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// requestIDCtxKey is the canonical, unexported context key for the per-request
// UUIDv7 request id. Empty struct + unexported name prevents collisions with
// other packages (S1: unexported empty-struct key types).
type requestIDCtxKey struct{}

// RequestID generates a UUIDv7 per request, stores it in ctx, and echoes it
// via the X-Request-Id response header.
//
// Why not chi's stdlib RequestID? chi.RequestID emits an atomic-counter id of
// the form "host/N-K" which is not a UUID. D-28 mandates UUIDv7 so that the
// id is monotonic-by-time, globally unique, and joinable with trace_id /
// span_id in slog / OTel pipelines.
//
// The X-Request-Id response header is set BEFORE next.ServeHTTP runs. This
// guarantees the header lands on the response even when a downstream handler
// fails to write a body (e.g. panics — chi's Recoverer then emits the 500
// response with the header already set).
//
// uuid.Must(uuid.NewV7()) panics only on entropy exhaustion (effectively
// never). chi's Recoverer middleware — registered ahead of RequestID in
// Plan 06's chain — catches the panic and emits HTTP 500. Acceptable failure
// mode per the T-1-UUID-PANIC threat disposition.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.Must(uuid.NewV7())
		ctx := context.WithValue(r.Context(), requestIDCtxKey{}, id)
		w.Header().Set("X-Request-Id", id.String())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the per-request UUIDv7 id stored by the
// RequestID middleware. Returns (uuid.Nil, false) when ctx does not carry
// one — happens in pre-middleware code paths (e.g. bypass routes that run
// before RequestID is mounted, or in test code that bypasses the middleware).
func RequestIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(requestIDCtxKey{}).(uuid.UUID)
	return id, ok
}
