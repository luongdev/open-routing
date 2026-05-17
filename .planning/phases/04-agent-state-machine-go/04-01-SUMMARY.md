---
phase: 04-agent-state-machine-go
plan: 01
subsystem: api
tags: [openapi, codegen, postgres, go, clockwork, sqlc, migration]

requires:
  - phase: 03-catalog-crud-go
    provides: "migration 000002 editable base, tenantTables SQLChecker, Phase 3 test suite, catalog package patterns"
  - phase: 02-openapi-contract-codegen
    provides: "oapi-codegen + openapi-typescript toolchain, codegen-drift CI gate (D-47/D-48)"
  - phase: 01-foundation-polyglot-monorepo
    provides: "SQLChecker framework, golang-migrate setup, OrgDB pattern"

provides:
  - "OpenAPI spec amended with optional force: boolean on PatchAgentStatusRequest (D-92)"
  - "agent_states table + 2 indexes in migration 000002 (D-78)"
  - "agent_states registered in SQLChecker tenantTables allowlist (Pitfall 4 mitigated)"
  - "Regenerated Go + TS codegen artifacts (server.gen.go, types.gen.go, spec.gen.go, generated.ts)"
  - "sqlc AgentState struct in models.go (from new table schema)"
  - "github.com/jonboulle/clockwork v0.4.0 pinned in go.mod"
  - "Migration 000002 applies cleanly via db:reset; Phase 3 test suite still green"

affects: [04-02, 04-03, 04-04, 04-05, 04-06, 05-bulk-import]

tech-stack:
  added:
    - "github.com/jonboulle/clockwork v0.4.0 — injectable Clock interface for WrapUp TTL goroutine deterministic testing"
  patterns:
    - "TEXT+CHECK constraint encoding for status enums (extends Phase 3 pattern, D-79)"
    - "No FK constraints from agent_states — app-layer probes only (D-80 extends D-76)"
    - "Denormalized org_id on agent_states for SQLChecker compliance (D-78)"
    - "Partial index WHERE status='WrapUp' for sweeper hot path (D-78)"

key-files:
  created:
    - "migrations/000002_catalog_v0_1.up.sql (agent_states block appended)"
    - "migrations/000002_catalog_v0_1.down.sql (DROP TABLE IF EXISTS agent_states prepended)"
  modified:
    - "openapi/openapi.yaml — force field added to PatchAgentStatusRequest (nullable+default removed per fix)"
    - "services/api/internal/api/types.gen.go — Force *bool omitempty on PatchAgentStatusJSONRequestBody"
    - "services/api/internal/api/server.gen.go — regenerated (embedded spec bytes)"
    - "services/api/internal/api/spec.gen.go — regenerated (embedded spec bytes refreshed)"
    - "web/packages/ui/src/api/generated.ts — force?: boolean (optional) on PatchAgentStatusRequest"
    - "services/api/internal/db/sqlcheck.go — agent_states added to tenantTables map"
    - "services/api/internal/db/generated/models.go — AgentState struct added by sqlc from new table"
    - "services/api/go.mod — clockwork v0.4.0 pinned as indirect dep"
    - "services/api/go.sum — clockwork hash entries added"

key-decisions:
  - "D-78: agent_states appended to migration 000002 (not new file per D-61); one row per agent; 9 columns; 2 indexes"
  - "D-79: TEXT+CHECK encoding for status enums — ALTER constraint not ALTER TYPE on PG14-"
  - "D-80: Zero FK constraints from agent_states — app-layer probes only"
  - "D-92: force field optional (no required entry, no default in spec) — openapi-typescript emits force?: boolean"
  - "Clockwork v0.4.0: AfterFunc semantics stabilized; FakeClock deterministic for Wave 3 tests; v1.x does not exist"
  - "[Fix] Removed nullable:true + default:false from force spec field — openapi-typescript was emitting force: boolean|null (required) instead of force?: boolean (optional), drifting from Go *bool omitempty"

patterns-established:
  - "Partial index pattern: WHERE predicate required for sweeper hot-path indexes"
  - "SQLChecker allowlist update required for every new table BEFORE Wave 1 sqlc generation (Pitfall 4)"
  - "clockwork injectable clock pattern for goroutine-based timer testing (Wave 3 will use FakeClock)"

