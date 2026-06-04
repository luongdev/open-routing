# Wave 3 — Live Routing Runtime: Implementation Plan (v2, post cross-AI review)

## Goal
Turn the deterministic runtime LIBRARY (used by simulation) into a LIVE plane:
accept a route request, execute the pinned published flow, offer reservations to
real agents, park durable continuations for waits/timeouts, resume on agent
action or due-time, drive agent state — org-isolated, race-safe, crash-safe,
replay-faithful.

Schema exists (migration 000001). v2 adds a small additive migration (000003)
for the route-level execution lock + cursor (below). Agent state machine +
WrapUp expiry are in `internal/state/`.

## What changed after codex + agy review (both independently flagged these)
1. **No pinning of volatile agent state.** Freezing the candidate pool at route
   start kills a live queue (it never sees agents who log in later). → **record /
   replay**: a live run reads CURRENT eligible+Ready agents at each offer; it
   RECORDS the read result into the trace. Simulation/replay reconstructs the
   pool from the recorded result, not the DB. So determinism = replay the
   recorded reads, NOT freeze-the-world. The pinned snapshot keeps only immutable
   inputs (flow version, interaction_input, skill defs for audit).
2. **Route-level exclusive lock.** Every execution entry CAS-acquires the route:
   `UPDATE route_requests SET status='running' WHERE id=$1 AND status IN
   ('pending','waiting') RETURNING …`. 0 rows → someone else owns it / cancelled
   → abort. On suspend → back to `waiting`; on terminal → completed/failed. This
   is the single authority that prevents split-brain (API vs worker) and fences
   stale resumes.
3. **Cursor lives on `route_requests`, not continuations** → no API↔worker
   deadlock (API never locks the continuation row). API/worker read
   `resume_cursor` off the route after acquiring the route lock.
4. **One transaction per resume step.** Acquire lock → run executor to the next
   suspension/terminal → ALL side-effects (reservation rows, route status+cursor,
   agent state, `runtime_events` outbox, continuation insert; NO cross-row
   continuation cancel) in ONE pgx.Tx, committed atomically. Executor's live
   driver writes through the tx (ExecCtx carries the tx-backed deps).
5. **CAS + fencing everywhere.** Reservation transitions are guarded UPDATEs
   (`WHERE state='offered' RETURNING`); the worker resolves its continuation with
   `WHERE claimed_by=$me` fencing. A stale timer / lost-lease worker updates 0
   rows and no-ops. Accept-vs-timeout: whoever flips `offered` first wins; the
   loser sees 0 rows.
6. **Route completion comes from the executor reaching a terminal**, not from
   `CompleteReservation`. Complete = reservation done + agent Engaged→WrapUp;
   the route's own status is driven by the flow run.

## Schema (pre-release: folded into 000001, NOT a new migration)
The route run-lock columns live directly in the `route_requests` CREATE TABLE in
`migrations/000001_init.up.sql`: `resume_cursor JSONB`, `current_reservation_id
UUID`, `run_seq INTEGER NOT NULL DEFAULT 0` (run_seq bumped on each suspend for
cursor fencing). Pre-release we keep ONE consolidated migration pair — no
forward 000003. `ux_reservations_route_active` (includes `completed`) is kept:
**v0.2 contract = one terminal accepted assignment per route** (transfer/consult/
multi-reservation is v0.3 — would loosen the index).

## Cursor schema (resume_cursor JSONB)
`{ v:1, node_id, node_kind:"wait"|"reservation", vars, reservation?:{attempt,
last_timed_out} }`. No pool_remaining — each reservation offer re-reads live
eligible agents and excludes this route's already-offered agents (its prior
reservation rows). Resume is fenced by the route lock + node_id match.

## Increments (each: code + tests + commit; deploy after W3-3)

### W3-1 — sqlc queries + route execution + reads
- Queries: `route_requests.sql` (insert; `AcquireRouteForRun` = the CAS lock;
  `SuspendRoute` sets waiting+cursor+bump run_seq; `FinishRoute` terminal;
  get/list), `reservations.sql` (insert offer; guarded accept/reject/timeout/
  complete/cancel; get; list-by-route; list-active-agents-for-skip),
  `continuations.sql` (insert; claim SKIP LOCKED; resolve fenced; cancel-by-guard),
  `runtime_events.sql` (append), flow_entry_bindings active lookup.
- `CreateRouteRequest`: resolve active published flow for (channel, entry_code) →
  422 `missing_published_flow`; in ONE tx: insert route_request, acquire run lock,
  compile plan, run LIVE executor to first suspension/terminal, write trace +
  events, set status. Reject (422) publishing/running a flow that suspends inside
  a control-flow region (non-live-routable; validated).
