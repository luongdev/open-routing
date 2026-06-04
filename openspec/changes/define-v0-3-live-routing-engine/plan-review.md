# v0.3 Plan — Cross-AI Review (2026-06-04, pre-implementation)

codex + agy reviewed the plan (proposal/design/tasks). Findings folded into
design.md (decisions D2/D4–D9) and tasks.md (wave reorder + new tasks).

## BLOCK
- **Capacity not DB-solid** (codex): `active < capacity` races; the v0.2 per-agent
  partial-unique reservation index only models capacity=1 and contradicts chat=N.
  → DB-enforced capacity slots / counted guarded insert in the reservation tx.
- **Adapter contract too media-shaped** (codex) + **opaque handle / channel-agnostic**
  (agy): generalize to a channel-neutral assignment-event contract
  (offer/accept/connect/established/fail/disconnect/complete/wrap-up); `handle`
  opaque to the engine.
- **Caller abandonment missing** (agy): no signal for caller hangup in queue/ringing.
  → add `caller_abandoned`/Cancel event + matcher teardown of in-flight reservations.

## HIGH
- **Presence liveness** (codex+agy): DB-authoritative presence survives a crashed
  gateway → black-hole offers. → leased/TTL heartbeat presence (Redis primary for
  the *connection* dimension; DB = audit/last-known).
- **RONA** (agy): on offer timeout, leaving the agent Ready re-offers immediately.
  → timeout sets the agent non-routable (Missed/RONA).
- **Ghost rings on disconnect** (agy): immediate reject on WS blip churns/loses
  interactions. → short grace window for mid-offer disconnect before reject;
  reconnect resumes the offer.
- **WS idempotency** (codex): reconnect replay can duplicate accept/reject/complete.
  → message IDs + reservation version/lease token + ack + server-side dedupe.
- **Fairness/starvation** (codex): priority+longest-idle alone starves. → aging,
  queue weight, max-priority-bypass, deterministic tie-break.
- **Matcher/gateway boundary** (codex): agent commands must run through
  runtime-owned transactional command handlers; the gateway only transports.
- **Disconnect/reassign policy channel-driven** (codex): voice has media/abandon
  state; reassign rule belongs to the adapter, with a recorded terminal reason.

## MED
- **Durable sweep is primary, notify is a hint** (codex+agy): LISTEN/NOTIFY &
  pub/sub drop on restart → a continuous reconciliation sweep guarantees no orphan.
- **Final-offer tx re-check** (codex): re-validate connected/Ready/skills/queue/
  capacity against authoritative leased state at the offer insert.
- **Reservation lease/version on every transition** (codex): conditional on state
  + version + lease token so replicas can't double-resolve.
- **route_decision trace schema now** (codex): eligibility inputs, excluded
  candidates + reasons, ranking values, capacity snapshot, decision version.
- **WrapUp (ACW) max timer** (agy): configurable cap via the continuation worker,
  alongside manual completion.
- **Adapter contract before lifecycle** (codex): reorder so reservation semantics
  aren't retrofitted.
- **Richer ops metrics** (codex): SLA age, oldest-waiting, abandon/timeout rate,
  accept latency, reject reasons, per-channel occupancy, stuck-reservation alerts.

## LOW
- WS auth specifics (codex): scoped agent identity, org binding, session
  revocation, queue/skill authz on connect + per command.
- Protocol contract tests (codex): golden WS schema + reconnect/dup/timeout/
  stale-offer fixtures.
- SKIP LOCKED priority inversion (agy): acceptable, but a failed offer insert must
  return the route to the queue head immediately.