requirements-completed:
  - STATE-01
  - STATE-05
  - STATE-06
  - STATE-07
  - STATE-08

duration: ~45min
completed: 2026-05-17
---

# Phase 4 Plan 01: Agent State Machine Wave 0 Summary

**OpenAPI force field + agent_states migration (9 columns, 2 indexes) + clockwork v0.4.0 pinned; codegen-drift gate green; Phase 3 suite still green**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-05-17T (prior agent Tasks 1-5) + continuation (Tasks 7-8)
- **Completed:** 2026-05-17
- **Tasks:** 8 (6 auto + 1 human checkpoint + 1 validation)
- **Files modified:** 9 source files + go.sum

## Accomplishments

- `agent_states` table with 9 columns and 2 indexes (including partial sweeper index) appended to migration 000002 inside existing BEGIN/COMMIT, applies cleanly via `task db:reset`
- OpenAPI spec amended with `force?: boolean` optional field on PatchAgentStatusRequest; Go `Force *bool` and TS `force?: boolean` aligned after peer-review auto-fix
- `github.com/jonboulle/clockwork v0.4.0` pinned post human legitimacy checkpoint (2861 importers, Apache-2.0, Kubernetes/etcd consumers confirmed)
- All 19 Phase 3 isolation tests green; SQLChecker tenantTables allowlist carries `agent_states` before Wave 1 sqlc generation

## Task Commits

Each task was committed atomically:

1. **Task 1: Amend openapi.yaml with force field** — `a3887c7` (feat)
2. **Task 2: Regenerate Go + TS codegen artifacts** — `6a0ee8e` (feat)
3. **Task 3: Append agent_states table + indexes to migration 000002** — `acc5d5f` (feat)
4. **Task 4: Prepend DROP TABLE IF EXISTS agent_states to down migration** — `634eef4` (feat)
5. **Task 5: Add agent_states to tenantTables allowlist** — `ce4d730` (feat)
6. **Task 6: Clockwork legitimacy verification** — human checkpoint (out-of-band, no commit)
7. **Task 7: go get clockwork@v0.4.0** — `81781bb` (feat)
8. **Task 8: Validation (db:reset + test + codegen drift gate)** — no source commit (validation only)
9. **Auto-fix: TS/Go force field alignment** — `610bceb` (fix — Codex peer review HIGH finding)

## Files Created/Modified

- `openapi/openapi.yaml` — force field added to PatchAgentStatusRequest (nullable+default removed to fix TS optional drift)
- `services/api/internal/api/types.gen.go` — Force *bool omitempty regenerated
- `services/api/internal/api/server.gen.go` — regenerated (spec bytes)
- `services/api/internal/api/spec.gen.go` — regenerated (embedded spec bytes refreshed twice: initial + fix)
- `web/packages/ui/src/api/generated.ts` — force?: boolean (optional, aligned with Go omitempty)
- `migrations/000002_catalog_v0_1.up.sql` — agent_states CREATE TABLE + 2 indexes appended before COMMIT
- `migrations/000002_catalog_v0_1.down.sql` — DROP TABLE IF EXISTS agent_states prepended before agent_skills drop
- `services/api/internal/db/sqlcheck.go` — "agent_states": {} added to tenantTables map
- `services/api/internal/db/generated/models.go` — AgentState struct generated by sqlc from new table
- `services/api/go.mod` — clockwork v0.4.0 pinned as indirect dep
- `services/api/go.sum` — clockwork hash entries

## Migration Block (committed in acc5d5f)