- Reads: Get/List route requests, GetRouteRequestTrace, GetReservation,
  ListRouteRequestReservations, ListFlowEntryBindings — org-scoped.

### W3-2 — live reservation (record/replay) + executor resume in a tx
- `ExecMode` live|sim. Live deps interface (tx-backed): `EligibleAgents(skills…)`
  returns current Ready+eligible agents EXCLUDING this route's already-offered;
  `OfferReservation(agent,timeout)` inserts offered row + a `reservation_timeout`
  continuation, returns id; both record into the trace/events.
- Reservation node as an explicit phase machine (codex MED-11): `start_offer`
  (read live eligible → if none: no_candidate; else offer top, suspend),
  `on_accept` → accepted port, `on_reject/on_timeout` → next offer or
  timeout/no_candidate. The live read result is recorded so replay is exact.
- `RunFrom(tx, plan, cursor, signal)` re-enters at cursor.node_id with saved vars,
  runs to next suspension/terminal, returns the new cursor or terminal outcome.
  Unit-tested with a fake tx-backed live deps (no DB): offer→accept→continue,
  offer→reject→next, offer→timeout→fallback, exhausted→no_candidate.

### W3-3 — reservation lifecycle handlers (atomic + raced)
Each handler: ONE tx → acquire route run-lock (0 rows → 409 conflict) → guarded
reservation transition (0 rows → 409) → agent state transition → RunFrom from
route.resume_cursor with the signal → write trace/events/new cursor → set route
status → commit. Never touches the timeout continuation row (worker self-cleans).
- `AcceptReservation`: offered→accepted; agent Ready→Engaged; resume accepted.
- `RejectReservation`: offered→rejected; resume rejected → next offer / no_candidate.
- `CompleteReservation`: accepted→completed; agent Engaged→WrapUp (+wrapup_expiry
  continuation, reuse v0.1). Does NOT force route completed.
- `CancelRouteRequest`: cancel route; retract any offered reservation
  (offered→cancelled) + `reservation.cancelled` event so the agent UI drops the
  ring.
- Gates: concurrent offers can't double-book; racing accepts → one wins; accept→
  Engaged; complete→WrapUp + server expiry intact; cancel records cancelled
  without Engaging.

### W3-4 — continuation worker (cmd/runtime)
- Claim: `SELECT … FOR UPDATE SKIP LOCKED` where due & (pending OR claimed-lease-
  expired); set claimed_at/claim_expires_at/claimed_by. Per kind, in ONE tx,
  same pattern as handlers (acquire route lock → guarded transition → RunFrom →
  side-effects), then resolve the continuation `WHERE claimed_by=$me` (fencing).
  - `reservation_timeout`: guarded offered→timeout (0 rows → already accepted/
    cancelled → just mark continuation done); else resume timeout signal.
  - `wait`: resume at cursor (clock advanced to due_at; events use now()).
  - `wrapup_expiry`: ExpireWrapUp (reuse v0.1).
- Bounded retry w/ backoff + last_error; poison → continuation `cancelled` +
  route `failed` + event (no silent stuck routes).
- Gates: multiple replicas safe; accept-vs-timeout → one transition; lease reclaim
  after worker death; phantom-write fenced.

### W3-5 — events + p95 + CI/e2e
- `runtime_events` appended IN the step tx (true outbox): route.created/completed/
  failed, reservation.offered/accepted/rejected/timeout/completed/cancelled,
  agent.engaged/wrapup. `correlation_id` = a per-command id (NOT route_request_id;
  route scope is `route_request_id`).
- p95 fixture: CreateRouteRequest→first-offer latency over N runs; report p95.
  Timeout semantics use scheduled `expires_at`; event timestamps use DB now().
- CI: Go runtime+flowrt+worker tests (testcontainers); e2e smoke publish→route→
  offer→accept→complete→trace; replay determinism test (re-run from recorded
  reads → identical trace).

## Determinism, isolation, safety (final)
- Determinism via RECORD/REPLAY of live reads, not world-freeze. Replay reads the
  pool from the trace; live reads the DB and records it.
- Route run-lock (status CAS) is the single execution-exclusion authority.
- All tenant tables org-filtered (SQLChecker). Guarded UPDATEs + partial unique
  indexes + claimed_by fencing are the only concurrency authority.
- Every resume step is one atomic tx incl. the runtime_events outbox.

## Out of scope (v0.3+)
- Live wait/reservation INSIDE a control-flow region (region resume cursor) —
  rejected at publish as non-live-routable. Broadcast offers. Real adapters.
  Transfer/consult/multi-assignment per route. Per-candidate offer node.
