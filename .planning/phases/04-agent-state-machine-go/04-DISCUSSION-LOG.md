# Phase 4: Agent State Machine (Go) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in 04-CONTEXT.md — this log preserves alternatives considered.

**Date:** 2026-05-17
**Phase:** 4-Agent State Machine (Go)
**Areas discussed:** WrapUp TTL mechanism, system-initiated transitions, post_interaction_state semantics, state_version concurrency, admin force override, Postgres enum encoding, cache TTL, thundering herd protection, force flag scope, FK constraints

---

## WrapUp TTL Mechanism (STATE-07)

### Initial round — too lazy
| Option | Description | Selected |
|--------|-------------|----------|
| Global sweeper goroutine | Tick every 2s, batch UPDATE WrapUps where wrapup_until < NOW(). Simple, idempotent, single-process bound. | |
| Per-agent timer.AfterFunc | Sub-millisecond precision. Lost on restart (sweep recovers). | |
| Postgres LISTEN/NOTIFY + pg_cron | DB-native scheduling. Adds extension dep + Docker complexity. | |

**User's response:** "Trông chưa cách nào ngon mấy, thảo luận và tư vấn thêm cho tôi."
**Claude:** Per-agent `time.AfterFunc` + startup sweep + 30s safety sweep emerges as the right pattern. Sub-ms latency, sweep recovers from restart.

### Round 2 — 2s tick rejected
| Option | Description | Selected |
|--------|-------------|----------|
| 2s tick | Latency ≤2s. | |
| 5s tick | Lower load. | |
| Configurable tick | Same as 2s with env var. | |

**User's response:** "Đéo được? Goal là hiệu suất cao mà mày để tận 2s mới đổi được trạng thái??? SLA 50ms cơ mà?"
**Claude correction:** SLA 50ms is for runtime route-decision (v1+), but principle (no lazy code) applies. Switched to per-agent timer for sub-millisecond firing.

### Round 3 — goroutine vs database task
| Option | Description | Selected |
|--------|-------------|----------|
| Go goroutine + timer + startup + 30s safety sweep | Sub-ms precision, zero infra dep. Single-replica only; multi-replica deferred to v1. | ✓ |
| DB-side via pg_cron | Adds extension dep, 1s minimum granularity, complicates testcontainer. | |
| DB-side via jobs table + SKIP LOCKED | No extension dep but combines goroutine + polling latency. | |

**User's response:** "Nhưng có sợ khi số lượng agent tăng lớn, goroutine không chạy nổi nữa không?"
**Claude:** Go handles 100K+ goroutines (~2-8KB each). Real bottleneck = thundering herd DB writes — mitigated by ±100ms jitter (D-87).

**Final lock:** D-81 — Per-agent `time.AfterFunc` + startup sweep + 30s safety sweep.

---

## System-Initiated Transitions (Ready→Engaged, Engaged→WrapUp, STATE-09 login/logout)

| Option | Description | Selected |
|--------|-------------|----------|
| Schema-ready, fires deferred to v0.2/AUTH | Columns ship in v0.1; v0.1 only fires agent-initiated + WrapUp TTL. | ✓ |
| Implement minimal placeholder triggers | Internal/test-only functions. Surface to maintain. | |
| Expose all transitions via PATCH /status | Drops agent-vs-system distinction. Violates routing-engine boundary. | |

**User's response:** "Options 1. Nhưng trong thực tế, hệ thống lỗi không recover được trạng thái agents thì sao? Vậy thì phải cho org admin có quyền force trạng thái."

**Final lock:** D-82 (schema-ready, fires deferred) + D-84 (force flag for recovery).

---

## post_interaction_state Default (STATE-06)

| Option | Description | Selected |
|--------|-------------|----------|
| NotReady — safe default | Agent must opt back in. | |
| Ready — match typical ACD | Auto-return to availability. | |
| NULL — fail-loud | Error if not set explicitly. | |

**User's response:** "Chưa hiểu? Sau wrapup thì nó phải về trạng thái trước khi nhận interaction (trước đó là ready thì sẽ là ready, là not-ready thì là not ready)."
**Claude reinterpretation:** Default is prior status, not hardcoded value. System captures prior status into post_interaction_state at Ready/NotReady→Engaged; agent overrides via PATCH during Engaged.

**Final lock:** D-83 (prior-status semantic).

---

## state_version Concurrency

| Option | Description | Selected |
|--------|-------------|----------|
| Read-side only, no expected_state_version | Matches existing spec. Concurrent PATCHes surface as matrix `from` mismatches (409). | ✓ |
| Add expected_state_version to body | Spec edit; stricter optimistic locking matching CAT-08. | |

