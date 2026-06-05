# Tasks: v0.4 Real Voice Media

Waves land runnable slices, each proven with the in-process/remote mock BEFORE
real LiveKit, so the engine plumbing is validated without media infrastructure.

## Wave 1 — Durable delivery outbox (prove with the mock)

- [ ] `delivery_command` durable rows (model on `agent_outbox`): accept commits a
      delivery intent in the SAME tx as the reservation flip.
- [ ] Delivery drain worker (cmd/runtime tick): claim due delivery commands
      (FOR UPDATE SKIP LOCKED), call adapter `Deliver`, persist the returned handle
      on the reservation, mark the command done. At-least-once + idempotent.
- [ ] Idempotent Deliver: a redelivered command for the same reservation maps to
      the same handle (no double room). Dedupe key = reservation id.
- [ ] Route the EXISTING in-process MockVoice through the outbox so the mechanism
      is proven before real media; crash-between-accept-and-deliver redelivers.
- [ ] Tests: accept→durable command→drain→handle persisted; crash/replay redelivers
      once; capacity/teardown still correct.

## Wave 2 — Inbound assignment-event webhook

- [ ] Authenticated, org-scoped HTTP endpoint the bridge calls to report lifecycle
      events; maps to `EventSink.OnAssignmentEvent`, idempotent by CorrelationID,
      terminal-final (later events for a terminal handle ignored).
- [ ] A "remote mock" adapter that drives the lifecycle OVER the webhook (so CI/e2e
      never needs real LiveKit) — the network-shaped twin of MockVoice.
- [ ] Tests: golden webhook schema; duplicate correlation deduped; out-of-order /
      post-terminal events rejected; cross-org event rejected.

## Wave 3 — Real voice adapter service (LiveKit/SIP bridge)

- [ ] Out-of-process adapter that on Deliver creates a LiveKit room, bridges the
      caller leg and the agent leg, returns the room handle, and reports lifecycle
      via the Wave 2 webhook; Release tears the room down.
- [ ] Map LiveKit/SIP room events → assignment events (participant joined =
      established; agent left = disconnected; caller left = caller_abandoned).
- [ ] Caller-leg boundary: the opentts/phone/sip stack owns the inbound trunk; the
      adapter bridges, it does not own the PSTN trunk (confirm in design).
- [ ] Tests/harness: against a local livekit-server in a soak env (not CI).

## Wave 4 — Automatic mid-call reassignment

- [ ] On an established call dropping (agent disconnect/fail), re-queue the
      interaction for re-matching to another agent with preserved caller context,
      instead of abandoning (completes the v0.3 reclaim path).
- [ ] Bounded reassign hops + a give-up fallback (flow no_agent terminal); a
      "transferred" marker on the new offer; audit each hop on the decision spine.
- [ ] Tests: drop → reassign to a second agent; exhausted hops → fallback.

## Wave 5 — Minimal agent console

- [ ] A small Lit agent surface in `web/packages/ui`: presence toggle, incoming-
      offer accept/reject, active-call panel + LiveKit audio client, wrap-up.
- [ ] Reuse the v0.3 WS client + the embed/admin shell; NOT a full desktop.
- [ ] Tests: offer renders, accept joins the room, complete ends + wrap-up.

## Wave 6 — Closure

- [ ] Real-media e2e in a soak env (inbound call → bridged → hang-up → WrapUp).
- [ ] Load/soak: many concurrent bridged calls; reassignment under churn.
- [ ] OpenAPI/webhook schema + migrations folded; drift gates; org-scoping audit.
- [ ] Full cross-AI review; fix findings.
- [ ] Merge behavior into `openspec/specs/` (channel-adapters, agent-connectivity,
      delivery-foundation, a new agent-console capability) + archive.

## Deferred to v0.5+

- [ ] Full agent-desktop product (dispositions, supervisor barge, CRM panels).
- [ ] Chat/email real adapters; outbound dialing; recording/QA; multi-region.
