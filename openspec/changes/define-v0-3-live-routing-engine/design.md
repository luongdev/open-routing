# Design: v0.3 Live Routing Engine

## Architecture overview

```
 agent client ──WS──┐                         ┌── channel adapter (mock voice in v0.3,
 (reference)        │                         │   real LiveKit/SIP in v0.4)
                    ▼                         ▼
            ┌──────────────┐   offer/accept  ┌──────────────────┐
            │  WS gateway  │◀───────────────▶│  matcher (engine) │
            │ (cmd/api)    │   presence      │  (cmd/runtime)    │
            └──────┬───────┘                 └────────┬─────────┘
                   │ presence/events                  │ reserve / pull
                   ▼                                  ▼
            ┌──────────────┐                  ┌──────────────────┐
            │ presence store│◀────────────────│ route_requests + │
            │ (Redis + DB)  │   live pool     │ reservations +   │
            └──────────────┘                  │ continuations    │
                                              └──────────────────┘
```

The control plane (`cmd/api`) terminates agent/adapter WebSockets and owns
presence. The runtime (`cmd/runtime`) owns the matcher and the reservation/
continuation state machine (already built in v0.2). They communicate through the
DB (route/reservation rows + outbox events) plus a lightweight notify channel
(Postgres LISTEN/NOTIFY or a Redis pub/sub) so the matcher reacts to "agent
became Ready" without polling.

## Key decisions

### D1 — Transport: WebSocket (not SSE+POST)
Offers must be pushed down AND accepts/presence pushed up with low latency; one
bidirectional socket per agent/adapter is simpler than SSE + a POST back-channel.
The socket carries a typed message envelope reusing the v0.2 canonical runtime
event shape. Reconnect: the socket is stateless-resumable — on reconnect the
client re-announces presence and the gateway replays any in-flight offer from the
durable reservation row (no lost offers).

### D2 — Presence: leased/TTL heartbeat (Redis primary for connection), DB = audit
The agent state machine stays the source of truth for *status*; presence adds a
**connection** dimension — a Ready agent with no live socket is NOT offerable. A
DB-authoritative presence row is unsafe for liveness: a crashed/OOM'd gateway
leaves agents "connected" → black-hole offers (review HIGH). So connection is a
**lease**: the gateway renews a TTL'd presence entry (Redis primary) via WS
heartbeat; expiry = offline. DB stores last-known/audit state, rebuilt into Redis
on gateway start; the matcher confirms the lease at the offer transaction.

### D3 — Live candidate pool replaces the snapshot for LIVE routes
v0.2 pins `buildSnapshot` at route start for determinism. For live routes the
matcher queries the **current** eligible+available+connected+under-capacity pool
at offer time. Simulation keeps the snapshot (determinism/replay unchanged). This
is a `LiveCandidateSource` behind the existing candidate interface — the executor
contract does not change. The live pool is a *hint*; the final offer transaction
re-checks connected/Ready/skills/queue-membership/capacity against authoritative
leased state (review MED) before inserting the offer.

### D4 — The matcher is bidirectional, lock-safe, DB-solid on capacity, and fair
Two triggers, one assignment invariant:
- **Interaction-driven**: a route needs an agent → offer the top live candidate.
- **Availability-driven**: an agent goes Ready / frees capacity → pull the
  best-ranked waiting route from a served queue and offer it.
Concurrency: per-route by the v2 `route_requests` run-lock CAS; cross-replica pull
via `FOR UPDATE SKIP LOCKED` over waiting routes; a failed offer insert returns
the route to the queue head immediately (no priority inversion, review LOW).
**Capacity is DB-solid** (review BLOCK): the v0.2 per-agent partial-unique
reservation index models capacity=1 only and contradicts `chat=N`, so capacity
becomes per-(agent,channel) **slot rows** (or a counted guarded insert in the
reservation tx) — an offer is created only if `held < capacity` under row lock.
**Ranking is fair, not just priority+idle** (review HIGH): priority with **aging**
(waiting time raises effective priority), queue weight, a bounded max-priority
bypass, and a deterministic final tie-break.

### D5 — Reservation lifecycle from real signals; RONA, grace, abandonment
Agent WS accept/reject and adapter events map onto the v0.2 reservation
transitions + route resume; **every transition is conditional on reservation
state + version + lease token** so replicas can't double-resolve (review MED). The
HTTP test-double endpoints stay for sim/CI/Route-Tester. Specifics the plan now
pins:
- **RONA** (review HIGH): an offer timeout sets the agent **non-routable**
  (Missed/RONA) — never an immediate re-offer to the same agent.
- **Disconnect grace** (review HIGH): mid-offer WS drop waits a short grace window
  (reconnect resumes the offer) before reject; no ghost-ring churn on a blip.
