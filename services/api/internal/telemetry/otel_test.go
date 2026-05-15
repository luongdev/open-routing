package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/luongdev/open-routing/services/api/internal/config"
)

// TestInitOTel_StdoutSuccess verifies that the default ("stdout") exporter
// produces a non-nil shutdown func and registers the global TracerProvider.
// No t.Parallel() — InitOTel mutates global state via otel.SetTracerProvider.
func TestInitOTel_StdoutSuccess(t *testing.T) {
	cfg := &config.Config{OTelExporter: "stdout"}
	shutdown, err := InitOTel(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	require.NotNil(t, otel.GetTracerProvider())
	require.NoError(t, shutdown(context.Background()))
}

// TestInitOTel_UnknownExporterReturnsError covers the default switch arm and
// asserts the error message identifies the offending env var so callers
// get a debuggable signal.
func TestInitOTel_UnknownExporterReturnsError(t *testing.T) {
	cfg := &config.Config{OTelExporter: "bogus"}
	_, err := InitOTel(context.Background(), cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown OTEL_EXPORTER")
}

// TestInitOTel_OTLPDoesNotPanic ensures the OTLP-exporter path constructs
// successfully without a live collector. otlptracehttp.New lazily connects,
// so InitOTel returns a valid shutdown even when the endpoint is unreachable.
// VALIDATION.md defers live-endpoint OTLP verification to Phase 2+.
func TestInitOTel_OTLPDoesNotPanic(t *testing.T) {
	cfg := &config.Config{OTelExporter: "otlp", OTLPEndpoint: "http://localhost:4318"}
	shutdown, err := InitOTel(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	// Best-effort shutdown; will likely return a connection error and that's fine.
	_ = shutdown(context.Background())
}
