package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// TracingHandler wraps any slog.Handler and decorates every record with the
// active OTel span's trace_id + span_id (when present) and the request's
// org_id (when present). Satisfies D-29: every log line carries trace_id,
// span_id, and (when in an org-scoped request) org_id.
//
// Use NewTracingHandler(slog.NewJSONHandler(os.Stdout, ...)) and call
// slog.SetDefault(slog.New(handler)) in cmd/api/main.go (Plan 06).
//
// Why a custom wrapper rather than the OTel slog bridge (otelslog)? Per
// RESEARCH.md "Don't Hand-Roll" (line 983): otelslog replaces the inner
// handler entirely and routes logs through the OTel logs SDK — incompatible
// with D-27 "JSON to stdout everywhere". This wrapper keeps JSON output and
// adds correlation fields in ~30 lines, full control.
type TracingHandler struct {
	inner slog.Handler
}

// NewTracingHandler returns a TracingHandler wrapping inner. The inner
// handler is expected to be a slog.NewJSONHandler in production (D-27); in
// tests it can be any slog.Handler — typically slog.NewJSONHandler with a
// bytes.Buffer destination for assertions.
func NewTracingHandler(inner slog.Handler) *TracingHandler {
	return &TracingHandler{inner: inner}
}

// Enabled delegates to the wrapped handler.
func (h *TracingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle injects trace_id, span_id, and org_id then delegates emission.
func (h *TracingHandler) Handle(ctx context.Context, rec slog.Record) error {
	// OTel correlation: only attach if the span is valid (recording or not).
	if span := trace.SpanFromContext(ctx); span != nil && span.SpanContext().IsValid() {
		sc := span.SpanContext()
		rec.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	// org_id correlation: best-effort. Health endpoints and pre-middleware
	// log lines do not carry org_id — that is expected and correct (D-21
	// bypass paths skip the org middleware).
	if id, ok := orgkey.OrgIDFromContext(ctx); ok {
		rec.AddAttrs(slog.String("org_id", id.String()))
	}
	return h.inner.Handle(ctx, rec)
}

// WithAttrs creates a new handler whose inner records carry the given attrs.
func (h *TracingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TracingHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup creates a new handler whose inner records belong to the named group.
func (h *TracingHandler) WithGroup(name string) slog.Handler {
	return &TracingHandler{inner: h.inner.WithGroup(name)}
}
