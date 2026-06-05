# Wave 4 — The matcher (the engine) — implementation plan

Goal (tasks.md W4 + design D4/D8/D9): turn the route engine from
**offer-now-or-fail** into a **queue + bidirectional matcher**. A route that has
no available agent at decision time now **waits in a queue** instead of falling
through to fallback; a continuously-running matcher pulls the best-ranked waiting
route onto an agent the moment that agent becomes Ready / frees capacity. Builds
directly on the W3 substrate (presence lease, capacity slots, route run-lock CAS,
durable continuation worker).

## What already exists (do NOT rebuild)
- Interaction-driven offer at route creation: `liveOfferer` acquires a capacity
  slot + inserts an `offered` reservation inside the route tx; the route suspends
  at the reservation with a `reservation_timeout` continuation.
- Capacity slots (acquire/confirm/release/sweep/reconcile), presence lease
  (Connected/ConnectedMany), route run-lock (`AcquireRouteForRun`), worker tick.
- `command.go` / HTTP handlers resolve accept/reject/complete + release capacity.

## The core change: wait-for-match instead of fallback
Today `liveReservation` (node_execute.go) loops candidates; if none can be offered
it returns the `no_candidate` port → fallback. W4: when the live pool is empty
(or all at capacity) the route **parks as `waiting_match`** and is owned by the
matcher. Fallback becomes the *exhaustion/SLA* path, not the *nobody-free-right-now*
path.

Open decision for review: reservation-node `timeout_sec`/`max_attempts` semantics.
Proposal: `max_attempts` bounds RONA re-offers to DISTINCT agents; an empty pool
parks `waiting_match` (does not consume an attempt); an overall
`route_requests.match_deadline` (queue SLA, configurable, default e.g. 120s) bounds
total wait → on expiry the route resumes with the `no_candidate`/`timeout` port to
fallback. This keeps the node contract while adding true queueing.

## Stage 1 — Data model (fold into 000001)
`route_requests` add: `queue_id uuid NULL`, `priority int NOT NULL DEFAULT 0`,
`waiting_since timestamptz NULL`, `next_match_at timestamptz NULL`,
`required_skills text[] NOT NULL DEFAULT '{}'`, `match_deadline timestamptz NULL`,
`match_attempt_seq int NOT NULL DEFAULT 0`. New status value `waiting_match`.
Indexes: `(org_id, status, next_match_at)` partial WHERE status='waiting_match';
GIN on `required_skills`.

