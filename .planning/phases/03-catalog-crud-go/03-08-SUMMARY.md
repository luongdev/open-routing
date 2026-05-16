---
phase: 03-catalog-crud-go
plan: 08
subsystem: api
tags: [go, sqlc, pgx, postgres, redis, catalog, crud, validation, fk-probe, d-76]

requires:
  - phase: 03-catalog-crud-go
    provides: "agents.go canonical template; cache + cursor + errors + mappers helpers; sqlc-generated queues.sql.go and channels.sql.go with QueueExistsAndEnabledInOrg probe"
provides:
  - "queues.go — 5 real CRUD handlers + channel_types[] round-trip + priority/acw_sec integer round-trip"
  - "channels.go — 5 real CRUD handlers + D-76 cross-row FK probe via QueueExistsAndEnabledInOrg before Insert/Update"
  - "queues_test.go — 13 end-to-end tests covering CAT-04/08/09/10/11"
  - "channels_test.go — 21 end-to-end tests covering CAT-05/08/09/10/11 plus four D-76 paths (happy / unknown / disabled / cross-org)"
  - "seedQueueForOrg test helper in testutil_test.go enabling cross-org D-76 probes"
affects: [03-09-PLAN, 03-10-PLAN]

tech-stack:
  added: []
  patterns:
    - "D-76 cross-row FK probe — QueueExistsAndEnabledInOrg pre-write check returning 422 invalid_reference on miss; identical response shape for missing/disabled/cross-org per FOUND-08"
    - "Sparse-PATCH nullable conversion — pgtype.UUID{Valid:false} when *UUIDv7 is nil so InsertChannel writes SQL NULL via pgx"
    - "channel_types[] round-trip — []api.ChannelType ↔ []string conversion at the handler boundary"

key-files:
  created:
    - services/api/internal/catalog/queues_test.go
    - services/api/internal/catalog/channels_test.go
  modified:
    - services/api/internal/catalog/queues.go
    - services/api/internal/catalog/channels.go
    - services/api/internal/catalog/testutil_test.go

key-decisions:
  - "Queues handlers skip BeginTx — no join table to update atomically; only UpdateQueue's version-checked UPDATE retains tx-free atomicity via D-66"
  - "CreateQueue path has 409 mapping but NOT 422 (queue schema has no FKs/CHECKs reachable by valid input); a 23503/23514 pg error here is genuine schema drift → 500"
  - "Channels probe → write hazard window (queue soft-deleted between probe and INSERT) is documented as accepted v0.1 risk (D-76); UI surface reads the channel and presents the integrity gap to admins"
  - "Cross-org D-76 probe response shape is identical to missing/disabled — FOUND-08 forbids any oracle that distinguishes 'queue exists in another org' from 'queue does not exist'"
  - "seedQueueForOrg helper bypasses the queues HTTP handler so cross-org D-76 tests can seed in arbitrary orgs without re-entering the org-scoped middleware chain"

patterns-established:
  - "FK probe template: probe BEFORE write, ErrNoRows → 422 invalid_reference, other err → 500 queue_probe_failed. Plan 03-09's agent_skills replace + Phase 4's break_reason_id validation reuse this exact shape."
  - "Nullable UUID handling: *UUIDv7 → pgtype.UUID with Valid=false on nil; mapChannel reads row.DefaultQueueID.Valid for the reverse conversion."
  - "TEXT[] round-trip: cast []api.ChannelType → []string for INSERT params; cast []string → []api.ChannelType in mapQueue."
  - "Two-template family: queues = simplified agents.go (no BeginTx); channels = simplified agents.go + D-76 probe block before Insert/Update."

requirements-completed: [CAT-04, CAT-05, CAT-08, CAT-09, CAT-10, CAT-11]

duration: 17min
completed: 2026-05-16
---

# Phase 3 Plan 08: Queues + Channels CRUD + D-76 Cross-Row FK Probe Summary

**Queues handlers ship the channel_types[]+priority+acw_sec round-trip; channels handlers ship the D-76 QueueExistsAndEnabledInOrg cross-row probe that returns 422 invalid_reference identically for missing/disabled/cross-org queues — the canonical FK pattern Plan 03-09 + Phase 4 reuse.**