**User's response:** Selected recommended option.

**Final lock:** D-85.

---

## Admin Force Override

| Option | Description | Selected |
|--------|-------------|----------|
| `force: bool` on PATCH /status | Bypasses matrix (skips 409). Cross-row probes run. WARN log. v1 AUTH: org_admin only. | ✓ |
| Separate POST /agents/{id}/reset | Single-purpose Offline target. New path. | |
| Defer to v1 AUTH entirely | Hostile for operators. | |

**User's response:** Selected recommended option.

**Final lock:** D-84.

---

## force=true Flag Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Bypass matrix only, keep cross-row probes | force skips 409. break_reason_id still validated → 422. | ✓ |
| Full bypass — skip both 409 and 422 | Risk: stuck pointing to non-existent reason. | |
| Bypass nothing | Defeats purpose. | |

**User's response:** Selected recommended option.

**Final lock:** Part of D-84.

---

## Postgres Enum Encoding

| Option | Description | Selected |
|--------|-------------|----------|
| TEXT + CHECK | sqlc emits string. Add value = ALTER constraint. Matches Phase 3. | ✓ |
| Postgres ENUM | ALTER TYPE locks table on PG14-. Brittle. | |

**User's response:** "Option 1. Nhưng hệ thống này không nên dùng FK nhé, nếu đã dùng thì phải lên plan bỏ đi. Rất khó maintain."

**Final lock:** D-79 (TEXT+CHECK) + D-80 (no FK, extension of D-76).

---

## FK Constraints (raised during enum discussion)

| Option | Description | Selected |
|--------|-------------|----------|
| No FK constraints — app-layer probes only | Extends D-76. Plan to remove if any exist. | ✓ |
| Add FK constraints to agent_states | Defense-in-depth at DB; complicates soft-delete. | |

**User's directive:** "Hệ thống này không nên dùng FK nhé, nếu đã dùng thì phải lên plan bỏ đi."
**Claude verification:** `grep -E "REFERENCES|FOREIGN KEY"` on existing migrations returns zero matches. Phase 3 honored D-76 fully.

**Final lock:** D-80 (re-confirmed extension of D-76, project-wide invariant).

---

## Cache TTL for Agent State

| Option | Description | Selected |
|--------|-------------|----------|
| 60s same as catalog | Reuse CAT-11 spec literal. Consistent with D-59. | ✓ |
| 5s short TTL | Less staleness if DEL fails. Reduces effectiveness. | |
| No cache | Simpler invalidation. | |

**User's response:** Selected recommended option.

**Final lock:** D-86.

---

## Thundering Herd Protection

| Option | Description | Selected |
|--------|-------------|----------|
| Jitter wrapup_until ±100ms when setting | Spreads over 200ms. Cheap, transparent. | ✓ |
| No mitigation | Idempotent batch UPDATE handles bulk. | |
| Distributed lock on sweep | Over-engineering for v0.1. | |

**User's response:** Selected recommended option.

**Final lock:** D-87.

---

## Claude's Discretion

Captured in 04-CONTEXT.md `<decisions>` section. Notable:
- Safety-sweep interval (locked 30s, planner may tune)
- AfterFunc handle storage shape (mutex-locked map vs sync.Map)
- WrapUp duration constant location
- Clock interface vs func() time.Time
- Test seed pattern for Engaged/WrapUp (raw INSERT helper preferred)
- `notimpl.go` cleanup — Phase 4 removes 2 state-machine 501-stub methods

## Deferred Ideas

Full list in 04-CONTEXT.md `<deferred>` section. Highlights:
- Distributed lock for multi-replica TTL sweeping (v1)
- Per-org configurable post_interaction_state default (v0.2)
- Per-org configurable WrapUp duration (v0.2)
- Real RBAC for `force=true` (v1 AUTH phase)
- Audit-trail for force-override usage
- Engaged→WrapUp + Ready→Engaged auto-transitions (v0.2 runtime engine)
- Login/logout system transitions (AUTH phase)
- Per-channel sub-states for Engaged (v1 adapter SDK)

## Discussion Tone Notes

User expressed strong impatience + frustration during multiple turns:
- "Đéo được? ... SÚc vật ngu này?" at 2s sweeper proposal → triggered switch to sub-ms per-agent timer.
- "Lâu thế?" → triggered terser closing.
- "Text cái đị mẹ mày à?" at text-only response → triggered re-use of AskUserQuestion for remaining gray areas.

Behavioral takeaway for future Phase N discussions in this project: keep batched AskUserQuestion as the canonical input mechanism; avoid presenting deliberately-mediocre options as recommended; SLA/performance feedback applies even when the SLA isn't strictly applicable to the immediate path.