- **Caller abandonment** (review BLOCK): a `caller_abandoned`/Cancel signal tears
  down the in-flight reservation and route immediately, with a recorded terminal
  reason; mid-handling disconnect/reassign policy is **adapter-driven**, not a
  fixed rule.
- **WrapUp cap** (review MED): a configurable max ACW timer via the continuation
  worker frees capacity even without manual completion.

### D6 — Channel-neutral assignment contract (mock voice in v0.3, opaque handle)
The contract is a **channel-neutral assignment-event** model, not a media model
(review BLOCK): `Deliver(interaction, agent) → handle` plus events
`accepted → connecting → established → (completed | failed | disconnected |
caller_abandoned)`, each with idempotent correlation; the `handle` is **opaque**
to the engine so chat/email fit later. v0.3 ships a **mock voice adapter** driving
that lifecycle without media; v0.4 maps LiveKit/SIP onto the same events.

### D7 — WS protocol is idempotent and runtime-owned
The gateway only transports; **all agent commands run through runtime-owned
transactional command handlers** (review HIGH). Every WS message carries a message
id; accept/reject/complete carry the reservation version/lease token and are
**ack'd + server-side deduped** so a reconnect replay can't double-apply (review
HIGH). WS auth binds a scoped agent identity to its org with session revocation
and queue/skill authorization on connect and per command (review LOW).

### D8 — Durable work discovery; notify is only a wake-up
LISTEN/NOTIFY (or Redis pub/sub) drops messages on restart, so it is a **hint**
only. A continuously-running reconciliation **sweep** (every few seconds) over
waiting routes + free agents is the primary, durable trigger so no route is
orphaned (review MED).

### D9 — `route_decision` trace schema (defined now)
Each live decision records a compact `route_decision`: eligibility inputs, the
candidates considered, **excluded candidates + reasons**, ranking values, the
capacity snapshot, and a decision version — so a live route is fully explainable
(review MED), even though it is not bit-replayable.

## Determinism & the v0.2 spine
Live routing is inherently non-deterministic (real timing), so it is NOT replayed
step-for-step. Instead every decision (pool queried, candidate chosen, offer,
accept, timeout) is recorded as a v0.2 runtime event + trace, so a live route is
**explainable and auditable** after the fact even though it is not bit-replayable.
Simulation remains fully deterministic on the snapshot path.

## Risks
- **WS scale / fan-out**: many agents × many orgs on one gateway. Mitigation: the
  gateway is stateless (presence in Redis), horizontally scalable; offers are
  delivered via the durable reservation row, so a missed push is recovered on
  reconnect.
- **Split-brain matcher**: two replicas pulling the same route. Mitigation: SKIP
  LOCKED + the route run-lock CAS (proven in v0.2) + the durable sweep (D8).
- **Capacity races**: two interactions offered to an at-capacity agent.
  Mitigation: per-(agent,channel) slot rows under row lock (D4), not the
  capacity=1-only partial-unique index.
- **Presence drift on crash**: a dead gateway leaves agents "connected".
  Mitigation: TTL'd heartbeat lease (D2) expires offerability; DB is audit-only;
  the offer tx re-confirms the lease.
- **Stuck reservations / poison continuations**: carried from the v0.2 review
  (worker poison-pill, run_seq fence). The v0.2 hardening pass lands first so the
  live lifecycle builds on a sound base.

## Round-2 concretions (implementation blueprint)

codex + agy round-2 (see `plan-review.md` for round 1). codex supplied concrete
schemas/SQL; agy then found races in them — both folded here. These pin the
round-1 "weak" items (capacity, matcher, presence, WS idempotency, decision trace).

### Capacity = slot rows (decided; NOT a counted column)
```sql
CREATE TABLE agent_capacity_slots (
  org_id uuid NOT NULL, agent_id uuid NOT NULL, channel text NOT NULL,
  slot_no int NOT NULL, reservation_id uuid NULL,
  held_at timestamptz NULL, hold_expires_at timestamptz NULL,
  PRIMARY KEY (org_id, agent_id, channel, slot_no),
  UNIQUE (org_id, reservation_id) WHERE reservation_id IS NOT NULL
);
CREATE INDEX agent_capacity_slots_sweep_idx
  ON agent_capacity_slots (hold_expires_at) WHERE hold_expires_at IS NOT NULL;
```
- Acquire one free slot inside the offer tx with `FOR UPDATE SKIP LOCKED LIMIT 1`;
  no free row → abort offer + release route lock.
- **accept MUST clear `hold_expires_at = NULL`** (agy BLOCK) so the leaked-hold
  sweeper can't rip a live call's slot; release on terminal sets reservation_id NULL.
