---
phase: 03-catalog-crud-go
plan: 02
subsystem: database
tags: [postgresql, golang-migrate, sqlc, schema, indexes, multi-org, migrations, task]

requires:
  - phase: 01-foundation-polyglot-monorepo
    provides: "migrations/000001_create_scaffold.up.sql baseline, golang-migrate locked (D-22), Taskfile orchestrator with $DATABASE_URL env"
  - phase: 03-catalog-crud-go (plan 01)
    provides: "Scaffold + 501-stub deletion (D-77), Phase-3 branch baseline"

provides:
  - "migrations/000002_catalog_v0_1.up.sql — single editable migration carrying all 7 catalog tables (agents, skills, queues, channels, adapters, break_reasons, agent_skills) plus 14 indexes"
  - "migrations/000002_catalog_v0_1.down.sql — drops all 7 tables and RECREATES _scaffold so `migrate down 1` lands on the Phase 1 baseline"
  - "Taskfile.yml `db:reset` task — drop+recreate dev database and re-apply all migrations from scratch (editable-migration dev workflow per D-61)"
  - "Universal version column (INTEGER NOT NULL DEFAULT 1) on every entity for CAT-08 optimistic locking — including channels and adapters per OQ-1/A4"
  - "Denormalized agent_skills.org_id satisfying H3 orgDB SQLChecker"
  - "agent_skills.proficiency CHECK BETWEEN 1 AND 10 — defense-in-depth backstop for CAT-03"

affects:
  - "03-03 (sqlc bindings) — reads this schema to emit row + param types"
  - "03-04 (cache pkg) — independent but Wave 1 sibling"
  - "03-05 (catalog skeleton) — depends on sqlc types from 03-03"
  - "03-06..03-09 (per-entity handlers) — depend on schema + sqlc types"
  - "Phase 4 (agent state) — will EDIT this migration in-place to add agent_states columns + state-transition CHECK"
  - "Phase 5 (bulk import) — will EDIT this migration in-place to add import_jobs"

tech-stack:
  added: []
  patterns:
    - "Composite DESC index (org_id, created_at DESC, id DESC) for cursor pagination"
    - "Partial functional index (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE for case-insensitive prefix name search"
    - "BEGIN/COMMIT-wrapped DDL migrations for atomic apply"
    - "Denormalized org_id on join tables to satisfy app-layer orgDB SQLChecker"
    - "Universal columns (id, org_id, enabled, version, created_at, updated_at) on every catalog entity"
    - "Migrate-down recreates Phase 1 baseline so rollback is symmetric"

key-files:
  created:
    - "migrations/000002_catalog_v0_1.up.sql"
    - "migrations/000002_catalog_v0_1.down.sql"
  modified:
    - "Taskfile.yml"

key-decisions:
  - "Single editable migration ships all 7 catalog tables (D-61)"
  - "Zero cross-entity FK constraints in v0.1 — app-layer validation only (D-76)"
  - "Universal version column on every entity (CAT-08, OQ-1/A4)"
  - "agent_skills carries denormalized org_id to satisfy orgDB SQLChecker (H3)"
  - "Migrate-down recreates _scaffold (RESEARCH Common Operation 3) — symmetric rollback"
  - "db:reset uses docker compose exec psql to drop+create the database, then migrate up (no host psql required)"

patterns-established:
  - "Cursor pagination index: (org_id, created_at DESC, id DESC) — every list query uses this"
  - "Soft-delete + prefix search index: partial functional (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE — substring '%foo%' searches still seq-scan (Pitfall 7); pg_trgm GIN deferred to v0.2"
  - "Optimistic-locking column: version INTEGER NOT NULL DEFAULT 1 with UPDATE WHERE version = $expected pattern (D-66)"
  - "Soft-delete column: enabled BOOLEAN NOT NULL DEFAULT TRUE filtered by default in List queries (D-65)"

requirements-completed:
  - CAT-01
  - CAT-02
  - CAT-03
  - CAT-04
  - CAT-05
  - CAT-06
  - CAT-07
  - CAT-08
  - CAT-09
  - CAT-10

duration: 4min
completed: 2026-05-16
---

