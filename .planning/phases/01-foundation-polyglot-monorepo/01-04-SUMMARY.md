---
phase: 01-foundation-polyglot-monorepo
plan: 04
subsystem: observability
tags: [opentelemetry, otel, slog, tracing, structured-logging, go, semconv]

# Dependency graph
requires:
  - phase: 01-foundation-polyglot-monorepo
    provides: "config.Config struct with OTelExporter/OTLPEndpoint/OTLPProtocol fields (Plan 01)"
  - phase: 01-foundation-polyglot-monorepo
    provides: "orgkey.SetOrgID / orgkey.OrgIDFromContext context-key helpers (Plan 01-1a)"
provides:
  - "telemetry.InitOTel(ctx, cfg) — tracer-provider construction, exporter selection (stdout/otlp), W3C propagators, shutdown handle"
  - "telemetry.TracingHandler — slog.Handler wrapper that injects trace_id, span_id, org_id on every Handle call"
  - "telemetry.NewTracingHandler(inner) constructor"
  - "Unit-test contract: 6 tests covering stdout/otlp/unknown exporter paths and trace/org injection"
affects: [01-05, 01-06, 03-catalog, 04-agent-state, 06-admin-ui]

# Tech tracking
tech-stack:
  added:
    - "go.opentelemetry.io/otel/sdk/trace (v1.43.0) — TracerProvider, BatchSpanProcessor, SyncSpanProcessor"
    - "go.opentelemetry.io/otel/exporters/stdout/stdouttrace (v1.43.0) — dev/CI span exporter"
    - "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp (v1.43.0) — prod OTLP/HTTP exporter"
    - "go.opentelemetry.io/otel/semconv/v1.26.0 — service.name / service.version resource attributes"
    - "go.opentelemetry.io/otel/propagation — W3C TraceContext + Baggage composite propagator"
  patterns:
    - "Pattern: env-driven exporter selection (cfg.OTelExporter switch) keeps dev/CI/prod on one InitOTel API"
    - "Pattern: WithSyncer for stdout (immediate flush, dev feedback), WithBatcher for OTLP (prod efficiency)"
    - "Pattern: slog.Handler composition — TracingHandler wraps slog.NewJSONHandler so JSON output (D-27) is preserved while correlation fields are appended in Handle"
    - "Pattern: best-effort context lookups in Handle — silent when ctx has no span/no org_id (bypass paths emit clean records)"

key-files:
  created:
    - "services/api/internal/telemetry/otel.go — InitOTel function, exporter switch, propagator registration"
    - "services/api/internal/telemetry/slog.go — TracingHandler struct + 4 slog.Handler methods"
    - "services/api/internal/telemetry/otel_test.go — InitOTel exporter selection and error-path tests"
    - "services/api/internal/telemetry/slog_test.go — TracingHandler trace_id/span_id/org_id injection tests"
  modified: []

key-decisions:
  - "Custom TracingHandler over otelslog bridge — preserves D-27 JSON-to-stdout and avoids routing logs through the OTel logs SDK"
  - "stdouttrace uses sdktrace.WithSyncer (per Open Question #1 recommendation) — immediate flush for dev feedback"
  - "otlptracehttp uses sdktrace.WithBatcher — production efficiency, defers connect to first emit"
  - "InitOTel returns the shutdown func directly (not a wrapped lifecycle handle) — caller-side defer in main() is the explicit pattern Plan 06 will adopt"
  - "semconv v1.26.0 chosen (latest stable in OTel v1.43 SDK; matches CONTEXT.md decision lineage)"

patterns-established:
  - "Telemetry init ordering: InitOTel(ctx, cfg) MUST be the first call inside cmd/api/main.go before any chi router or middleware construction (FOUND-07 hard rule; enforced by Plan 06)"
  - "slog correlation chain: chi middleware writes org_id into ctx via orgkey.SetOrgID, then TracingHandler.Handle reads it back on every log record"
  - "Bypass-path quiet behavior: TracingHandler emits no trace_id/span_id/org_id keys when ctx lacks them — proven by TestTracingHandler_NoSpan_NoTraceFields"

requirements-completed: [FOUND-07]

# Metrics
duration: ~12min
completed: 2026-05-15
---

# Phase 1 Plan 04: OTel SDK + slog TracingHandler Summary

**OTel tracer-provider init with env-driven stdout/OTLP exporter selection plus a slog.Handler wrapper that auto-injects trace_id, span_id, and org_id on every JSON log record.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-05-15T10:30Z (approx)
- **Completed:** 2026-05-15T10:42Z
- **Tasks:** 4 (all auto, no checkpoints, no deviations)
- **Files created:** 4
- **Files modified:** 0

