---
gsd_state_version: 1.0
milestone: v0.1
milestone_name: Catalog Foundation
status: executing
stopped_at: Phase 04.1 context gathered
last_updated: "2026-05-17T18:13:43.055Z"
last_activity: 2026-05-17 -- Phase 06 planning complete
progress:
  total_phases: 8
  completed_phases: 6
  total_plans: 61
  completed_plans: 49
  percent: 75
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-16)

**Core value:** Product teams can define, simulate, debug, publish, and embed powerful routing flows quickly without Open Routing becoming a media platform, agent desktop, CRM, or ticketing system.
**Current focus:** Phase 06 — Shared UI Library & Standalone Admin (next)

## Current Position

Phase: 04.1 + 05 — SHIPPED (merged to main 2026-05-17 as save point)
Status: Ready to execute
Last activity: 2026-05-17 -- Phase 06 planning complete

Progress: [████████░░] 88%

## Phase 04.1 + 05 Ship Notes (2026-05-17)

- Phase 04.1 (catalog identity normalization): 6 plans, all merged inside chain. Introduced universal `code` field + demoted `external_id` to optional. Migration 000002 amended in place (Phase 4 baseline still editable per D-61).
- Phase 5 (bulk-import-go): 8 plans + 1 FIXUP plan (05-09). 705 tests pass with race detector. `task gen` clean, `task lint` clean, `go vet` clean.
- Combined cross-AI peer review on Plans 04.1 + 05 returned BLOCK with 12 findings (3 HIGH + 5 MED + 4 LOW). Plan 05-09 FIXUP addressed all 12 in 11 atomic commits (~55min).
- Re-run UAT (Gemini + Codex parallel): Gemini PASS WITH NOTES (highlight oneOf 409 contract in client release notes); Codex confirmed runtime tests green; one LOW advisory about `oneOf` discriminator (deferred to v0.2).
- Merge: `git branch -f main HEAD` (104 commits FF'd into local main as save point; no push to origin).

## Performance Metrics

**Velocity:**

- Total phase plans completed: 17
- Additional hardening/ship summaries: 1
- Average duration: —
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 11 | - | - |
| 02 | 6 (+1 hardening) | - | - |

**Recent Trend:**

- Last 5 plans: —
- Trend: —

| Phase 01-foundation-polyglot-monorepo P09 | 2 | 4 tasks | 4 files |
| Phase 04 P06 | 180 | 9 tasks | 13 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table. Key decisions affecting all phases (locked stack, v0.1):

- Backend: Go (latest) + chi + sqlc + pgx + golang-migrate + slog (not Node.js/Fastify/Drizzle)
- Database: PostgreSQL 17 + Redis (cache layer for hot-path reads)
- API contract: OpenAPI 3.0 as stored source-of-truth (downgraded from 3.1 for oapi-codegen v2 compatibility) → oapi-codegen (Go server stubs) + openapi-typescript (TS client); contract-first to prevent drift
- Frontend: Vite + Lit + Shoelace + TypeScript (not React, not shadcn/ui, not Module Federation)
- Embedding: Web Components (Custom Elements + Shadow DOM) — not iframe, not Module Federation
- Repo: polyglot monorepo (Go module + pnpm workspaces side-by-side, Makefile or Turborepo for orchestration)
- Org isolation: app-layer `orgDB` wrapper around pgx.Pool is primary defense; PostgreSQL RLS deferred as optional operator hardening
- `org_id` naming locked (not `tenant_id`); applies to schema, APIs, UI, and docs
- REST polling only in v0.1 (no WebSocket/SSE); push channels arrive with the runtime engine
- Browser workers deferred to v0.2 (no offline, no push notification dedup, no client-side CSV preview)
- No XState in v0.1; pure Go transition table in `services/api/internal/domain`
- OpenAPI contract scaffold in Phase 2, before any endpoint implementation in Phase 3
- [Phase 04]: agentEnabledCheck at app layer; TOCTOU race deferred to Phase 5 AUTH phase
- [Phase 04]: COALESCE engaged_channel (preserve system-set) + explicit PIS assignment (nil clears stale) enforces STATE-05/STATE-06
- [Phase 04]: per-org DELETE replaces global TRUNCATE in cleanStateTables for parallel test isolation

### Pending Todos

- Phase 3 simplification cleanup: see `.planning/phases/03-catalog-crud-go/03-SIMPLICITY-REVIEW.md`. Fix request-id injection, transitional stubs, overly generic cache API, entity-specific DB conflict mapping, and plan-generated code bloat before Phase 3 grows more handlers.

### Blockers/Concerns

From research/SUMMARY.md gaps to address during implementation:

- WrapUp TTL with multi-replica (Phase 4): Go goroutine scheduler works single-process; flag before v1 multi-replica (`setInterval` analog — needs distributed lock or single-writer pattern).
- `postInteractionState` default configurability (Phase 4): product decision needed — can orgs configure the default to `not_ready`?
- Phase 7 host integration test harness: Playwright + React/Vue/HTML stub shell must be set up at Phase 7 start, before embed component work begins.
- Import schema version communication: `?schema_version=v0.1` is specified; customer notification mechanism for v0.2 schema changes is not defined yet.

## Deferred Items

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* |      |        |             |

## Session Continuity

Last session: 2026-05-17T06:46:39.058Z
Stopped at: Phase 04.1 context gathered
Resume file: .planning/phases/04.1-catalog-identity-normalization/04.1-CONTEXT.md

## Phase 3 — Decision Coverage Override (2026-05-16)

7 CONTEXT decisions (D-56, D-68, D-70, D-72, D-73, D-74, D-77) implement-but-do-not-cite in plan `must_haves.truths`. They ARE reflected in plan narrative + acceptance criteria + task bodies. Override recorded so verify-phase resurfaces them.

- D-56 (409 cache DEL): Implemented in 03-06 + 03-07/08/09 handlers (`cache.Del` after 409 disambiguation).
- D-68 (single catalog pkg, one file per entity): Implemented by 03-05 skeleton + 03-06/07/08/09 entity files.
- D-70 (forward-compat ApiHandlers embed): Implemented in 03-10 main.go wiring.
- D-72 (per-entity _test.go beside source): Implemented by 03-06/07/08/09 test files.
- D-73 (testutil_test.go shared setup): Implemented by 03-05 + 03-06 after iter 3 rename.
- D-74 (two-layer validation): Implemented across 03-06/07/08/09 handler tests (Layer 1 + Layer 2).
- D-77 (scaffold deletion in commit 1): Implemented by 03-01 Wave 0.