```sql
-- ---------------------------------------------------------------------------
-- STATE-01..STATE-10: agent_states (Phase 4 — D-78)
-- ---------------------------------------------------------------------------
-- One row per agent. PK = agent_id. NO FK (D-80 — app-layer probes only).
-- TEXT + CHECK encoding for status/engaged_channel/post_interaction_state
-- (D-79). Adding values = ALTER constraint, not ALTER TYPE on PG14-.
-- org_id is denormalized so SQLChecker (Phase 1 D-04) sees the column on
-- every sqlc-generated query; the sweeper goroutine bypasses ctx-injection
-- and relies on this column in the SQL WHERE clauses for org auditability.
CREATE TABLE agent_states (
    agent_id                UUID PRIMARY KEY,
    org_id                  UUID NOT NULL,
    status                  TEXT NOT NULL
                              CHECK (status IN ('Ready','NotReady','Break','Engaged','WrapUp','Offline')),
    engaged_channel         TEXT
                              CHECK (engaged_channel IS NULL OR engaged_channel IN ('voice','chat','email')),
    break_reason_id         UUID,
    post_interaction_state  TEXT
                              CHECK (post_interaction_state IS NULL OR post_interaction_state IN ('ready','not_ready')),
    wrapup_until            TIMESTAMPTZ,
    state_version           BIGINT NOT NULL DEFAULT 1,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_agent_states_org_status ON agent_states (org_id, status);

-- Partial index — sweeper hot path is `WHERE status='WrapUp' AND wrapup_until < NOW()`.
-- Partial index ensures the planner reads only the small subset of rows in WrapUp.
CREATE INDEX ix_agent_states_wrapup_until
    ON agent_states (wrapup_until)
    WHERE status = 'WrapUp';
```

## Task 8 Validation Outputs

### db:reset
```
task: [db:reset] ... DROP DATABASE ... CREATE DATABASE
1/u create_scaffold (7.772958ms)
2/u catalog_v0_1 (19.221583ms)
exit code: 0
```

### task test (full suite)
```
ok  github.com/luongdev/open-routing/services/api/internal/cache       3.235s
ok  github.com/luongdev/open-routing/services/api/internal/catalog     13.561s
ok  github.com/luongdev/open-routing/services/api/internal/db          2.099s
ok  github.com/luongdev/open-routing/services/api/internal/db/orgkey   2.515s
ok  github.com/luongdev/open-routing/services/api/internal/middleware  4.050s
ok  github.com/luongdev/open-routing/services/api/internal/server      3.609s
ok  github.com/luongdev/open-routing/services/api/internal/telemetry   4.571s
ok  github.com/luongdev/open-routing/services/api/test/isolation       5.947s
All packages PASS — zero regressions (19 isolation tests green)
```
macOS linker warnings (`malformed LC_DYSYMTAB`) are benign toolchain noise, not test failures.

### Codegen Drift Gate
```
task gen (sqlc + oapi-codegen + openapi-typescript) → idempotent
git diff --exit-code services/api/internal/api/ services/api/internal/db/generated/models.go \
  web/packages/ui/src/api/generated.ts openapi/openapi.yaml
exit code: 0
```

### agent_states Table Verification (via docker exec psql)
```
                         Table "public.agent_states"
         Column         |           Type           | Nullable | Default
------------------------+--------------------------+----------+---------
 agent_id               | uuid                     | not null |
 org_id                 | uuid                     | not null |
 status                 | text                     | not null |
 engaged_channel        | text                     |          |
 break_reason_id        | uuid                     |          |
 post_interaction_state | text                     |          |
 wrapup_until           | timestamp with time zone |          |
 state_version          | bigint                   | not null | 1
 updated_at             | timestamp with time zone | not null | now()
Indexes:
    "agent_states_pkey" PRIMARY KEY, btree (agent_id)
    "ix_agent_states_org_status" btree (org_id, status)
    "ix_agent_states_wrapup_until" btree (wrapup_until) WHERE status = 'WrapUp'::text
Check constraints:
    "agent_states_engaged_channel_check"
    "agent_states_post_interaction_state_check"
    "agent_states_status_check"
```

9 columns confirmed. Both custom indexes present including partial index with WHERE predicate.

### Clockwork Version
```
go list -m github.com/jonboulle/clockwork
github.com/jonboulle/clockwork v0.4.0
```

## Decisions Made