## Accomplishments

- `InitOTel(ctx, cfg)` constructs the global TracerProvider, registers W3C TraceContext + Baggage propagators, and hands back a `Shutdown` func that Plan 06's `main()` will `defer`. The exporter switch covers the two D-14 cases (`stdout` → `WithSyncer` for dev/CI; `otlp` → `WithBatcher` for prod via `OTEL_EXPORTER_OTLP_ENDPOINT`) and rejects unknown values with a descriptive error.
- `TracingHandler` wraps any `slog.Handler` (in production, `slog.NewJSONHandler(os.Stdout, ...)` per D-27). On every `Handle` call it reads `trace.SpanFromContext(ctx)` and `orgkey.OrgIDFromContext(ctx)` and appends `trace_id`, `span_id`, `org_id` attributes — best-effort; bypass paths (`/healthz`, `/readyz`, `/metrics`) emit clean records without empty keys.
- Six unit tests cover the contract: exporter selection (stdout/otlp/unknown), shutdown handle, trace_id/span_id presence, org_id presence, and quiet behavior with naked ctx. Zero testcontainer dependency — in-memory `sdktrace.NewTracerProvider` and `bytes.Buffer` only.
- Confirmed no Jaeger / no collector container added (D-15 holds): dev stack remains stdout-only via `air` foreground or `docker compose logs`.

## Public API exported

The two pieces Plan 06 consumes:

```go
// services/api/internal/telemetry/otel.go
func InitOTel(ctx context.Context, cfg *config.Config) (func(context.Context) error, error)

// services/api/internal/telemetry/slog.go
type TracingHandler struct { /* inner slog.Handler */ }
func NewTracingHandler(inner slog.Handler) *TracingHandler
func (h *TracingHandler) Enabled(ctx context.Context, level slog.Level) bool
func (h *TracingHandler) Handle(ctx context.Context, rec slog.Record) error
func (h *TracingHandler) WithAttrs(attrs []slog.Attr) slog.Handler
func (h *TracingHandler) WithGroup(name string) slog.Handler
```

## Ordering guarantee for Plan 06

`InitOTel` MUST be the **first** call inside `cmd/api/main.go` `main()`:

1. `shutdown, err := telemetry.InitOTel(ctx, cfg)` — registers global TracerProvider + propagators.
2. `handler := telemetry.NewTracingHandler(slog.NewJSONHandler(os.Stdout, ...))` then `slog.SetDefault(slog.New(handler))` — slog default now reads the OTel global.
3. `mux := server.NewMux(...)` — chi router construction.
4. `rootHandler := otelhttp.NewHandler(mux, "open-routing-api")` — wraps the mux.
5. `srv.ListenAndServe()`.

Steps 1–2 BEFORE step 3 is the FOUND-07 hard rule. Inverting them causes Pitfall 5 (spans missing org_id even though `OrgContext` middleware logs success).

## Task Commits

1. **Task 1: Implement InitOTel** — `475a84e` (feat)
2. **Task 2: Implement TracingHandler** — `e00fcf6` (feat)
3. **Task 3: Write otel_test.go** — `4fd099b` (test)
4. **Task 4: Write slog_test.go** — `9715c09` (test)

## Test Results

```
=== RUN   TestInitOTel_StdoutSuccess
--- PASS: TestInitOTel_StdoutSuccess (0.00s)
=== RUN   TestInitOTel_UnknownExporterReturnsError
--- PASS: TestInitOTel_UnknownExporterReturnsError (0.00s)
=== RUN   TestInitOTel_OTLPDoesNotPanic
--- PASS: TestInitOTel_OTLPDoesNotPanic (0.00s)
=== RUN   TestTracingHandler_AddsTraceID
--- PASS: TestTracingHandler_AddsTraceID (0.00s)
=== RUN   TestTracingHandler_AddsOrgID
--- PASS: TestTracingHandler_AddsOrgID (0.00s)
=== RUN   TestTracingHandler_NoSpan_NoTraceFields
--- PASS: TestTracingHandler_NoSpan_NoTraceFields (0.00s)
PASS
ok      github.com/luongdev/open-routing/services/api/internal/telemetry        0.254s
```

`go vet ./...` clean across the whole module; `go build ./internal/telemetry/...` clean.

## Files Created/Modified

