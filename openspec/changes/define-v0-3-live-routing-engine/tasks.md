# Tasks: v0.3 Live Routing Engine

Waves are ordered so each lands a runnable slice. The reference agent client and
the mock voice adapter let the whole engine run end to end before any real media.

## Wave 1 — Realtime transport (WS gateway)

- [ ] Define the WS message envelope (reuse the v0.2 canonical runtime event
      shape): presence, offer, accept/reject, interaction events, wrap-up.
- [ ] WS gateway in `cmd/api`: authenticate (trusted-host org model), per-agent
      socket, heartbeat/ping, graceful close.
- [ ] Outbound push: deliver an offer to the agent's socket from the durable
      reservation row; on reconnect, replay any in-flight offer (no lost offers).
- [ ] Inbound: accept/reject/presence messages map to existing handlers.
- [ ] Backpressure + per-org connection caps; structured logs + metrics.
- [ ] Tests: connect, offer push, accept upstream, reconnect-replays-offer,
      disconnect-mid-offer = reject.

## Wave 2 — Live presence & capacity

- [ ] Presence dimension on agent state: connected? since when? Redis hot path +
      DB source of truth; rebuild Redis from DB on gateway start.
- [ ] Per-channel capacity model (voice=1, chat=N) configurable per agent/queue;
      active-assignment count tracked.
- [ ] `LiveCandidateSource`: eligible ∧ available ∧ connected ∧ under-capacity,
      queried at offer time (behind the existing candidate interface).
- [ ] Keep `buildSnapshot` for simulation (determinism unchanged).
- [ ] Tests: presence transitions, capacity gating, live pool excludes
      disconnected/at-capacity agents.

## Wave 3 — The matcher (the engine)

- [ ] Interaction-driven path on the live pool (extend the v0.2 offerer).
- [ ] Availability-driven pull: on Ready / capacity-freed, pull the highest-
      priority waiting route from a served queue and offer it.
- [ ] Notify path (Postgres LISTEN/NOTIFY or Redis pub/sub) so the matcher reacts
      without polling; fall back to a periodic sweep.
- [ ] Concurrency: `FOR UPDATE SKIP LOCKED` over waiting routes + the route
      run-lock CAS + per-agent partial-unique reservation; guarded capacity insert.
- [ ] Priority + skills + longest-idle tie-break; fairness check.
- [ ] Tests: no double-assign under concurrent replicas, capacity never exceeded,
      queue-pull picks correct priority, two interactions ↛ one one-capacity agent.

## Wave 4 — Real reservation lifecycle (live signals)

- [ ] Map agent WS accept/reject and adapter answered/ended onto the v0.2
      reservation transitions + route resume (keep HTTP test-double for sim/CI).
- [ ] Offer timeout via the existing durable continuation worker.
- [ ] Disconnect handling: mid-offer = reject + re-offer; mid-handling = grace
      window then reassign.
- [ ] Runtime-owned Ready→Engaged→WrapUp driven by real accept/complete.
- [ ] Tests: full live lifecycle, timeout, disconnect/reassign, wrap-up expiry.

## Wave 5 — Adapter contract + mock voice + ops

- [ ] `ChannelAdapter` interface (Deliver / OnAdapterEvent / Release), shaped for
      LiveKit/SIP (a bridge = a media room).
- [ ] Mock voice adapter satisfying the contract (answered on accept, ended on
      complete) so the engine runs without media.
- [ ] Reference agent client (minimal) to prove transport + lifecycle.
- [ ] Live ops view: queue depth, agent occupancy, live route-decision p95,
      offers/sec.
- [ ] Tests: engine end-to-end through the mock adapter + reference client.

## Wave 6 — Contracts, CI, closure

- [ ] OpenAPI/WS schema + migrations (fold into the single pre-release migration
      per the project convention until release); sqlc + Go/TS regen drift gates.
- [ ] Org-scoping/tenant isolation on every new table, query, and WS session.
- [ ] Backend + frontend test gates; live-engine e2e smoke (extends
      `scripts/e2e_v02_runtime.sh`).
- [ ] Cross-AI peer review of the full v0.3 diff; fix findings.
- [ ] Merge accepted behavior into `openspec/specs/` and archive this change.

## Deferred to v0.4

- [ ] Real LiveKit/SIP media behind the `ChannelAdapter` contract.
- [ ] Full agent desktop; outbound campaigns; multi-region.
