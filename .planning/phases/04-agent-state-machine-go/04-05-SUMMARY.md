---
phase: 04-agent-state-machine-go
plan: 05
subsystem: state-machine
tags: [wiring, composite, atomic-seed, api-handlers]
dependency_graph:
  requires: [04-01, 04-02, 04-03, 04-04]
  provides: [ApiHandlers composite, stateServer lifecycle, atomic CreateAgent seed]
  affects: [cmd/api/main.go, catalog/agents.go, catalog/notimpl.go, catalog/handlers.go, test/isolation]
tech_stack:
  added: []
  patterns: [D-89 composite embedding, D-93 atomic tx seed, D-95 synchronous startup sweep]
key_files:
  created: []
  modified:
    - services/api/cmd/api/main.go
    - services/api/internal/catalog/agents.go
    - services/api/internal/catalog/notimpl.go
    - services/api/internal/catalog/handlers.go
    - services/api/internal/catalog/testutil_test.go
    - services/api/test/isolation/main_test.go
decisions:
  - "D-89 ApiHandlers composite defined as local type inside run() — Go permits local types; composite merges disjoint method sets without rename"
  - "stateServer.Start(ctx) synchronous startup before http.Server — guarantees no stuck WrapUp survives restart (D-95)"
  - "defer stateServer.Stop() registered after successful Start — failed Start does not register defer on uninitialized Server"
  - "Isolation test wires stateServer.Start/Stop in TestMain — matches production lifecycle contract"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-17"
  tasks_completed: 3
  files_modified: 6
  tests_run: 379
---

# Phase 4 Plan 05: ApiHandlers Wire + Atomic Seed Summary

**One-liner:** Activated D-89 composite ApiHandlers in main.go (`*catalog.Handlers + *state.Server`) with synchronous stateServer.Start/Stop lifecycle, atomically seeded `agent_states` inside CreateAgent's OrgTx (D-93 Pitfall 10), and deleted dead state stubs from catalog/notimpl.go.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Seed initial agent_states row in CreateAgent tx | 8674b50 | catalog/agents.go |
| 2 | Delete state stubs + move compile-time assertion | 54d4c6e | catalog/notimpl.go, catalog/handlers.go |
| 3 | Wire ApiHandlers composite + stateServer in main.go | 615e4a6 | cmd/api/main.go, testutil_test.go, isolation/main_test.go |
| fix | stateServer.Start/Stop in isolation TestMain | e4ad85e | test/isolation/main_test.go |

## What Was Built

**Task 1 — Atomic agent_states seed (D-93):**
- Inserted `qtx.InsertAgentState(...)` inside the existing `OrgTx` block in `CreateAgent`, BETWEEN the `InsertAgent` call and the `replaceAgentSkills` block
- Uses `qtx` (the OrgTx-bound Queries) — NOT `generated.New(h.deps.OrgDB)` (Pitfall 10 avoided)
- Status seeded as `api.AgentStatusOffline`; `state_version` defaults to 1 via SQL `VALUES ($1, $2, $3, 1)` literal
- WHY-comment encodes Phase 5 inheritance: bulk-import MUST use `ON CONFLICT (agent_id) DO NOTHING` (Pitfall 6)
- Failure returns 500 `agent_state_insert_failed`; `defer tx.Rollback(ctx)` rolls back both agent and state rows atomically

**Task 2 — Cleanup catalog package (Hazard 2):**
- Deleted `GetAgentStatus` and `PatchAgentStatus` 501 stubs from `catalog/notimpl.go` (state.Server now owns them)
- Removed `var _ api.StrictServerInterface = (*Handlers)(nil)` from `catalog/handlers.go` (catalog alone no longer satisfies the full interface)
- Removed now-unused `api` import from `handlers.go`
- Preserved `BulkImportCatalog` and `GetImportJob` Phase 5 stubs untouched
- Both changes in ONE atomic commit (Hazard 2 — splitting would leave intermediate broken build)
- Build was intentionally broken between Task 2 and Task 3, as designed

**Task 3 — D-89 composite wiring in main.go:**
- Added `clockwork` and `state` imports
- Constructed `stateServer := state.New(state.Deps{...}, state.WithClock(clockwork.NewRealClock()))` after `catalogHandlers`
- `stateServer.Start(ctx)` runs synchronously before serving; failure returns 1 (aborts startup)
- `defer stateServer.Stop()` registered immediately after `Start` succeeds (LIFO shutdown order)
- Declared `type ApiHandlers struct { *catalog.Handlers; *state.Server }` as local type inside `run()`
- `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` compile-time assertion in main.go (moved from catalog/handlers.go)
- `StrictHandlers: apiHandlers` replaces `StrictHandlers: catalogHandlers`
- Updated `catalog/testutil_test.go` and `test/isolation/main_test.go` to use composite (Rule 1 auto-fix — vet failures)