`reservations` add: `lease_token uuid NULL`, `agent_session_id uuid NULL` (D5
fencing — bound at offer, checked on every accept/reject/complete so a stale
replica can't resolve a re-offered reservation).

`route_decisions` (audit, D9): `id, org_id, route_request_id, decision_type
(interaction_offer|availability_pull|retry|sweep), decision_version, matcher_instance,
channel, queue_id, selected_agent_id, selected_slot_no, outcome
(offered|no_candidate|capacity_lost|lease_lost|route_lost), reason, created_at`,
plus JSONB `eligibility/candidates/excluded/ranking/capacity_snapshot`. Index
`(org_id, route_request_id, created_at DESC)`.

`agents` add: `routing_state text NOT NULL DEFAULT 'routable'`,
`routing_state_expires_at timestamptz NULL` (RONA cooldown — Missed agent is
non-routable until the TTL or an explicit Ready clears it).

SQLChecker: add `route_decisions` to tenantTables.

## Stage 2 — Enqueue (interaction-driven side)
`liveReservation` / `liveOfferer` flow: try the live pool first (existing
offer-now). If no candidate is offerable, park the route `waiting_match` with
`queue_id`, `priority`, `waiting_since=now()`, `required_skills` (denormalized from
the match_skill node config + queue), `match_deadline`. The reservation node emits
a new suspension kind `match` (no per-offer timeout; the matcher owns it).
`required_skills` come from the compiled plan (the match_skill node feeding the
reservation) — captured at enqueue.

## Stage 3 — The matcher (availability-driven pull) — cmd/runtime tick
Primary trigger = a durable sweep every ~2s (design D8: LISTEN/NOTIFY is only a
wake hint, NOT the trigger). Per org, bounded batch (≤100 agents / ≤100 routes):

For each **available agent** (Ready + connected lease + under-capacity + routable):
1. Pull the best-ranked waiting route this agent can serve:
   `SELECT ... FROM route_requests rr WHERE org_id=$ AND status='waiting_match'
    AND next_match_at<=now() AND required_skills <@ $agentSkills AND
    (queue served by agent) ORDER BY <effective score> FOR UPDATE OF rr
    SKIP LOCKED LIMIT 1` — lock ONLY rr (agy HIGH: bare FOR UPDATE locks joined
    rows → cross-agent deadlock). The same statement flips `status='offering'` /
    bumps `next_match_at` so a second puller can't grab it (pull-to-offer race).
2. In the offer tx: re-confirm the lease (`presence.Connected` + token), acquire a
   capacity slot (`AcquireCapacitySlot` FOR UPDATE SKIP LOCKED), insert the
   `offered` reservation with `lease_token`+`agent_session_id`, set
   `active_reservation_id`, then resume the route to suspend AT the reservation
   (reuse `resumeRoute`). Any failure (slot lost / lease lost / route moved) →
   roll back, return the route to `waiting_match` at the queue head
   (`next_match_at=now()`), record the outcome in `route_decisions`.
3. On success: `route_decisions` row (outcome=offered, selected agent/slot,
   ranking snapshot), `agent_outbox` offer frame (gateway relays it).

Fair ranking (design D4/agy HIGH): `effective = priority*W_p + queue_weight*10 +
aging + sla_boost` where aging grows with `now()-waiting_since` UNCAPPED (or
`W_p < max_aging`) so a sufficiently-aged lower band overtakes — NO priority
inversion that starves. Avoid sorting the whole backlog on a `now()` expression:
order by the indexable `priority DESC, waiting_since ASC` to get a bounded
candidate set, then refine the effective score in app (bounded N).

Concurrency invariant: per-route run-lock CAS (proven v2) + `FOR UPDATE OF rr SKIP
LOCKED` + capacity slot lock ⇒ no double-assign across replicas; the durable sweep
is the backstop if a NOTIFY is lost.

## Stage 4 — RONA + lease fencing
- RONA: an offer that times out (continuation worker) sets the agent
  `routing_state='missed'`, `routing_state_expires_at=now()+cooldown`; the matcher
  filters out non-routable agents; the next explicit Ready (or the TTL) clears it.
  Never an immediate re-offer to the same agent (design HIGH).
- Lease fencing: accept/reject/complete (command.go + HTTP) must match the
  reservation `lease_token`; a mismatch ⇒ conflict (a stale command for a
  re-offered reservation can't resolve it).

## Stage 5 — Tests
- Enqueue: a route with an empty live pool parks `waiting_match` (not fallback).
- Availability pull: agent becomes Ready → matcher offers the waiting route;
  reservation appears + slot held + route suspended at reservation.
- Fairness: a higher-priority newer route beats a lower-priority older one, BUT a
  sufficiently-aged lower-priority route overtakes (no starvation).
- Concurrency: two matcher instances, one waiting route + one agent → exactly one
  offer (SKIP LOCKED + run-lock); N agents + M routes no double-assign (`-race`,
  concurrent goroutines).
- RONA: offer timeout → agent non-routable → not re-offered until cooldown.
- Lease fencing: a stale-token accept ⇒ conflict.
- route_decisions: one row per decision with outcome + ranking.
- Determinism: sim/replay unaffected (matcher is live-only).

## Sequencing & guardrails
1. Migration + sqlc + SQLChecker. 2. Enqueue (node + route model) + tests.
3. Matcher pull loop + ranking + route_decisions + tests. 4. RONA + lease fencing.
5. Wire the matcher tick into cmd/runtime; bounded batch + Postgres `now()` only
   (never gateway clocks). Cross-AI review of THIS plan before building, then of
   the implementation. Keep each stage build+test+lint green and committed.