- `services/api/internal/telemetry/otel.go` — `InitOTel` function: resource construction (semconv v1.26.0), exporter switch (stdout/otlp), TracerProvider + propagator registration, returns `Shutdown`.
- `services/api/internal/telemetry/slog.go` — `TracingHandler` struct wrapping `slog.Handler`; implements `Enabled` / `Handle` / `WithAttrs` / `WithGroup`. `Handle` injects `trace_id`, `span_id` (from `trace.SpanFromContext`), and `org_id` (from `orgkey.OrgIDFromContext`).
- `services/api/internal/telemetry/otel_test.go` — 3 tests: stdout success, unknown-exporter error, OTLP construct-without-collector.
- `services/api/internal/telemetry/slog_test.go` — 3 tests: trace-id injection via in-memory `sdktrace.NewTracerProvider`, org-id injection via `orgkey.SetOrgID`, naked-ctx silence.

## Decisions Made

- **Custom `TracingHandler` over the `otelslog` bridge** — per Research §"Don't Hand-Roll" (line 983). `otelslog` replaces the inner handler entirely and routes records through the OTel logs SDK; incompatible with D-27's lock that logs are JSON-to-stdout everywhere. The custom wrapper is ~70 lines including doc comments, full control over field shape.
- **`WithSyncer` for stdout, `WithBatcher` for OTLP** — locked by Open Question #1 recommendation. Dev/CI want immediate flush (so a fresh request's span shows up in `docker compose logs` immediately); prod wants batching for efficiency.
- **semconv v1.26.0** — the version explicitly named in the PLAN's code template. Verified present in the local OTel v1.43.0 SDK module cache before writing the import.
- **No early-return on `resource.New` error suppressed** — research code did `res, _ := resource.New(...)` but PLAN spec required `if err != nil { return nil, fmt.Errorf(...) }`. Followed PLAN: explicit error propagation per Go-idiomatic style and to surface resource misconfig early.

## Deviations from Plan

None — plan executed exactly as written. The action blocks in each task contained the exact source to write; verification commands all passed on first invocation. No Rules 1-4 triggered. No auth gates. No package install failures. No architectural questions surfaced.

## Issues Encountered

None.

## Self-Check

Verifying claims:

- `services/api/internal/telemetry/otel.go`: **FOUND** (3273 bytes)
- `services/api/internal/telemetry/slog.go`: **FOUND** (2728 bytes)
- `services/api/internal/telemetry/otel_test.go`: **FOUND** (1805 bytes)
- `services/api/internal/telemetry/slog_test.go`: **FOUND** (3060 bytes)
- Commit `475a84e` (Task 1): **FOUND** in git log
- Commit `e00fcf6` (Task 2): **FOUND** in git log
- Commit `4fd099b` (Task 3): **FOUND** in git log
- Commit `9715c09` (Task 4): **FOUND** in git log
- `go vet ./...` exit 0: **PASSED**
- `go build ./internal/telemetry/...` exit 0: **PASSED**
- `go test -count=1 ./internal/telemetry/...` 6/6 PASS: **PASSED**

## Self-Check: PASSED

## Known Stubs

None. All artifacts are functional. `InitOTel` and `TracingHandler` are wired against real OTel + slog APIs; tests exercise the contract end-to-end (in-memory tracer for trace path, real `orgkey.SetOrgID` for org path).

## User Setup Required

None — this plan creates pure-Go telemetry primitives. No external services or dashboards involved. Phase 1 keeps the dev stack stdout-only (D-15); production deployments will set `OTEL_EXPORTER=otlp` and `OTEL_EXPORTER_OTLP_ENDPOINT=https://...` per their collector setup, but no Phase 1 environment configuration is needed.

## Next Phase Readiness

Plan 05 (RequestID + chi middleware chain) and Plan 06 (cmd/api/main.go wiring) can now import:

- `internal/telemetry.InitOTel(ctx, cfg)` — call in main() before router setup.
- `internal/telemetry.NewTracingHandler(slog.NewJSONHandler(os.Stdout, opts))` — pass result to `slog.SetDefault(slog.New(handler))`.

Plan 06 will additionally wrap the chi mux with `otelhttp.NewHandler(mux, "open-routing-api")` to bind incoming HTTP spans to the global TracerProvider this plan registered. The mux wrap happens AFTER `mux.Use(...)` calls but BEFORE `http.ListenAndServe` — that ordering is Plan 06's responsibility; this plan's API design encourages it but does not enforce it directly.

No blockers, no concerns flagged. FOUND-07 (telemetry init ordering rule) is now satisfiable end-to-end once Plan 06 wires the pieces together.

---
*Phase: 01-foundation-polyglot-monorepo*
*Plan: 04 — OTel SDK + slog TracingHandler*
*Completed: 2026-05-15*
