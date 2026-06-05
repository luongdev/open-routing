# Tasks: v0.3 Live Routing Engine

Waves land runnable slices. Cross-AI plan review (`plan-review.md`) reordered the
channel-neutral assignment contract BEFORE lifecycle wiring and added RONA /
caller-abandonment / lease-presence / WS-idempotency / fairness / durable-sweep /
decision-trace requirements.

## Wave 0 — v0.2 hardening (prerequisite)

- [x] Land the FIX-NOW batch from the v0.2 review (`v0.2-review-findings.md`):
      run_seq fence, Lua sandbox, region-suspension reject, branch-port validation,
      accept guards, worker poison-pill, snapshot enabled filter, etc. The live
      lifecycle builds on this base.

## Wave 1 — Channel-neutral assignment contract (review: before lifecycle)

- [x] Channel-neutral assignment-event contract: Deliver(interaction, agent) ->
      opaque handle + events accepted -> connecting -> established ->
      (completed | failed | disconnected | caller_abandoned), idempotent with
      correlation. Handle opaque to the engine (chat/email fit later).
- [x] Mock voice adapter driving that lifecycle without media.
- [x] Tests: adapter event mapping incl. caller_abandoned + failed.

## Wave 2 — Realtime transport (WS gateway)

- [x] WS message envelope (reuse v0.2 canonical event shape) with per-message ids;
      accept/reject/complete carry reservation version/lease token.
- [x] Gateway in cmd/api: scoped agent identity + org binding + session revocation;
      heartbeat/ping; graceful close. Gateway only transports; commands run through
      runtime-owned transactional handlers (review HIGH).
- [x] Outbound push from the durable reservation row; reconnect replays an
      in-flight offer; ack + server-side dedupe so replay can't double-apply.
- [ ] Backpressure + per-org connection caps; structured logs + metrics.
- [x] Tests: connect, offer push, accept upstream, reconnect-replays-offer,
      duplicate-command-deduped, queue/skill authz on command.

## Wave 3 — Lease presence & DB-solid capacity

- [x] Lease/TTL presence: gateway heartbeat renews a Redis-primary connection
      lease; expiry = offline; DB (agent_sessions) is the audit trail. (Redis
      rebuild-on-gateway-start deferred per D2 — documented in internal/presence.)
- [x] Capacity as per-(agent,channel) slot rows (voice=1, chat=N) held/released
      transactionally (FOR UPDATE SKIP LOCKED) — NOT the capacity=1 partial-unique
      index. Acquire (offer), confirm (accept), release (terminal), sweep (pending
      expired), reconcile (terminal orphan). NOTE: relaxing the existing
      ux_reservations_agent_active index to per-(agent,channel) for true chat=N
      end-to-end is deferred to W4 — v0.3 live channel is voice (cap=1).
- [x] LiveCandidateSource (eligible AND Ready AND leased-connected AND
      under-capacity) as a hint; buildSnapshot kept for simulation; NO silent
      live→sim fallback; presence error parks the route (review BLOCK/HIGH).
- [x] Tests: disconnected removes offerability, capacity gating, concurrent
      acquire = exactly N (chat), sweep/reconcile, accept-confirm/complete-release.
- [ ] FOLLOW-UP (W5): confirmed-slot reclaim for an agent who crashed mid-call
      (reservation stuck 'accepted') — presence-loss → RONA/abandonment, built on
      the W3 lease. Not reclaimed by the W3 terminal-orphan reconcile by design.

## Wave 4 — The matcher (the engine)

- [ ] Interaction-driven offer on the live pool; final offer tx re-checks
      connected/Ready/skills/queue/capacity against authoritative leased state.
- [ ] Availability-driven pull: on Ready / capacity-freed, pull the best-ranked
      waiting route from a served queue; a failed insert returns it to the head.
- [ ] Durable reconciliation sweep as the primary trigger (every few seconds);
      LISTEN/NOTIFY or pub/sub is only a wake-up hint (review MED).
- [ ] Concurrency: FOR UPDATE SKIP LOCKED + route run-lock CAS + capacity slot
      lock; no double-assign across replicas.
- [ ] Fair ranking: priority with aging, queue weight, bounded max-priority bypass,
      deterministic tie-break.
- [ ] route_decision trace: eligibility inputs, considered + excluded (with
      reasons), ranking values, capacity snapshot, decision version.
- [ ] Tests: no double-assign, capacity never exceeded, aging prevents starvation,
      queue-pull priority correct.

## Wave 5 — Real reservation lifecycle (live signals)

- [ ] Agent WS accept/reject + adapter events -> v0.2 reservation transitions +
      route resume, each conditional on reservation state + version + lease token
      (keep HTTP test-double for sim/CI).
- [ ] Offer timeout via the durable continuation worker; timeout sets the agent
      non-routable (RONA), never an immediate re-offer to the same agent.
- [ ] Disconnect: mid-offer grace window (reconnect resumes) before reject;
      mid-handling reassign policy is adapter-driven with a recorded terminal
      reason; caller_abandoned tears down the reservation + route immediately.
- [ ] Runtime-owned Ready -> Engaged -> WrapUp; configurable max ACW timer via the
      worker frees capacity even without manual completion.
- [ ] Tests: full live lifecycle, timeout/RONA, blip-reconnect keeps offer,
      caller abandon, wrap-up cap.

## Wave 6 — Reference client + ops + closure

- [ ] Minimal reference agent client to prove transport + lifecycle end to end.
- [ ] Live ops view: queue depth, oldest-waiting / SLA age, occupancy by channel,
      offers/sec, accept latency, abandon/timeout rate, reject reasons,
      stuck-reservation alerts, live route-decision p95.
- [ ] Protocol contract tests: golden WS schema + reconnect / duplicate-command /
      timeout / stale-offer fixtures.
- [ ] OpenAPI/WS schema + migrations (fold into the single pre-release migration);
      sqlc + Go/TS regen drift gates; org-scoping on every new table/query/session.
- [ ] Backend + frontend test gates; live-engine e2e smoke (extends
      scripts/e2e_v02_runtime.sh).
- [ ] Cross-AI peer review of the full v0.3 diff; fix findings.
- [ ] Merge accepted behavior into openspec/specs/ and archive this change.

## Deferred to v0.4

- [ ] Real LiveKit/SIP media behind the channel-neutral assignment contract.
- [ ] Full agent desktop; outbound campaigns; multi-region.
