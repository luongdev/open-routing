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

### D2 — Presence: Redis hot path, DB source of truth
Agent presence/capacity is written to DB (authoritative, survives restart) and
mirrored to Redis for the matcher's live pool query (sub-ms). The v0.2 agent
state machine stays the source of truth for status transitions; presence adds a
**connection** dimension (a Ready agent with no live socket is NOT offerable).
Capacity is per channel: `voice=1`, `chat=N` (configurable per agent/queue).

### D3 — Live candidate pool replaces the snapshot for LIVE routes
v0.2 pins `buildSnapshot` at route start for determinism. For live routes the
matcher queries the **current** eligible+available+connected+under-capacity pool
at offer time. Simulation keeps the snapshot (determinism/replay unchanged). This
is a `LiveCandidateSource` behind the existing candidate interface — the executor
contract does not change.

### D4 — The matcher is bidirectional and lock-safe
Two triggers, one assignment invariant:
- **Interaction-driven**: a new/È resumed route needs an agent → offer the top
  live candidate (v0.2 offerer, but live pool).
- **Availability-driven** (the new half): an agent transitions to Ready (or frees
  capacity) → the matcher pulls the highest-priority waiting route from a queue
  the agent serves and offers it.
Concurrency: assignment is serialized per-route by the existing `route_requests`
run-lock (CAS) and per-agent by a `reservations` partial-unique index (one
active offered/accepted per agent). The availability-driven pull additionally
takes a short advisory lock per (queue) or uses `SELECT … FOR UPDATE SKIP LOCKED`
over waiting routes so two matcher replicas can't pull the same route. Capacity
is enforced by a guarded conditional insert (offer only if active count <
capacity).

### D5 — Reservation lifecycle from real signals (test-double preserved)
Accept/reject from the agent WS and answered/ended from the adapter map onto the
SAME v0.2 reservation transitions (`AcceptReservation`/`RejectReservation`/
`CompleteReservation`) and route resume. The HTTP test-double endpoints stay for
sim/CI/Route-Tester. Offer timeout still fires via the durable continuation
worker. A disconnect mid-offer = reject (freed immediately); a disconnect
mid-handling = a grace window then reassign (configurable).

### D6 — Adapter contract (voice-first, mock in v0.3)
`ChannelAdapter` interface: `Deliver(interaction, agent) → handle`,
`OnAdapterEvent(answered|ended|failed)`, `Release(handle)`. The contract is shaped
for LiveKit/SIP (a "bridge" = a media room the agent + caller join). v0.3 ships a
**mock voice adapter** that immediately reports `answered` on accept and `ended`
on complete, so the full engine path runs without media. v0.4 implements the same
interface against livekit-server/sip.

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
  LOCKED + the route run-lock CAS (proven in v0.2).
- **Capacity races**: two interactions offered to a one-capacity agent.
  Mitigation: the per-agent partial-unique reservation index + guarded count.
- **Presence drift**: Redis vs DB divergence. Mitigation: DB authoritative, Redis
  TTL'd + rebuilt from DB on gateway start; the matcher treats Redis as a hint and
  confirms capacity at the guarded insert.
