---
phase: 04
slug: agent-state-machine-go
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-17
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from 04-RESEARCH.md §Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `testing` stdlib + `github.com/stretchr/testify/require` |
| **Config file** | N/A (Go test, no separate config) |
| **Quick run command** | `cd services/api && go test -short ./internal/state/... ./internal/domain/...` |
| **Full suite command** | `cd services/api && go test ./...` (includes testcontainer + isolation lanes) |
| **Estimated runtime** | ~5s (quick) / ~60s (full with testcontainers + clockwork) |

---

## Sampling Rate

- **After every task commit:** Run `go test -short ./internal/state/... ./internal/domain/...` (~5s)
- **After every plan wave:** Run `go test ./internal/state/... ./internal/domain/... ./test/isolation/...` (~60s with testcontainers)
- **Before `/gsd-verify-work`:** Full suite must be green + `task gen && git diff --exit-code` (codegen-drift gate)
- **Max feedback latency:** 60s

---

## Per-Task Verification Map

| Req ID | Wave | Behavior | Test Type | Automated Command | File Exists |
|--------|------|----------|-----------|-------------------|-------------|
| STATE-01 | 4 | agent_states row per agent with status enum | integration | `go test ./internal/state/... -run TestCreateAgentSeedsState` | ❌ W0 |
| STATE-02 | 1 | PATCH accepts allowed transitions | unit (table-driven) | `go test ./internal/state/ -run TestTransitionMatrix_Exhaustive` | ❌ W0 |
| STATE-03 | 2 | Invalid transitions → 409 with from/to | integration | `go test ./internal/state/ -run TestPatchAgentStatus_InvalidTransition` | ❌ W0 |
| STATE-04 | 2 | Ready→Break requires same-org break_reason_id | integration | `go test ./internal/state/ -run TestPatchAgentStatus_BreakReasonProbe` | ❌ W0 |
| STATE-04 | 5 | cross-org break_reason → 422 | isolation | `go test ./test/isolation/ -run TestState_BreakReasonCrossOrg` | ❌ W0 |
| STATE-05 | 1 | Engaged carries engaged_channel (system-only schema-ready) | unit + schema | `go test ./internal/state/ -run TestTransitionMatrix_EngagedRequiresChannel` | ❌ W0 |
| STATE-06 | 2 | Agent sets post_interaction_state while Engaged | integration | `go test ./internal/state/ -run TestPatchAgentStatus_PostInteractionState` | ❌ W0 |
| STATE-07 | 3 | WrapUp TTL fires server-side | integration (clockwork) | `go test ./internal/state/ -run TestWrapUpTTL_FiresOnExpiry` | ❌ W0 |
| STATE-07 | 5 | WrapUp survives browser disconnect (acceptance) | integration | `go test ./internal/state/ -run TestWrapUpTTL_AcceptanceTTLPlus5` | ❌ W0 |
| STATE-08 | 2 | Monotonic state_version | unit | `go test ./internal/state/ -run TestStateVersion_Monotonic` | ❌ W0 |
| STATE-09 | 1 | Login/logout schema-only (no v0.1 fire) | unit | `go test ./internal/state/ -run TestMatrix_LogoutDeferred` | ❌ W0 |
| STATE-10 | 1 | IsRoutable 4-case matrix | unit | `go test ./internal/domain/ -run TestIsRoutable` | ❌ W0 |
| Acceptance #1 | 5 | All allowed/rejected transitions (success criterion) | integration (table-driven) | `go test ./internal/state/ -run TestAcceptance_AllTransitions` | ❌ W0 |
| Acceptance #2 | 5 | Cross-org break_reason rejection | isolation | `go test ./test/isolation/ -run TestState_CrossOrgBreakReason` | ❌ W0 |
| Acceptance #3 | 5 | WrapUp + disconnect + TTL+5s | integration | `go test ./internal/state/ -run TestAcceptance_WrapUpExpiresWithoutClient` | ❌ W0 |
| Acceptance #4 | 1 | IsRoutable 4 cases | unit | `go test ./internal/domain/ -run TestIsRoutable` | covered by STATE-10 |
| Acceptance #5 | 5 | state_version monotonic on every mutation | integration | `go test ./internal/state/ -run TestAcceptance_StateVersionMonotonic` | ❌ W0 |
| force=true happy | 2 | force bypasses transition matrix | integration | `go test ./internal/state/ -run TestForce_BypassesMatrix` | ❌ W0 |
| force=true cross-org | 5 | force does NOT bypass cross-org break_reason probe | isolation | `go test ./test/isolation/ -run TestState_ForceDoesNotBypassCrossOrgProbe` | ❌ W0 |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/api/internal/state/agent_states_test.go` — stubs for STATE-01..STATE-06, STATE-08, force
- [ ] `services/api/internal/state/transitions_test.go` — stubs for STATE-02, STATE-05, STATE-09 (matrix exhaustive walk)
- [ ] `services/api/internal/state/ttl_test.go` — stubs for STATE-07 (clockwork.FakeClock)
- [ ] `services/api/internal/state/testutil_test.go` — shared fixtures (mirrors `catalog/testutil_test.go`)
- [ ] `services/api/internal/domain/state_test.go` — stubs for STATE-10 (IsRoutable matrix)
- [ ] `services/api/test/isolation/state_test.go` — D-94 cross-org probes
- [ ] `services/api/internal/server/request_id_exhaustiveness_test.go` (EDITED) — covers new response types from D-92
- [ ] Framework install: `go get github.com/jonboulle/clockwork@v0.4.0` (Wave 0)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| *(none — all phase behaviors have automated verification)* | | | |

*All Phase 4 acceptance criteria are covered by automated integration tests (clockwork enables deterministic TTL expiry; testcontainers provide real Postgres; miniredis covers cache).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (8 files above)
- [ ] No watch-mode flags (Go test runs single-pass)
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter (after plan-checker green)

**Approval:** pending