- **accept tx re-checks `WHERE reservation_id = :resID`** on the slot (agy HIGH:
  sweeper-vs-accept) so an accept of a swept offer fails closed.

### Matcher
- Availability-driven pull (agy fixes applied): lock **only the route** —
  `FOR UPDATE OF rr SKIP LOCKED` (bare `FOR UPDATE` would lock the joined
  queue/membership rows → cross-agent deadlock); the same statement sets
  `status='offering'` (or `next_match_at = now()+Δ`) so a second puller can't grab
  the same route before the offer tx (agy HIGH: pull-to-offer race).
- Eligibility skills: denormalize `route_requests.required_skills text[]` + GIN
  index and filter `required_skills <@ agent_skills` (agy HIGH: no per-row plpgsql).
- Score (bypass math fixed — aging/SLA must be able to cross a priority band, agy
  HIGH): `effective = priority*W_p + queue_weight*10 + aging + sla_boost +
  idle_bonus`, with `W_p` and the aging cap chosen so a sufficiently-aged lower
  band can overtake (e.g. uncapped aging or `W_p < max_aging`). Avoid sorting the
  whole backlog on a `now()` function (agy HIGH): precompute a `deadline_score` /
  order by indexable `priority DESC, created_at ASC` and refine in app, or
  materialize aging on the sweep tick.
- No double-offer: capacity slots + partial-unique
  `reservations_one_active_per_route` (offered..wrap_up); for voice add
  `reservations_one_live_offer_per_agent_channel` — but **NOT for chat=N** (slots
  enforce it) and **only if blast/simultaneous ringing stays out of scope** (agy MED).

### Presence lease (Redis primary, DB audit)
`presence:{org}:{agent}:{session} = {lease_token,gateway_id,status,channels}` TTL
30s, heartbeat 10s, mid-offer reconnect grace 45–60s; `agent_sessions:{org}:{agent}`
set for lookup. DB `agent_presence_audit` for last-known/revocation. Offer tx:
after locking route+slot, confirm Redis lease + `lease_token`, insert reservation
with `agent_session_id`+`lease_token`; every accept/reject/complete must match the
reservation `lease_token` (fences stale commands). **Redis outage → fail closed**
for live offers (PART C).

### WS protocol idempotency
Envelope `{id,type,org_id,agent_id,session_id,seq,sent_at,payload,ack}`. Inbound
commands deduped by `PRIMARY KEY (org_id, agent_id, client_msg_id)` in
`ws_command_dedupe` (stores the result for replay). Outbound persisted to
`agent_outbox` with a monotonic per-agent `server_seq`; reconnect `hello{last_server_seq}`
replays `server_seq > last`, and current `offered` reservations are re-derived so an
offer survives outbox retention expiry.

### route_decisions (auditability)
One row per decision: `decision_type` (interaction_offer|availability_pull|retry|
sweep), `decision_version`, `matcher_instance`, `channel`, `queue_id`,
`selected_agent_id`, `selected_slot_no`, `outcome` (offered|no_candidate|
capacity_lost|lease_lost|route_lost), `reason`, and JSONB `eligibility/candidates/
excluded/ranking/capacity_snapshot`; indexed by `(org_id, route_request_id,
created_at DESC)` and `(org_id, selected_agent_id, …)`.

### New data model (folded into the single pre-release migration)
Tables: `agent_capacity_slots`, `agent_presence_audit`, `ws_command_dedupe`,
`agent_outbox`, `route_decisions` (+ optional `agent_sessions`,
`agent_routing_state` if RONA/Missed isn't modeled on the agent row).
Columns: `reservations.{channel,agent_session_id,lease_token,version,
offer_expires_at,wrapup_expires_at,assignment_handle,adapter_correlation_id,
terminal_reason}`; `route_requests.{channel,queue_id,priority,waiting_since,
next_match_at,match_attempt_seq,active_reservation_id,required_skills}`;
`agents.{routing_state,routing_state_expires_at}`.

### Decisions pinned (round-2 PART C)
- Capacity = slot rows (final). RONA recovery = cooldown TTL + next explicit Ready
  clears it. Offer-timeout authority = the continuation worker. Multi-session =
  one routable primary session per agent. Redis outage = fail closed. Adapter
  dedupe key = `(org_id, adapter_correlation_id, adapter_event_id)`. Sweep cadence
  ~2s, batch ≤100 routes/100 agents per org. Clock = Postgres `now()` (never trust
  gateway clocks). Retention policies for dedupe/outbox/decisions = TBD-at-impl,
  bounded. Reservation state machine + race precedence (accept vs caller_abandoned,
  timeout vs accept, disconnect vs reject) = define explicitly in Wave 1.
- Fairness must be tested with seeded wait times + concurrent workers (not just
  unit tests).