- **force field spec**: Removed `nullable: true` and `default: false` from openapi.yaml — openapi-typescript v7 interprets `nullable:true` as required `T|null`, and `default:false` as non-optional. The spec-driven approach to optional is to simply omit from `required:` array (already done). Server handles nil pointer as false.
- **clockwork as indirect**: `go mod tidy` would prune clockwork since no source file imports it yet. Left without tidy to preserve the explicit pin. Wave 1 sqlc + Wave 3 state.Handlers will promote it to direct import.
- **models.go inclusion**: sqlc generated `AgentState` struct during Task 8 `task gen` validation. Committed alongside spec fix since it's a correct generated artifact from the new table.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TS/Go type mismatch on force field — Codex peer review HIGH finding**
- **Found during:** Cross-AI peer review (post Task 8, CLAUDE.md HARD RULE)
- **Issue:** `openapi-typescript` generated `force: boolean | null` (required) when spec had `nullable: true`. Go generated `Force *bool json:"force,omitempty"` (optional). Wire contract drift: TS clients forced to send `force` on every PATCH.
- **Fix:** Removed `nullable: true` and `default: false` from force property in openapi.yaml; regenerated all artifacts. TS now emits `force?: boolean` (optional, matches Go omitempty semantics).
- **Files modified:** openapi/openapi.yaml, spec.gen.go, generated.ts, models.go
- **Verification:** Drift gate passes; `force?: boolean` confirmed in generated.ts
- **Committed in:** `610bceb` (fix commit)

**2. [Rule 3 - Blocking] go mod tidy removed clockwork after install**
- **Found during:** Task 7
- **Issue:** `go mod tidy` strips modules with no source importers. Running tidy after `go get` removed the clockwork pin.
- **Fix:** Re-ran `go get` without subsequent tidy. Left as `// indirect` pin — Wave 1 + Wave 3 imports will promote it.
- **Files modified:** go.mod, go.sum
- **Verification:** `go list -m github.com/jonboulle/clockwork` returns `v0.4.0`
- **Committed in:** `81781bb`

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both fixes required for correctness. The force field fix resolves a real TS client contract issue caught by mandatory peer review. The tidy deviation is expected behavior for pre-import module pins.

## Issues Encountered

- `psql` not in PATH on this machine — used `docker exec` to query PostgreSQL directly. `\di agent_states*` pattern only matched the PK index (glob doesn't match `ix_` prefix); confirmed both custom indexes via `pg_indexes` table query.
- Codex peer review initially received full diff including planning PLAN.md files (not just Wave 0 source changes). Codex correctly identified the HIGH TS/Go drift concern despite the large diff context.

## Cross-AI Peer Review Results

**Gemini verdict:** READY (LOW concerns only — PATCH optimistic locking deviation per D-85 acknowledged as valid trade-off; status casing mapping noted as hidden dependency for Wave 2 ExpireWrapUp query)

**Codex verdict:** READY WITH FIXES — HIGH: `force: boolean | null` (TS required) vs `Force *bool` (Go optional) → auto-fixed in `610bceb`

## Pitfall Status

- **Pitfall 4 (tenantTables panic):** ELIMINATED — `agent_states` in allowlist before Wave 1 sqlc generation
- **Pitfall 6 (CreateAgent must seed agent_states):** OUT OF SCOPE for Wave 0 — tracked for Wave 4 (04-05), which modifies `catalog/agents.go` CreateAgent to INSERT initial `agent_states` row inside existing OrgTx (D-93)

## Phase 5 Inheritance Reminder

Any new agent INSERT path (Phase 5 bulk import CSV/JSON) MUST also insert an `agent_states` row with `status='Offline'`, `state_version=1`, all nullable columns NULL. Failure to do so means `GET /agents/{id}/status` returns 404 for bulk-imported agents. This is Pitfall 6 per D-93 — captured in 04-PATTERNS.md.

## Next Phase Readiness

Wave 1 (04-02) is unblocked:
- Migration 000002 carries agent_states table — sqlc can generate queries against real schema
- tenantTables allowlist has agent_states — no ErrSQLMissingOrgFilter panic
- Regenerated types.gen.go carries `Force *bool` — state handler can parse force flag
- clockwork pinned — Wave 3 can import it immediately

Known blockers for Wave 1: none.

---
*Phase: 04-agent-state-machine-go*
*Completed: 2026-05-17*
