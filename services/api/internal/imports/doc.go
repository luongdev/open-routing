// Package imports implements the Phase 5 bulk-import endpoints
// (BulkImportCatalog + GetImportJob) and the supporting crash-recovery
// sweep goroutine.
//
// # Single owner
//
// One struct, [Importer], owns every long-running concern (sweep
// goroutine + handler methods promoted into the cmd/api ApiHandlers
// composite). The type is named [Importer] (NOT Server) to avoid the
// composite-selector ambiguity Go reports when two anonymous embedded
// fields share a type name. cmd/api/main.go already embeds
// *catalog.Handlers and *state.Server in its ApiHandlers struct
// (D-89 / Phase 4 Pitfall 1); adding *imports.Server would collide
// with *state.Server and refuse to compile. Renaming the Phase 5
// struct here is the clean fix (RESEARCH §F3 Open Q5).
//
// # Wave-by-wave file map
//
// Plan 05-04 (this wave — package skeleton) authors:
//
//   - handlers.go   — [Importer] struct + Deps + Option + New + Start + Stop
//     (lifecycle mirrors state.Server verbatim; clockwork.Clock seam for tests).
//   - coerce.go     — typed pipeline trim → lower → split → parse dispatched
//     by sqlc field type (D5-01..D5-08). Sentinel errors here.
//   - parser_csv.go — encoding/csv reader with BOM strip + UTF-8 validate
//     + strict quote/header policy (D5-05, D5-08; PITFALLS 5.1).
//   - parser_json.go — reMarshalAs[T any] generic helper (F1 accommodation:
//     strict-server eager-decodes into []interface{}; we re-marshal per row
//     into the typed Import*Request shape).
//   - header.go     — per-entity column registry + strict header validator
//     (D5-05). Reuses catalog.ValidateCodeFormat for `code` columns.
//   - mappers.go    — generated.ImportJob → api.ImportJob wire conversion.
//   - errors.go     — rowError + wrapPgError thin adapter over the
//     catalog.MapPgError export from Plan 05-02.
//   - jobs.go       — createJob + finaliseJob lifecycle (D5-10).
//   - idempotency.go — lookupIdempotentReplay + rehydrateBulkImportResult
//     (D5-13 replay path; KNOWN LIMITATION: succeeded[] empty on replay).
//   - sweep.go      — safetySweep + runSweepPastDue + startupSweep mirroring
//     state/ttl.go verbatim, parameterised to import_jobs (D5-11). Calls
//     SweepCrashedImportJobs via db.WithBypass(ctx, "import_crash_sweep")
//     because the sweep is intentionally org-agnostic and is the LONE
//     SQLChecker exception in the entire codebase.
//
// Plan 05-05 (Wave 3 — row processors + chunk loop) will add:
//
//   - chunk.go              — batched-savepoint orchestrator (D5-09).
//   - rows_agents.go        — per-row processor for entity=agents (incl. skills merge).
//   - rows_skills.go        — entity=skills.
//   - rows_queues.go        — entity=queues.
//   - rows_channels.go      — entity=channels (FK probe for default_queue_code).
//   - rows_adapters.go      — entity=adapters.
//   - rows_break_reasons.go — entity=break_reasons.
//
// Plan 05-06 (Wave 4 — handler wiring) will add:
//
//   - The BulkImportCatalog and GetImportJob method bodies on *Importer
//     (currently stubbed in catalog/notimpl.go until Wave 4 deletes them).
//
// # Invariants this package owns
//
// Decision IDs (see .planning/phases/05-bulk-import-go/05-CONTEXT.md):
//
//   - D5-01..D5-08 — typed coercion pipeline; charset is UTF-8 only.
//   - D5-09        — 50-row chunk size, batched savepoints (Wave 3).
//   - D5-10        — import_jobs lifecycle: INSERT-pending → process →
//     UPDATE-final.
//   - D5-11        — 1h crash-recovery sweep; 24h TTL; org-agnostic via
//     db.WithBypass(ctx, "import_crash_sweep") — the SOLE WithBypass
//     caller outside cmd/migrate.
//   - D5-13, D5-27 — Idempotency-Key replay returns idempotent_replay=true
//     with succeeded[] empty (v0.1 KNOWN LIMITATION).
//   - D5-21        — 50 MB body limit via http.MaxBytesReader middleware.
//   - D5-22        — 500-row hard cap.
//   - D5-23        — JSON parsing path accepts the strict-server's
//     eager-decoded []interface{} body and per-row re-marshals.
//   - D5-25..D5-27 — OpenAPI extensions (Import*Request schemas, ImportJob
//     status enum, Idempotency-Key header) live in openapi.yaml.
//
// # Reuse boundary
//
// Only two helpers are imported from sibling packages:
//
//   - catalog.ValidateCodeFormat — Phase 04.1 regex `^[a-z][a-z0-9_]{0,63}$`.
//     Reused (not duplicated) for the `code` column and for the skill_code
//     part of agent skills tokens (D5-15, D5-17).
//   - catalog.MapPgError — the constraint-name introspection that maps
//     pgconn.PgError codes to (status, ErrorCode, reason). Wave 3 row
//     processors call wrapPgError(err, entity) which delegates to this.
//
// No other catalog internals are touched; no Phase 5 → Phase 3 coupling
// beyond these two exported helpers.
package imports
