# Proposal: v0.3 Live Routing Engine

## Why

v0.2 shipped flow authoring, a deterministic runtime, and the reservation
lifecycle — but reservations resolve via **test-double / simulator signals**, the
candidate pool is a **point-in-time snapshot** pinned at route start, and channel
effects are **record/mock**. There is no live agent connectivity and no
availability-driven distribution. In other words: we can design and simulate
routing, but we cannot actually route a live interaction to a real, connected
agent.

v0.3 turns the runtime into a **live routing engine**: real agents connect over a
realtime transport, the engine matches interactions to who is actually available
right now (in both directions — interaction→agent and agent-Ready→queue-pull),
and the reservation lifecycle is driven by real agent + adapter signals.

## Boundary (decided with the user)

- **Engine-first, media abstract.** v0.3 delivers the engine end to end: WS
  transport, live presence + capacity, the bidirectional matcher, and real
  accept/handle/complete from a connected agent client. The channel **media**
  (actual call audio / chat transport) stays behind a pluggable adapter contract.
- **Voice (LiveKit/SIP) is the first concrete adapter target.** The adapter
  contract is shaped around the existing `opentts` stack (livekit-server, sip,
  tts, phone). v0.3 ships a **mock voice adapter** that satisfies the contract;
  **v0.4 wires the real LiveKit/SIP media** behind the same contract.
- The v0.2 test-double accept/reject/complete endpoints **remain** for
  simulation, CI, and the Route Tester — live signals are additive, not a
  replacement of the deterministic test path.

## What ships in v0.3

1. **Realtime transport** — a WebSocket gateway: agent presence up, offers down,
   accept/reject up, interaction + wrap-up events both ways. Built on the v0.2
   outbox-first event foundation (durable first, then pushed).
2. **Live presence & capacity** — real-time Ready/Engaged/capacity per agent
   (Redis hot path + DB source of truth), per-channel capacity (voice = 1,
   chat = N). The candidate pool is computed **live at offer time**, replacing the
   snapshot for live routes (simulation keeps the snapshot for determinism).
3. **The matcher** — the actual routing engine: interaction-driven offers AND
   availability-driven queue pulls, with priority, required skills, longest-idle
   tie-break, and capacity — with no double-assignment under concurrency and
   multiple runtime replicas.
4. **Real reservation lifecycle** — accept/reject/timeout/complete from real
   agent actions and adapter events, reusing the v0.2 route-lock + durable
   continuation machinery.
5. **Mock voice adapter + live ops view** — the adapter contract with a mock
   voice implementation, plus an operations surface (queue depth, agent
   occupancy, live route-decision p95, offers/sec).

## Explicitly out of scope (v0.4+)

- Real LiveKit/SIP media bridging (the adapter is contract + mock in v0.3).
- Outbound/proactive campaigns; multi-region active/active.
- An agent desktop product (v0.3 ships a minimal reference agent client to prove
  the transport + lifecycle, not a full desktop).

## Success criteria

- A reference agent client connects over WS, is offered a live interaction the
  engine matched to it, accepts, handles, and completes — with no polling and no
  test-double endpoint involved.
- An idle Ready agent is pulled the highest-priority waiting interaction from a
  queue it serves.
- Under concurrent offers/replicas, an interaction is never assigned to two
  agents and an agent never exceeds its channel capacity.
- The whole live path is observable (presence, queue depth, p95) and replayable
  for post-hoc analysis via the v0.2 trace/event spine.
