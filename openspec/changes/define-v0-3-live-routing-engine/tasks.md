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
- [x] Backpressure + per-org connection caps; structured logs + metrics.
      (Backpressure: enqueue drops a slow consumer → reconnect+replay. Caps:
      WS_MAX_CONNS_PER_ORG per gateway instance, 429 before upgrade. Metrics:
      internal/metrics registry on /metrics — ws conns/overflow/commands +
      matcher offers/reclaims.)
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
- [x] Confirmed-slot reclaim for an agent who vanished mid-call (reservation
      stuck 'accepted'): the matcher tick reclaims when the agent is unseen on any
      session past reclaimGrace (90s, aligned to presence TTL) — grace on
      last_seen_at so a blip+reconnect is NOT reclaimed. Reservation-scoped (a
      route can be terminal while its accepted reservation still holds the slot):
      cancel reservation (agent_lost) → free slot → agent out of Engaged → tear the
      route down if still live. Auto-reassign to another agent is adapter/media-
      coupled → v0.4.

## Wave 4 — The matcher (the engine)

- [x] Interaction-driven offer on the live pool; final offer tx re-checks
      connected/capacity against authoritative leased state. (Stage 2; lease_token
      echo-back fence deferred to W5 with the agent-WS offer frame.)
- [x] Availability-driven pull: on Ready / capacity-freed, pull the best-ranked
      waiting route from a served queue; a failed insert returns it to the head.
      (Stage 3: RunMatchCycle/tryOfferToAgent, token-fenced; capacity-at-cap →
      ReturnRouteToQueue.)
- [x] Durable reconciliation sweep as the primary trigger (every few seconds);
      LISTEN/NOTIFY or pub/sub is only a wake-up hint (review MED). (Stage 5:
      RunMatcher on the cmd/runtime tick — stale-offering + SLA-deadline sweeps +
      per-org pull. NOTIFY wake-hint deferred.)
- [x] Concurrency: FOR UPDATE SKIP LOCKED + route run-lock CAS + capacity slot
      lock; no double-assign across replicas. (Stage 2a/3; cross-AI reviewed.)
- [x] Fair ranking: priority with aging, deterministic tie-break. (Stage 3:
      SQL-computed score, aging crosses bands — anti-starvation test. Queue weight
      deferred until agent↔queue membership lands.)
- [x] route_decision trace: selected/excluded agent, outcome, ranking weights,
      decision version. (Stage 3: InsertRouteDecision on every offer + pre-offer
      failure.)
- [x] Tests: no double-assign, capacity never exceeded, aging prevents starvation,
      queue-pull priority correct. (Stage 3/5: testcontainer + -race.)
- [x] Stage 4 — Lease fencing (lease_token + agent_session_id bound at offer;
      the WS command path fences on the echoed lease_token) + the durable
      agent_outbox offer frame that delivers it + RONA agent_routing_state cooldown
      on timeout (last_ready_at Ready-race fence baked into MarkAgentMissed). NOTE:
      the fence is conservative until the agent Ready transition writes
      last_ready_at (W5, additive); the short self-healing TTL covers it meanwhile.

## Wave 5 — Real reservation lifecycle (live signals)

- [x] Agent WS accept/reject + adapter events -> reservation transitions + route
      resume, each conditional on reservation state + lease token (HTTP test-double
      kept for sim/CI). (W4 Stage 4: lease fence + durable offer frame; the relay
      delivers lease_token, the agent echoes it.)
- [x] Offer timeout via the durable continuation worker; timeout sets the agent
      non-routable (RONA), never an immediate re-offer to the same agent. (W4
      Stage 4 + the last_ready_at write that activates the Ready-race fence.)
- [x] Disconnect / caller_abandoned: caller_abandoned tears down the reservation +
      route immediately (AbandonRouteRequest). Mid-offer reconnect resumes via the
      durable outbox frame + session-independent lease_token (no reject on a blip).
      Mid-handling reassign is adapter-driven (deferred with real media, v0.4).
- [x] Runtime-owned Ready -> Engaged -> WrapUp; max ACW timer via the worker frees
      capacity even without manual completion. (accept→Engaged, complete→WrapUp,
      wrapup_expiry continuation→Ready+capacity-free — shipped in W3/W4.)
- [x] Tests: live lifecycle (accept/complete), timeout/RONA + Ready-race fence,
      caller abandon, wrap-up cap. (blip-reconnect-keeps-offer is architectural —
      a full WS e2e harness is deferred to W6 protocol contract tests.)

## Wave 6 — Reference client + ops + closure

- [x] Minimal reference agent client to prove transport + lifecycle end to end.
      (cmd/refagent: dials the WS gateway, hello/heartbeat, auto-accepts offers
      echoing the lease_token, optional complete. Gateway test proves the
      offer-frame lease round-trips relay→client→command.)
- [x] Mock voice adapter wired to the engine: accept hands the assignment to the
      channel adapter (Deliver, handle bound on the reservation); the engine's
      EventSink maps adapter-originated terminals (caller_abandoned/disconnect/
      fail/reject) onto reservation+route teardown, engine-initiated ones are
      no-ops. Release on abandon. cmd/api wires MockVoice for "voice".
- [x] Live ops view: GET /routing/stats backend + an <or-ops-view> frontend
      dashboard (Live Ops nav) — queue depth, offering, outstanding offers,
      oldest-wait SLA age, held slots, and a recent-route-requests feed, polling
      2s. Rich rates (offers/sec, accept latency, reject reasons, route-decision
      p95) remain additive.
- [ ] Protocol contract tests: golden WS schema + reconnect / duplicate-command /
      timeout / stale-offer fixtures.
- [ ] OpenAPI/WS schema + migrations (fold into the single pre-release migration);
      sqlc + Go/TS regen drift gates; org-scoping on every new table/query/session.
- [~] Backend + frontend test gates; live-engine e2e smoke. CI: a black-box
      integration test (TestE2E_MatcherPullToComplete) drives matcher pull → offer
      → lease-fenced accept → resume → complete → slot freed → WrapUp with real
      Endpoints. Manual: scripts/e2e_v03_runtime.sh seeds + runs cmd/refagent over
      WS against a live stack (needs MATCHER_ENABLED). Frontend gates unchanged.
- [ ] Cross-AI peer review of the full v0.3 diff; fix findings.
- [ ] Merge accepted behavior into openspec/specs/ and archive this change.

## Deferred to v0.4

- [ ] Real LiveKit/SIP media behind the channel-neutral assignment contract.
- [ ] Full agent desktop; outbound campaigns; multi-region.