## Performance

- **Duration:** ~17 min
- **Started:** 2026-05-16T15:40:00Z
- **Completed:** 2026-05-16T15:57:28Z
- **Tasks:** 2
- **Files modified:** 3 (+ 2 created)

## Accomplishments

- queues.go ships 5 real handlers (Create/Get/List/Update/Delete) + mapQueue covering CAT-04 fields (channel_types TEXT[], priority, acw_sec) with full cache + cursor + D-66 disambiguation.
- channels.go ships 5 real handlers + the D-76 cross-row FK probe pattern. Probe runs BEFORE InsertChannel/UpdateChannel; missing/disabled/cross-org queue all surface as identical 422 invalid_reference (FOUND-08 — no existence oracle).
- queues_test.go provides 13 end-to-end tests (cache, cursor, name search, soft-delete + include_disabled, version conflict, channel_types round-trip, priority/acw_sec round-trip, cross-org GET 404, limit boundary).
- channels_test.go provides 21 end-to-end tests including the four critical D-76 paths (happy / not-found / disabled / cross-org) and the plan-required aliases.
- 38 plan tests under `go test -race` pass (TestQueues_* + TestChannels_* counts including subtests); 64 tests total in the catalog package green after this plan.

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement queues.go + queues_test.go (CAT-04: channel_types + priority + acw_sec)** — `d1c2bf5` (feat)
2. **Task 2: Implement channels.go + channels_test.go (CAT-05: D-76 cross-row FK validation)** — `4e367c5` (feat)

**Plan metadata:** _commit follows after SUMMARY.md is staged_

## Files Created/Modified

- `services/api/internal/catalog/queues.go` — Replaced placeholder bodies with real CAT-04 handlers; mapQueue, cache.GetOrSet[api.Queue], D-66 disambiguation, channel_types[] cast.
- `services/api/internal/catalog/queues_test.go` — 13 end-to-end tests via httptest server + miniredis.
- `services/api/internal/catalog/channels.go` — Replaced placeholder bodies with real CAT-05 handlers; D-76 probe before Insert/Update; mapChannel handles nullable default_queue_id.
- `services/api/internal/catalog/channels_test.go` — 21 end-to-end tests, four of which are the canonical D-76 proofs (happy / not-found / disabled / cross-org).
- `services/api/internal/catalog/testutil_test.go` — Added `seedQueueForOrg(orgID)` helper so cross-org tests can seed queues in arbitrary orgs.

## Test Coverage Matrix

### Queues (CAT-04 + CAT-08 + CAT-09 + CAT-10 + CAT-11)

| Test | CAT Coverage |
|------|--------------|
| TestQueues_CreateThenGet | CAT-04, CAT-11 (cache fill) |
| TestQueues_GetMissing | CAT-04 (404) |
| TestQueues_VersionConflict | CAT-08 (D-66 disambiguation) |
| TestQueues_SoftDelete | CAT-09 (D-65 idempotent-disabled) |
| TestQueues_SoftDelete_IncludeDisabled | CAT-09 (include_disabled toggle) |
| TestQueues_Cursor | CAT-10 (60-row pagination, N+1 sentinel) |
| TestQueues_NameSearch | CAT-10 (ILIKE %name%) |
| TestQueues_CacheInvalidationOnUpdate | CAT-11 + D-55 |
| TestQueues_CacheInvalidationOnDelete | CAT-11 + D-55 |
| TestQueues_ChannelTypesRoundTrip | CAT-04 (TEXT[] semantics) |
| TestQueues_PriorityAcwSec | CAT-04 (integer round-trip) |
| TestQueues_CrossOrgGet404 | FOUND-08 inline canary |
| TestQueues_LimitOutOfRange | CAT-10 (400 invalid_body boundary) |

### Channels (CAT-05 + CAT-08 + CAT-09 + CAT-10 + CAT-11 + D-76)

