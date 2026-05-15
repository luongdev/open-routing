package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// TestTracingHandler_AddsTraceID verifies that when ctx carries an active
// OTel span, the emitted JSON record contains trace_id and span_id keys
// whose values match the SpanContext. Uses an in-memory tracer provider so
// no exporter / network is required.
func TestTracingHandler_AddsTraceID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewTracingHandler(inner)

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	logger := slog.New(handler)
	logger.InfoContext(ctx, "hello")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	traceID, ok := rec["trace_id"].(string)
	require.True(t, ok, "trace_id missing: %s", buf.String())
	spanID, ok := rec["span_id"].(string)
	require.True(t, ok, "span_id missing: %s", buf.String())
	require.Equal(t, span.SpanContext().TraceID().String(), traceID)
	require.Equal(t, span.SpanContext().SpanID().String(), spanID)
}

// TestTracingHandler_AddsOrgID verifies that when ctx carries an org_id via
// orgkey.SetOrgID, the emitted JSON record contains an org_id field with the
// UUID's string form. No span in this scope — proves the org_id branch is
// independent of the OTel branch.
func TestTracingHandler_AddsOrgID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	handler := NewTracingHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	orgID := uuid.Must(uuid.NewV7())
	ctx := orgkey.SetOrgID(context.Background(), orgID)

	slog.New(handler).InfoContext(ctx, "scoped")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	require.Equal(t, orgID.String(), rec["org_id"], "org_id missing: %s", buf.String())
}

// TestTracingHandler_NoSpan_NoTraceFields verifies the handler is silent
// when ctx has neither a span nor an org_id. This is the bypass-path case
// (health/readyz/metrics) — adding empty trace_id/span_id/org_id would
// pollute logs with noise.
func TestTracingHandler_NoSpan_NoTraceFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	handler := NewTracingHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	slog.New(handler).InfoContext(context.Background(), "naked")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	_, hasTrace := rec["trace_id"]
	_, hasSpan := rec["span_id"]
	_, hasOrg := rec["org_id"]
	require.False(t, hasTrace, "trace_id should be absent: %v", rec)
	require.False(t, hasSpan, "span_id should be absent: %v", rec)
	require.False(t, hasOrg, "org_id should be absent: %v", rec)
}
