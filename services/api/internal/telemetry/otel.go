// Package telemetry initializes the OpenTelemetry tracer provider and the
// custom slog handler that decorates every log record with correlation
// identifiers (trace_id, span_id, org_id). It is consumed by cmd/api/main.go
// (Plan 06) — InitOTel is intended to be the FIRST call inside main(),
// BEFORE chi.NewRouter() is constructed (D-14, FOUND-07 hard rule).
//
// This package is allowed to depend on internal/config and
// internal/db/orgkey; it MUST NOT import internal/server, internal/middleware,
// or any handler package — those depend on this one.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/luongdev/open-routing/services/api/internal/config"
)

// InitOTel constructs the tracer provider per cfg.OTelExporter, sets it as the
// global provider, registers the W3C TraceContext + Baggage propagator, and
// returns the provider's Shutdown function for cleanup. MUST be called BEFORE
// any HTTP handler construction (FOUND-07).
//
// Exporter selection (D-14):
//
//	"stdout" -> stdouttrace with WithSyncer (immediate flush, dev/CI friendly)
//	"otlp"   -> otlptracehttp with WithBatcher (default; respects
//	            OTEL_EXPORTER_OTLP_ENDPOINT / OTEL_EXPORTER_OTLP_PROTOCOL env)
//
// Anti-pattern guard: do NOT wrap otel.SetTracerProvider inside another
// function. The global registration must happen exactly once, before any
// consumer (including the slog TracingHandler) reads trace.SpanFromContext.
func InitOTel(ctx context.Context, cfg *config.Config) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("open-routing-api"),
			semconv.ServiceVersion("0.1.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	var exporter sdktrace.SpanExporter
	var tpOpts []sdktrace.TracerProviderOption
	tpOpts = append(tpOpts, sdktrace.WithResource(res))

	switch cfg.OTelExporter {
	case "otlp":
		otlpExp, err := otlptracehttp.New(ctx) // reads OTEL_EXPORTER_OTLP_ENDPOINT / _PROTOCOL env
		if err != nil {
			return nil, fmt.Errorf("telemetry: otlp exporter: %w", err)
		}
		exporter = otlpExp
		tpOpts = append(tpOpts, sdktrace.WithBatcher(exporter))
	case "stdout":
		stdoutExp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("telemetry: stdout exporter: %w", err)
		}
		exporter = stdoutExp
		// WithSyncer for dev/CI per Open Question #1 — immediate flush for
		// "I just hit an endpoint, where's the span?" feedback.
		tpOpts = append(tpOpts, sdktrace.WithSyncer(exporter))
	default:
		return nil, fmt.Errorf("telemetry: unknown OTEL_EXPORTER %q (expected 'stdout' or 'otlp')", cfg.OTelExporter)
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp.Shutdown, nil
}
