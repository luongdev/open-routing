---
phase: 04-agent-state-machine-go
reviewed: 2026-05-17T05:13:26Z
depth: deep
base: 251444bfdc5bd569d210ede9c9146e88d1d1d2a3
head: e9ea7b23de4bd255c7328bae856376e185b0632d
files_reviewed: 33
findings:
  critical: 3
  warning: 3
  info: 1
  total: 7
status: fixed_after_review
cross_review:
  gemini: BLOCK
  claude: READY
  gemini_post_fix: READY
  synthesis: READY
---

# Phase 04: Code Review Report

**Reviewed:** 2026-05-17T05:13:26Z
**Depth:** deep
**Scope:** source diff from `origin/main..HEAD` plus current cross-review from Gemini and Claude
**Status:** FIXED AFTER REVIEW

## Summary

Initial review found three ship blockers. They have been fixed in the working tree and re-verified: lint is green, forced state recovery clears `engaged_channel`, and WrapUp expiry clears `post_interaction_state`.

This report supersedes the earlier `reviews/phase4-final-*.md` artifacts. Those reviews were run on narrower or text-described diffs and missed the current cross-field bugs.

## Verification

- `cd services/api && go test -count=1 ./...`: PASS
- `cd services/api && go test -race -count=1 ./...`: PASS
- `cd services/api && go vet ./...`: PASS
- `cd services/api && ../../.task/bin/golangci-lint run`: PASS
- `cd web && pnpm -F '*' lint`: PASS
- `cd web && pnpm -F '*' typecheck`: PASS
- `git diff --check`: PASS
- Gemini current focused review: BLOCK
- Gemini post-fix focused review: READY
- Claude focused review: READY, but fallback default model was used. Claude validated the unused helper finding and refuted the other findings; the refutations do not match the current SQL/control-flow paths called out below.

## Fix Disposition

- **CR-01 fixed:** removed unused `httpPOST` from `services/api/internal/state/testutil_test.go`.
- **CR-02 fixed:** `UpdateAgentStateStatus` and `ForceUpdateAgentStateStatus` clear `engaged_channel` whenever target status is not `Engaged`; added forced `Engaged -> NotReady` regression coverage.
- **CR-03 fixed:** `ExpireWrapUp` now derives target status from `post_interaction_state` and clears `post_interaction_state` in the same `UPDATE`; added `ready` and `not_ready` regression coverage.
- **WR-01 remains a product/API decision:** STATE-06 still requires `force=true` for the current v0.1 Engaged self-update path.
- **WR-02/WR-03 remain non-blocking performance scale risks.**

## Critical Issues

### CR-01: Lint Fails On Unused Test Helper

**File:** `services/api/internal/state/testutil_test.go:422`

`httpPOST` was defined but unused. `go test` passed, but `golangci-lint run` failed, so the branch was CI-blocked.

**Status:** Fixed by deleting `httpPOST`.

### CR-02: `engaged_channel` Can Leak Onto Non-Engaged States

**Files:**
- `services/api/internal/db/queries/agent_states.sql:31`
- `services/api/internal/db/queries/agent_states.sql:52`
- `services/api/internal/state/agent_states.go:330`

Initially both `UpdateAgentStateStatus` and `ForceUpdateAgentStateStatus` used:

```sql
engaged_channel = COALESCE(sqlc.narg('engaged_channel')::text, engaged_channel)
```

`buildForceUpdateParams` never sets `EngagedChannel`. A forced recovery from `Engaged` to `Ready` or `NotReady` therefore preserves the old channel. That violates `openapi/openapi.yaml:1037`, which says `engaged_channel` is non-null only when `status == Engaged`.

The existing force test seeds an Engaged row without `engaged_channel`, so it does not catch this.

**Status:** Fixed. SQL now clears `engaged_channel` whenever the target status is not `Engaged`; `TestPatchAgentStatus_Force_NotReadyClearsEngagedChannel` covers the forced recovery path.

### CR-03: WrapUp Expiry Leaves `post_interaction_state` On Ready/NotReady Rows

**Files:**
- `services/api/internal/db/queries/agent_states.sql:85`
- `openapi/openapi.yaml:1052`

Initially `ExpireWrapUp` derived the target status from `post_interaction_state`, then cleared `wrapup_until` and `engaged_channel`, but did not clear `post_interaction_state`.

After expiry, the row becomes `Ready` or `NotReady` while still exposing a non-null `post_interaction_state`. That violates the OpenAPI invariant: `post_interaction_state` is null when not in Engaged or WrapUp state.

**Status:** Fixed. The SQL now sets `post_interaction_state = NULL`; `TestWrapUpTTL_ExpireWrapUp_UsesAndClearsPostInteractionState` covers both `ready` and `not_ready`.

## Warnings

### WR-01: STATE-06 Requires `force=true` For Normal PIS Updates

**Files:**
- `services/api/internal/state/transitions.go:32`
- `services/api/internal/state/agent_states_test.go:336`
- `openapi/openapi.yaml:1119`
- `openapi/openapi.yaml:1818`

The API says agents may set `post_interaction_state` while currently `Engaged`. The implementation only has a passing test for `Engaged -> Engaged` with `force=true`, because the transition matrix has no Engaged self-edge.

This makes a normal agent preference update look like an admin recovery path and produces force audit semantics for routine behavior.

**Fix:** Handle a PIS-only update while current status is `Engaged` as a valid non-force operation, or update the spec to explicitly say this is admin/system-only in v0.1.

### WR-02: `GetAgentStatus` Still Hits DB On Every Cache Hit

**Files:**
- `services/api/internal/state/agent_states.go:21`
- `services/api/internal/state/agent_states.go:50`
- `services/api/internal/state/agent_states.go:69`

`GetAgentStatus` performs `agentEnabledCheck` against `agents` before calling `cache.GetOrSet`. This means the hot read path still does a DB query even when the state object is cached.

The enabled check is required for soft-delete semantics, but placing it before cache access removes most of the benefit of caching this endpoint.

**Fix:** Cache a combined enabled/state projection, move the enabled check into the cache loader with explicit invalidation on soft-delete, or document that this cache only avoids `agent_states` reads and not DB reads generally.

### WR-03: Expiring WrapUp Sweep Is Unbounded

**File:** `services/api/internal/db/queries/agent_states.sql:63`

`ListExpiringWrapUps` returns every WrapUp row across all orgs. This is acceptable for a small v0.1 dataset, but it is not production-safe once WrapUp volume grows.

**Fix:** Add batching or a due-time window before Phase 4 becomes production load-bearing.

## Info

### IN-01: Cross Review Disagreed, Gemini Matches The Current Code Paths

Gemini current focused review returned `BLOCK` and validated the same five main concerns: cache bypass, stale `post_interaction_state`, stale `engaged_channel`, STATE-06 force requirement, and unused `httpPOST`.

Claude current focused review returned `READY`, but used fallback default model after the requested high-quality invocation failed. Its reasoning treats `ExpireWrapUp` retaining `post_interaction_state` as intentional and uses `ExpireWrapUp` cleanup to refute the forced-transition channel leak. Those claims do not cover the forced `Engaged -> Ready/NotReady` path.

## Verdict

**READY WITH NOTES.** The three blockers are fixed and post-fix Gemini review returned READY. WR-01 remains an API semantics decision; WR-02 and WR-03 are non-blocking performance risks.