**Peer Review Fix:**
- Codex LOW: `stateServer.Start(ctx)` + `stateServer.Stop()` added to isolation TestMain lifecycle (production contract compliance)

## Build Verification

```
cd services/api && go build ./...   # PASS
cd services/api && go vet ./...     # PASS
cd services/api && go test -count=1 ./...  # 379 passed
cd services/api && task gen && git diff --exit-code  # CODEGEN CLEAN
```

## Shutdown Order (T-04-14 mitigated)

LIFO via `defer` declarations in `run()`:
1. `http.Server.Shutdown` drains in-flight HTTP requests
2. `stateServer.Stop()` drains sweeper goroutine + cancels AfterFunc timers
3. `rdb.Close()` closes Redis client
4. `pool.Close()` closes Postgres pool
5. `shutdownOTel(ctx)` flushes spans

Sweeper UPDATEs complete before pool closes — T-04-14 mitigated.

## Deviations from Plan

**Auto-fixed (Rule 1 — Bug):** Task 3 triggered two `go vet` failures in test files:
- `catalog/testutil_test.go:140` — `h (*Handlers)` no longer satisfies `api.StrictServerInterface`
- `test/isolation/main_test.go:211` — `catalogHandlers (*catalog.Handlers)` no longer satisfies `api.StrictServerInterface`

Both fixed by introducing the same composite embedding pattern in the test files. `state.New` is wired with the same OrgDB + Cache instances already constructed in those test setups.

**Peer Review Fix (Codex LOW):** `stateServer.Start(ctx)` was missing from isolation TestMain — stateServer was constructed but the startup sweep (D-95) was never run and the sweeper goroutine never started. Fixed by adding `Start` before `m.Run()` and `Stop` in cleanup (matching production LIFO order).

## Pre-existing Issues Deferred (out of scope for 04-05)

Found in Wave 2-3 code (state/agent_states.go, state/ttl.go) during peer review. Logged to `deferred-items.md`:

- **CODEX HIGH:** `buildUpdateParams` accepts `break_reason_id` for non-Break transitions (persists in DB) — Wave 5 fix
- **CODEX MED:** Missing enum `.Valid()` check on `req.Body.To` and `PostInteractionState` — Wave 5 fix  
- **CODEX MED:** AfterFunc goroutines not tracked in WaitGroup — `Stop()` can return while DB work in-flight — Wave 5 fix
- **GEMINI HIGH:** Soft-deleted agents can have status read/modified (no `agents.enabled=true` join) — Wave 5 fix

These are correctness issues in pre-existing code. The scope boundary rule prevents auto-fixing them here; they are catalogued for Wave 5 (04-06 isolation + acceptance tests).

## Cross-AI Peer Review

- **Codex verdict:** BLOCK (pre-existing Wave 2-3 issues; LOW finding fixed in this plan)
- **Gemini verdict:** READY WITH FIXES (soft-delete gap in pre-existing Wave 2 code)
- **Outcome:** Codex BLOCK and Gemini HIGH concern both reference pre-existing code outside 04-05 scope. The LOW finding (stateServer lifecycle in isolation test) was fixed. Pre-existing HIGH/MED issues logged to deferred-items.md for Wave 5.

## Threat Model Coverage

| Threat | Status |
|--------|--------|
| T-04-13 (atomic agent_states INSERT) | MITIGATED — qtx.InsertAgentState inside existing OrgTx |
| T-04-14 (sweeper shutdown ordering) | MITIGATED — LIFO defer ordering; stateServer.Stop before pool.Close |

## Self-Check: PASSED

- [x] 04-05-SUMMARY.md exists
- [x] deferred-items.md exists
- [x] 8674b50 (Task 1) commit found
- [x] 54d4c6e (Task 2) commit found
- [x] 615e4a6 (Task 3) commit found
- [x] e4ad85e (peer review fix) commit found
- [x] `qtx.InsertAgentState` present in catalog/agents.go
- [x] State stubs deleted from catalog/notimpl.go
- [x] `type ApiHandlers struct` in cmd/api/main.go
- [x] `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` in main.go
- [x] `stateServer.Start(ctx)` in main.go
- [x] `defer stateServer.Stop()` in main.go