# Phase 03 Plan 02: Catalog v0.1 Schema Migration Summary

**Single editable migration carrying 7 catalog tables (agents, skills, queues, channels, adapters, break_reasons, agent_skills) with 14 indexes, universal version column, denormalized agent_skills.org_id, and proficiency CHECK; matching down.sql recreates _scaffold; Taskfile gains `db:reset`.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-05-16T14:02:11Z
- **Completed:** 2026-05-16T14:06:13Z
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- 7-table catalog schema lands in one BEGIN/COMMIT-wrapped editable migration (D-61) — Phases 4 and 5 will edit this file in-place during v0.1.
- 14 indexes cover (org_id, created_at DESC, id DESC) cursor pagination on every entity, plus the partial functional (org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE for case-insensitive prefix name search.
- Universal optimistic-locking column `version INTEGER NOT NULL DEFAULT 1` is present on every entity — including channels and adapters per OQ-1/A4.
- `agent_skills.org_id` is denormalized so the orgDB SQLChecker (Hazard H3) accepts every sqlc-generated agent_skills query in Plan 03-03.
- `agent_skills.proficiency CHECK BETWEEN 1 AND 10` provides defense-in-depth for CAT-03 (handler validates at 422 first; CHECK guards admin shell / future bulk-import paths).
- Zero `REFERENCES` clauses (verified by acceptance grep) — D-76 mandates app-layer cross-row validation only in v0.1.
- `migrations/000002_catalog_v0_1.down.sql` drops the 7 catalog tables and recreates `_scaffold` with the schema from `000001_create_scaffold.up.sql`, so `migrate down 1` from a fully-applied state lands on the Phase 1 baseline.
- `task db:reset` shipped to Taskfile.yml inserted after `migrate-create:`; uses `docker compose exec -T postgres psql` so dev environments without host psql still work.

## Task Commits

Each task was committed atomically:

1. **Task 1: Author migrations/000002_catalog_v0_1.up.sql** — `70c56fa` (feat)
2. **Task 2: Author migrations/000002_catalog_v0_1.down.sql** — `af7391a` (feat)
3. **Task 3: Add `task db:reset` to Taskfile.yml** — `3d77fd6` (feat)

## Files Created/Modified

- `migrations/000002_catalog_v0_1.up.sql` (created, 179 lines) — 7 catalog tables, 14 indexes, DROP _scaffold, universal version column, agent_skills CHECK + denormalized org_id, zero FKs, BEGIN/COMMIT wrap
- `migrations/000002_catalog_v0_1.down.sql` (created, 35 lines) — 7 DROP TABLE IF EXISTS in reverse-dependency order, recreates _scaffold + idx_scaffold_org_id, BEGIN/COMMIT wrap
- `Taskfile.yml` (modified, +6 lines) — new `db:reset:` target inserted after `migrate-create:`

## Acceptance Criteria Verification

| Criterion | Expected | Actual | Status |
|-----------|----------|--------|--------|
| up.sql: CREATE TABLE catalog tables | 7 | 7 | PASS |
| up.sql: CREATE INDEX ix_* count | ≥ 12 | 14 | PASS |
| up.sql: DROP TABLE IF EXISTS _scaffold | 1 hit | 1 hit | PASS |
| up.sql: universal version columns | ≥ 7 | 7 | PASS |
| up.sql: agent_skills proficiency CHECK | 1 hit | 1 hit | PASS |
| up.sql: denormalized org_id rationale | ≥ 1 hit | 1 hit | PASS |
| up.sql: BEGIN; line | 1 hit | 1 hit | PASS |
| up.sql: REFERENCES clauses (D-76) | 0 hits | 0 hits | PASS |
| up.sql: line count | ≥ 90 | 179 | PASS |
| down.sql: DROP TABLE IF EXISTS count | ≥ 7 | 7 | PASS |
| down.sql: CREATE TABLE _scaffold | 1 hit | 1 hit | PASS |
| down.sql: BEGIN; / COMMIT; lines | 2 | 2 | PASS |
| Taskfile: db:reset target | present | present | PASS |
| Taskfile: migrate -path migrations -database | present | present | PASS |

## Smoke Test (deferred — environmental)

Plan acceptance criteria 3.2-3.4 require running `docker compose up -d postgres && task db:reset` on a live container and verifying via `\dt` that 7 catalog tables appear and `_scaffold` is gone. **This smoke could not run in the agent's worktree environment** — the OrbStack-backed Docker daemon is not connected (`Cannot connect to the Docker daemon at unix:///Users/luong/.orbstack/run/docker.sock`).

What we did instead:
- `task --list` parses the Taskfile cleanly and lists `db:reset` with the expected description.
- The recipe mirrors the proven `migrate-up` pattern (same `migrate -path migrations -database "$DATABASE_URL" up` invocation) and uses `docker compose exec -T postgres psql` to drop+create the database without requiring host psql.
- SQL was hand-verified against `migrations/000001_create_scaffold.up.sql` and Phase 1's Postgres 17 baseline.

**Action for the verifier:** start the postgres container locally (`docker compose up -d postgres`) and run `task db:reset` to confirm the schema applies end-to-end. Expected outcome: `migrate up` exits 0, `\dt` lists `agents, skills, queues, channels, adapters, break_reasons, agent_skills` (7 rows), and `_scaffold` is absent.

## Decisions Made

None - followed plan as specified. The schema, index list, and Taskfile structure all came verbatim from the `<interfaces>` block; the only judgment call was structuring the `db:reset` `psql` command on a single line so the plan's `grep -A 5 "db:reset:"` acceptance pattern matches (a multi-line `psql` block pushed the migrate command past the 5-line window without changing behavior).

## Deviations from Plan

None - plan executed exactly as written.

The only minor structural choice (single-line `psql` invocation) was made to keep the plan's own acceptance grep (`grep -A 5 "db:reset:" | grep -F "migrate -path migrations -database"`) passing. The recipe is functionally identical to the multi-line form in the plan's `<interfaces>` block.

## Issues Encountered

- **Docker daemon unavailable in worktree environment** — prevented live smoke test. Resolved by validating Taskfile parses (`task --list`) and the recipe structure matches the proven `migrate-up` pattern. Verifier will exercise the live path with Docker running.

## Next Phase Readiness

- **Plan 03-03 (sqlc bindings)** can now `sqlc generate` against the schema; row + param types will emit for all 7 catalog entities.
- **Plan 03-04 (cache pkg)** is independent and ran in parallel — no coupling.
- **Plan 03-05+ (catalog handlers)** depend on sqlc-generated types from 03-03 and can begin once that completes.
- **Phase 4 (agent state)** will edit `000002_catalog_v0_1.up.sql` in-place to add `agent_states` columns + state-transition CHECK during v0.1; the editable-migration discipline (D-61) is now established by this plan.
- **Phase 5 (bulk import)** will edit `000002_catalog_v0_1.up.sql` in-place to add `import_jobs` during v0.1.

## Threat Surface Notes

Mitigations for the plan's `<threat_model>` register are all in place:

- **T-3-05 (Information Disclosure via missing denormalized org_id):** mitigated — `agent_skills.org_id UUID NOT NULL` ships with rationale comment.
- **T-3-06 (Tampering via inconsistent migrate-down):** mitigated — `down.sql` recreates `_scaffold` and `task db:reset` provides the safety net.
- **T-3-07 (DoS via adapters.config JSONB unbounded):** accepted — `config JSONB NOT NULL DEFAULT '{}'::jsonb` ships as designed; production hardening deferred.
- **T-3-08 (EoP via cross-entity FKs):** mitigated by design — acceptance grep confirms 0 `REFERENCES` clauses.

No new threat surface introduced beyond the plan's register.

## Self-Check: PASSED

- `migrations/000002_catalog_v0_1.up.sql` exists (179 lines).
- `migrations/000002_catalog_v0_1.down.sql` exists (35 lines).
- `Taskfile.yml` modified (db:reset target added at line 43).
- Commit `70c56fa` (Task 1 up.sql) found in `git log`.
- Commit `af7391a` (Task 2 down.sql) found in `git log`.
- Commit `3d77fd6` (Task 3 Taskfile.yml) found in `git log`.

---
*Phase: 03-catalog-crud-go, Plan: 02*
*Completed: 2026-05-16*
