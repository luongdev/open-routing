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
// # Landed file map (Phase 5 Plans 04..07 + 09 fix-up)
//
// Phase 5 plans 04..07 landed the full implementation; plan 09 (this
// commit's parent) addressed the cross-AI review HIGH/MED findings.
// The file map reflects the SHIPPED state — not a forward-looking
// wave plan — so future authors see what's actually here.
//
// Core infrastructure (Plan 05-04 — Wave 2 package skeleton):
//
//   - handlers.go    — [Importer] struct + Deps + Option + New + Start +
//     Stop (lifecycle mirrors state.Server verbatim; clockwork.Clock
//     seam for tests; M3-fix-up adds WithFinaliseOverride hook for
//     audit-failure injection).
//   - coerce.go      — typed pipeline trim → lower → split → parse
//     dispatched by sqlc field type (D5-01..D5-08). Sentinel errors
//     declared here (ErrInvalidBool, ErrInvalidInt, ErrInvalidJSON,
//     ErrInvalidCodeFormat, etc.).
//   - parser_csv.go  — encoding/csv reader with BOM strip + UTF-8
//     validate + strict quote/header policy (D5-05, D5-08;
//     PITFALLS 5.1).
//   - parser_json.go — reMarshalAs[T any] generic helper (F1
//     accommodation: strict-server eager-decodes into []interface{};
//     we re-marshal per row into the typed Import*Request shape).
//   - header.go      — per-entity column registry + strict header
//     validator (D5-05). Reuses catalog.ValidateCodeFormat for `code`
//     columns. coerceJSONBObject parses adapter `config` cells at
//     this layer (M3 fix-up — surfaces invalid_json as field-level
//     error).
//   - mappers.go     — generated.ImportJob → api.ImportJob wire
//     conversion.
//   - errors.go      — rowError + wrapPgError thin adapter over the
//     catalog.MapPgError export.
//   - jobs.go        — createJob + finaliseJob lifecycle (D5-10).
//     finaliseJob honours the test override hook (H3 fix-up).
//   - idempotency.go — lookupIdempotentReplay + rehydrateBulkImportResult
//     (D5-13 replay path; KNOWN LIMITATION: succeeded[] empty on
//     replay; M5 fix-up — replay status code now mirrors original
//     200/207/422).
//   - sweep.go       — safetySweep + runSweepPastDue + startupSweep
//     mirroring state/ttl.go verbatim, parameterised to import_jobs
//     (D5-11). Calls SweepCrashedImportJobs via
//     db.WithBypass(ctx, "import_crash_sweep") because the sweep is
//     intentionally org-agnostic and is the LONE SQLChecker exception
//     in the entire codebase.
//
// Chunk orchestrator + row processors (Plan 05-05 — Wave 3):
//
//   - chunk.go               — batched-savepoint orchestrator (D5-09).
//     M1+M2 fix-up: savepoint BEGIN/RELEASE failures now abort the
//     chunk and roll back already-succeeded rows.
//   - row_agent.go           — entity=agents row processor (incl.
//     skills merge). M4 fix-up: proficiency 1..10 validated pre-DB.
//   - row_skill.go           — entity=skills.
//   - row_queue.go           — entity=queues.
//   - row_channel.go         — entity=channels (FK probe for
//     default_queue_code).
//   - row_adapter.go         — entity=adapters.
//   - row_break_reason.go    — entity=break_reasons.
//
// Handler wiring (Plan 05-06 — Wave 4):
//
//   - handler_import.go      — BulkImportCatalog + GetImportJob method
//     bodies on *Importer. H3 fix-up: finaliseJob failure returns 500
//     (not 200) so the audit-row + result wire don't diverge.
//
// Integration tests (Plan 05-07 — Wave 5):
//
//   - handlers_test.go, idempotency_test.go, handler_import_test.go,
//     row_*_test.go, sweep_test.go, jobs_test.go, header_test.go,
//     parser_csv_test.go, coerce_test.go, testutil_test.go, doc_test.go.
//   - testdata/*.{csv,json} fixtures.
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
//     with succeeded[] empty (v0.1 KNOWN LIMITATION). M5 fix-up: replay
//     status code mirrors original (200/207/422), not always 200.
//   - D5-21        — 50 MB body limit via http.MaxBytesReader middleware.
//   - D5-22        — 500-row hard cap.
//   - D5-23        — JSON parsing path accepts the strict-server's
//     eager-decoded []interface{} body and per-row re-marshals.
//   - D5-25..D5-27 — OpenAPI extensions (Import*Request schemas, ImportJob
//     status enum, Idempotency-Key header) live in openapi.yaml.
//
// # CSV-import PUT semantics — admin advisory
//
// Bulk-import via this package is PUT-style on top-level fields per
// (org_id, code): a column present in the payload OVERWRITES the
// stored value; a column omitted preserves the existing value at the
// SQL-driver level (sqlc emits *string for nullable types, and the
// Upsert*ByCode queries SET column=EXCLUDED.column). For row writes
// that come through here, an admin who imports a CSV missing the
// `external_id` column to fix a typo in `name` will NOT wipe
// external_id — but the column-LEVEL semantics are MERGE only because
// every column has a value supplied (the row processor either fills
// from typed.ExternalId or uses the default). The actual destructive
// risk surfaces when a column is documented but the admin's CSV
// header literally omits it (a different scenario from "missing
// row"). The skills-array MERGE exception is documented at D5-18.
//
// For partial-update semantics, use the dedicated PATCH endpoints
// (PATCH /v1/orgs/{org_id}/agents/{id}, etc) which honour the
// omitted-field convention via COALESCE in the catalog handlers.
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
