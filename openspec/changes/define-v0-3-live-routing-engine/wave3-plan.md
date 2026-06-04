# Wave 3 — Lease presence & DB-solid capacity (implementation plan, rev 2)

Goal (tasks.md W3): an agent's *offerability* derives from live signals —
**connected** (TTL lease, Redis-primary) AND **Ready** (agent_states) AND
**under-capacity** (per-(agent,channel) slot rows held transactionally). Expose a
**LiveCandidateSource** the live route path uses as a hint; the authoritative gate
is the offer-tx slot acquire. The matcher (W4) consumes these; W3 builds + tests
the substrate and wires presence into the gateway.

> rev 2 folds the codex + gemini plan review (1 BLOCK, 6 HIGH, 3 MED, 1 LOW).
> The deltas from rev 1 are marked **[Rxx]**.

## Slot state model (made explicit — codex MED)

A slot row is in exactly one state, derived from two columns:
- **free**: `reservation_id IS NULL` (and `hold_expires_at IS NULL`).
- **pending**: `reservation_id IS NOT NULL AND hold_expires_at IS NOT NULL`
  (offered, not yet accepted; the hold can expire → sweepable).
- **confirmed**: `reservation_id IS NOT NULL AND hold_expires_at IS NULL`
  (accepted; held for the live interaction; NEVER swept by the timer).

Confirmed slots are reclaimed only by an explicit terminal release or the
**reconciler** (below) — not by the sweep timer **[R-leak]**.

## Stage 1 — Capacity slot rows (DB-solid)

Schema (fold into `migrations/000001_init.up.sql` per single-migration-pre-release
rule — note: golang-migrate keys on version, NOT a file checksum, so an
already-applied dev DB will NOT pick up the new table; it must be reset/
re-migrated via the pipeline, same pending blocker as the W2 tables. Fresh
testcontainers are unaffected):

```sql
CREATE TABLE agent_capacity_slots (
  org_id          UUID NOT NULL,
  agent_id        UUID NOT NULL,
  channel         TEXT NOT NULL,
  slot_no         INT  NOT NULL,
  reservation_id  UUID NULL,
  hold_expires_at TIMESTAMPTZ NULL,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (org_id, agent_id, channel, slot_no)
);
CREATE INDEX agent_capacity_slots_sweep_idx
  ON agent_capacity_slots (hold_expires_at) WHERE hold_expires_at IS NOT NULL;
-- One reservation can hold at most one slot anywhere (codex MED). Does NOT
-- impose capacity=1: the key is reservation_id, not agent.
CREATE UNIQUE INDEX agent_capacity_slots_one_per_reservation
  ON agent_capacity_slots (org_id, reservation_id) WHERE reservation_id IS NOT NULL;
```

Queries (`internal/db/queries/capacity.sql`, all org_id-filtered):
- `ProvisionCapacitySlots` — INSERT slot_no `generate_series(1,$cap)` for
  (agent,channel) `ON CONFLICT DO NOTHING`. Capacity source = per-channel constant
  map (voice=1, chat=N) with a TODO pointer to per-agent capacity config (D4
  deferral). Shrinking does NOT delete rows; acquire filters by current cap **[R-shrink]**.
- `AcquireCapacitySlot :one` — CTE by PK (not ctid — codex LOW), filtered by the
  caller's current capacity so over-provisioned rows from a past higher cap are
  never acquired **[R-shrink]**:
  ```sql
  WITH picked AS (
    SELECT org_id, agent_id, channel, slot_no FROM agent_capacity_slots
    WHERE org_id=$1 AND agent_id=$2 AND channel=$3
      AND reservation_id IS NULL AND slot_no <= $5
    ORDER BY slot_no FOR UPDATE SKIP LOCKED LIMIT 1)
  UPDATE agent_capacity_slots s SET reservation_id=$4, hold_expires_at=$6, updated_at=NOW()
  FROM picked p WHERE s.org_id=p.org_id AND s.agent_id=p.agent_id
    AND s.channel=p.channel AND s.slot_no=p.slot_no
  RETURNING s.slot_no;
  ```
  ErrNoRows ⇒ at capacity → caller aborts the offer (the authoritative gate).