| Test | CAT / Decision Coverage |
|------|-------------------------|
| TestChannels_CreateThenGet | CAT-05, CAT-11 |
| TestChannels_GetMissing | CAT-05 |
| TestChannels_VersionConflict | CAT-08 |
| TestChannels_SoftDelete | CAT-09 |
| TestChannels_SoftDelete_IncludeDisabled | CAT-09 |
| TestChannels_Cursor | CAT-10 |
| TestChannels_NameSearch | CAT-10 |
| TestChannels_CacheInvalidationOnUpdate | CAT-11 + D-55 |
| TestChannels_CacheInvalidationOnDelete | CAT-11 + D-55 |
| TestChannels_DefaultQueueId_HappyPath | D-76 happy (queue in caller's org, enabled=true) |
| TestChannels_DefaultQueueId_NotFound | D-76 not-found (422 invalid_reference) |
| TestChannels_Create_UnknownQueueId_422 | D-76 plan alias |
| TestChannels_DefaultQueueId_Disabled | D-76 disabled (soft-deleted queue → 422) |
| TestChannels_DefaultQueueId_CrossOrg | D-76 + FOUND-08 (queue in other org → 422, identical reason) |
| TestChannels_CrossOrgQueueReject_422 | D-76 + FOUND-08 plan alias |
| TestChannels_UpdateChannelInvalidReference | D-76 on PATCH path (422 + version unchanged) |
| TestChannels_Update_QueueDisabledInSameOrg_422 | D-76 on PATCH with disabled-in-same-org queue (alias) |
| TestChannels_CrossOrgGet404 | FOUND-08 GET probe |
| TestChannels_ChannelTypeRoundTrip | CAT-05 (channel_type enum) |
| TestChannels_LimitOutOfRange | CAT-10 boundary |
| TestChannels_InvalidReference | D-76 must_have alias |

## FOUND-08 Verification

The cross-org D-76 test proves the same response shape as the not-found case — no existence oracle:

```
{ "error": "invalid_reference", "reason": "default_queue_id_not_found_or_disabled" }
```

Both `TestChannels_DefaultQueueId_NotFound` (random UUID) and `TestChannels_DefaultQueueId_CrossOrg` (real queue in different org) assert identical `e.Reason` strings. Clients cannot distinguish "queue is in another org" from "queue does not exist" — FOUND-08 enforced.

## Response Wrappers Used

### Queues handlers

| Status | Wrapper |
|--------|---------|
| 201 | `api.CreateQueue201JSONResponse(mapQueue(row))` |
| 200 (GET) | `api.GetQueue200JSONResponse(queue)` |
| 200 (LIST) | `api.ListQueues200JSONResponse{Items, NextCursor, HasMore}` |
| 200 (PATCH) | `api.UpdateQueue200JSONResponse(mapQueue(row))` |
| 204 | `api.DeleteQueue204Response{}` |
| 400 | `api.{Create|Get|List|Update|Delete}Queue400JSONResponse{BadRequestJSONResponse}` |
| 404 | `api.{Get|Update|Delete}Queue404JSONResponse{NotFoundJSONResponse}` |
| 409 (CREATE) | `api.CreateQueue409JSONResponse(api.ErrorResponse{...})` |
| 409 (UPDATE) | `api.UpdateQueue409JSONResponse{Current, Error: ...VersionConflict, Reason}` |
| 500 | `api.{Create|Get|List|Update|Delete}Queue500JSONResponse` |

### Channels handlers

| Status | Wrapper |
|--------|---------|
| 201 | `api.CreateChannel201JSONResponse(mapChannel(row))` |
| 200 (GET) | `api.GetChannel200JSONResponse(channel)` |
| 200 (LIST) | `api.ListChannels200JSONResponse{Items, NextCursor, HasMore}` |
| 200 (PATCH) | `api.UpdateChannel200JSONResponse(mapChannel(row))` |
| 204 | `api.DeleteChannel204Response{}` |
| 400 | `api.{Create|Get|List|Update|Delete}Channel400JSONResponse{BadRequestJSONResponse}` |
| 404 | `api.{Get|Update|Delete}Channel404JSONResponse{NotFoundJSONResponse}` |
| 409 (CREATE) | `api.CreateChannel409JSONResponse(api.ErrorResponse{...})` |
| 409 (UPDATE) | `api.UpdateChannel409JSONResponse{Current, Error: ...VersionConflict, Reason}` |
| **422 (CREATE)** | `api.CreateChannel422JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidReference, Reason: "default_queue_id_not_found_or_disabled"})` |
| **422 (UPDATE)** | `api.UpdateChannel422JSONResponse(api.ErrorResponse{Error: api.ErrorCodeInvalidReference, Reason: "default_queue_id_not_found_or_disabled"})` |
| 500 | `api.{Create|Get|List|Update|Delete}Channel500JSONResponse` |

## Decisions Made

None — plan executed exactly as specified. The four critical patterns (channel_types[] cast, nullable default_queue_id, D-76 probe, FOUND-08 identical-shape) all matched the plan's pre-written interface block.

## Deviations from Plan

None — plan executed exactly as written.

The two new files (queues_test.go, channels_test.go) plus the seedQueueForOrg helper in testutil_test.go all align with the plan's must_haves and `read_first` guidance. queues.go has 376 lines (plan min 220), channels.go has 415 lines (plan min 280), queues_test.go has 430 lines (plan min 250), channels_test.go has 618 lines (plan min 300).

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Threat Model Re-check

| Threat ID | Disposition Verified |
|-----------|----------------------|
| T-3-31 (cross-org information disclosure via probe) | Mitigated — TestChannels_DefaultQueueId_CrossOrg asserts identical 422 reason as not-found case. |
| T-3-32 (TOCTOU probe-then-INSERT race) | Accepted (v0.1, D-76). No test exercises the race; pg 23503 backstop catches the rare race in CreateChannel's 422 fallback (mapPgError "fk_violation"). |
| T-3-33 (TEXT[] DoS) | Accepted (v0.1). No `maxItems` set in spec; will inherit Phase 5's MaxBytesReader. |
| T-3-34 (legacy channel rows with undefined version) | Not applicable — greenfield deploy, all channels created post-migration with version=1. |

## Self-Check

Files exist:
- queues.go (376 lines), queues_test.go (430 lines), channels.go (415 lines), channels_test.go (618 lines), 03-08-SUMMARY.md (this file) — all present.

Commits exist:
- `d1c2bf5` (Task 1: queues), `4e367c5` (Task 2: channels) — both in git log.

Plan acceptance criteria:
- queues.go: cache.GetOrSet[api.Queue]=1, cache.Key("queues",...)=9, h.deps.Cache.Del=4, GetQueueByIdAnyVersion=1, ChannelTypes:=3, mapQueue=1 ✓
- channels.go: cache.GetOrSet[api.Channel]=1, cache.Key("channels",...)=9, QueueExistsAndEnabledInOrg=2, ErrorCodeInvalidReference=2, default_queue_id_not_found_or_disabled=2, CreateChannel422JSONResponse=2, UpdateChannel422JSONResponse=2, GetChannelByIdAnyVersion=1, mapChannel=1 ✓
- queues_test.go: 13 top-level TestQueues_*; TestQueues_ChannelTypesRoundTrip=1 ✓
- channels_test.go: 21 top-level TestChannels_*; all four D-76 must-haves (HappyPath, NotFound, Disabled, CrossOrg) + plan aliases (Create_UnknownQueueId_422, Update_QueueDisabledInSameOrg_422, CrossOrgQueueReject_422, InvalidReference) present ✓

End-to-end verification:
- `go build ./internal/catalog/...` → Success
- `go vet ./internal/catalog/...` → No issues
- `go test -race -run "TestQueues|TestChannels"` → 38 passed (no failures, no skips)
- Full catalog package test run → 64 passed (agents 19 + queues 13 + channels 21 + cursor 5 + subtests)

## Self-Check: PASSED

## Next Phase Readiness

- Plan 03-09 inherits two new template patterns: the D-76 probe block (channels.go) for agent_skills' SkillsPresentInOrg pre-write check, and the simplified-agents.go skeleton (queues.go) for adapters.go.
- Plan 03-10 isolation suite gains TestChannels_DefaultQueueId_CrossOrg as the canonical cross-org D-76 proof to extend.
- No blockers introduced; the orgDB SQLChecker in panic mode caught no false-positives during 38 -race test runs.

---
*Phase: 03-catalog-crud-go*
*Plan: 08*
*Completed: 2026-05-16*