- `ConfirmCapacitySlot :execrows` — accept: `SET hold_expires_at=NULL WHERE
  reservation_id=$res AND hold_expires_at > now()`. The `> now()` guard means an
  expired pending hold cannot be confirmed even if the sweep hasn't run yet
  (defense-in-depth atop AcceptReservation's own `expires_at > clock_timestamp()`
  guard) **[R-confirm]**. 0 rows ⇒ slot lost → accept conflict (release the route
  run-lock via the existing savepoint path).
- `ReleaseCapacitySlot :execrows` — terminal: `SET reservation_id=NULL,
  hold_expires_at=NULL WHERE reservation_id=$res`. **Idempotent**: 0 rows is a
  debug log, not an error — a sweep/reconcile may have freed it first **[R-rel]**.
- `SweepExpiredCapacityHolds :execrows` — pending-only:
  `SET reservation_id=NULL, hold_expires_at=NULL WHERE hold_expires_at IS NOT NULL
  AND hold_expires_at < now()`. Sets BOTH NULL so a freed row is fully free and
  not re-swept; confirmed slots (hold_expires_at NULL) are untouched **[R-sweep]**.
- `ReconcileOrphanedSlots :execrows` — release any slot (pending OR confirmed)
  whose reservation is no longer live: `... SET reservation_id=NULL,
  hold_expires_at=NULL WHERE reservation_id IN (SELECT id FROM reservations r
  WHERE r.org_id=$1 AND r.state IN ('accepted'...terminal) ...)` — precise query
  TBD against the reservation lifecycle; this is the safety net for a confirmed
  slot orphaned by a gateway/agent crash **[R-leak]** (codex+agy HIGH).
- `CountFreeCapacitySlots :one` — hint read for the candidate source; filtered by
  current cap, `reservation_id IS NULL AND slot_no <= $cap`.

`CapacityService` (`internal/flowrt/capacity.go`) wraps these on an `*OrgTx` so an
acquire composes with the offer tx (same savepoint pattern as `liveOfferer`):
`AcquireInTx`, `ConfirmInTx`, `ReleaseInTx`. Release/Confirm are **gated by the
terminal transition that actually fired** in the same tx — e.g. the RONA/timeout
worker releases the slot ONLY when its `UPDATE reservations ... WHERE
state='offered' AND expires_at<now()` affected 1 row, so a stale worker can't free
a slot that was meanwhile accepted **[R-staleworker]** (codex HIGH).

Tests (testcontainer, `-race`): provision 1 (voice) + N (chat); acquire to
exhaustion → ErrNoRows; confirm clears hold + rejects an expired hold; release is
idempotent; M>N concurrent acquirers yield exactly N winners (SKIP LOCKED);
sweep frees a pending expired hold but never a confirmed one; reconcile frees a
confirmed slot whose reservation went terminal; the one-per-reservation unique
index rejects a double-acquire; shrink: cap=N then acquire with cap=1 only ever
takes slot_no 1.

## Stage 2 — Presence lease (Redis primary, DB audit)

`internal/presence/presence.go`: `Store` interface —
`Renew(ctx, org, agent uuid.UUID, sessionID string) error`,
`Connected(ctx, org, agent) (bool, error)`, `Drop(ctx, org, agent, sessionID string) error`.
Key = `presence:{org}:{agent}` → **sessionID** (the lease token), TTL =
`presenceTTL` (3× the heartbeat budget). Redis impl (`redis_store.go`) + `MemStore`
fake.
- `Drop` is a **CAS delete** (Lua: `if redis.call('GET',k)==ARGV[1] then
  redis.call('DEL',k)`) so a stale teardown can't evict a newer reconnect's lease
  **[R-drop]** (codex HIGH). `Renew` is `SET k sessionID EX ttl` (last writer for
  this agent wins — a reconnect with a new session takes over).

Wire into the gateway (`internal/wsgateway/gateway.go`), additive:
- `Deps.Presence presence.Store`. connect → `Renew(sessionID)`; heartbeat →
  `Renew`; teardown defer → `Drop(sessionID)`. Socket lifecycle stays best-effort
  (a Redis blip must not tear a live socket — the `readDeadline` from W2 still
  bounds half-open). Presence failures are logged.

DB audit/rebuild: `agent_sessions` already records connect/heartbeat/terminate; a
gateway-start Redis rebuild from live sessions is a documented follow-up (D2),
NOT W3.

Tests: MemStore renew/expire/drop incl. CAS (stale-session Drop is a no-op);
gateway connect/heartbeat/close drive presence (MemStore via Deps).

## Stage 3 — LiveCandidateSource + offer/command wiring

`internal/flowrt/candidate_live.go`: live candidate source = eligible+Ready
(`ListRoutableCandidates`) post-filtered by **connected** (`presence.Connected`)
AND a free slot hint (`CountFreeCapacitySlots`). Shape mirrors
`buildSnapshot`'s `runtime.Snapshot{QueueCandidates}` so the executor is unchanged.

**No silent fallback (codex BLOCK) [R-block]:** a LIVE route REQUIRES
presence+capacity. If the deps are absent it is a configuration error — the live
path does NOT degrade to `buildSnapshot`. `buildSnapshot` remains reachable ONLY
from the explicit sim/replay path (trace_sim) and tests that opt into it. Tests
for live wiring inject a `MemStore` + a real `CapacityService` (testcontainer),
NOT nil.

**Redis fail-closed-as-infra-error (codex+agy HIGH) [R-redis]:** if
`presence.Connected` errors, `buildLiveSnapshot` returns an infra error that
aborts the route run (the worker retries) — it does NOT treat the agent as
disconnected and it does NOT fall back to a DB snapshot. A Redis outage parks
live routes (retryable) rather than mis-routing or silently emptying the pool.

Offer/accept/terminal wiring:
- `liveOfferer.Offer` acquires a slot (`AcquireInTx`) in its existing per-offer
  savepoint BEFORE `InsertReservationOffer`; ErrNoRows (at capacity) ⇒ rollback +
  skip candidate (same control flow as today's busy/ineligible path). The acquire
  is the **authoritative gate**; the candidate-source free-count is only a hint
  (TOCTOU is safe — codex/agy LOW).
- accept (`command.go applyAccept`) → `ConfirmInTx` after `AcceptReservation`
  succeeds, in the same tx. 0 confirmed rows ⇒ conflict (savepoint rollback).
- reject/complete/cancel + worker RONA/timeout + `no_candidate` fallback →
  `ReleaseInTx`, gated on the terminal transition affecting 1 row **[R-staleworker]**.

Tests: disconnected agent omitted from live pool; at-capacity agent omitted;
offer→reject releases the slot and re-offers; offer→accept confirms (hold
cleared); Redis-error aborts the run (no mis-route); sim path unchanged.

## Sequencing (codex)

1. Migration (000001 fold) → `sqlc generate` → add `agent_capacity_slots` to
   SQLChecker `tenantTables`.
2. CapacityService + queries + tests (no live wiring yet).
3. Presence store (with session-token CAS) + gateway lease wiring + tests.
4. Capacity provisioning + startup backfill, wired into agent create/update so an
   agent is provisioned BEFORE it can be routed (else it looks at-capacity)
   **[R-provision]**.
5. LiveCandidateSource + offer/accept/terminal wiring + tests.
6. `cmd/api/main.go`: construct `presence.NewRedisStore(rdb)` + `CapacityService`,
   inject into `wsgateway.Deps` + `flowrt` deps + the sweep/reconcile worker loop.

Do NOT add live fallback behavior just to keep old tests green — use fakes or the
explicit sim mode (codex).

## Verification

1. `go build ./... && go vet ./...`; `go test ./internal/...` incl.
   `./internal/presence/...`; `go test -race` on capacity-concurrency + presence.
2. `golangci-lint run` clean; `sqlc generate` + `go generate` no drift.
3. Cross-AI review of the implementation diff before declaring W3 done.
4. Determinism: sim/replay never touches presence/capacity — trace replay stays
   byte-identical.
