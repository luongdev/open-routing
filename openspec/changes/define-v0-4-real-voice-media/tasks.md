# Tasks: v0.4 Real Voice Media

Waves land runnable slices, each proven with the in-process/remote mock BEFORE
real LiveKit, so the engine plumbing is validated without media infrastructure.

Cross-AI round 1 (codex + gemini) reshaped this: a Wave 0 to formalize identities
+ the state machine BEFORE any outbox/media work (both flagged "lifecycle identity
confusion" as the #1 risk), explicit adapter health/backpressure, and a chaos
suite. See discussion.md.

## Wave 0 — Identity & state model (foundation; before any media)

- [ ] Formalize the distinct identities: `interaction_id` (stable across hops),
      `reservation_attempt_id` (new per hop), `delivery_attempt_id` (the outbox
      idempotency key), `media_session_id`/room handle. No code reuses one for
      another.
- [ ] Inbound-interaction contract: the opentts/phone stack notifies Open Routing
      of an inbound caller leg (reference, not ownership) → routing begins.
- [x] Deterministic assignment/media state-transition table (the engine-side
      anti-corruption FSM) + a late-event QUARANTINE rule: out-of-order, skipped,
      duplicated, and post-terminal events are tolerated and never leak capacity or
      deadlock. Shipped: `internal/assignment` — pure `Decide(state,event)`,
      monotonic progress, first-terminal-wins, missed-event tolerant.
- [x] Tests: table-driven (mirrors the table) + a 20k-trial property/fuzz proving
      the invariants (monotonic, ≤1 terminal, sticky-terminal, totality).

## Wave 1 — Durable delivery outbox (prove with the mock)

- [x] `delivery_commands` durable rows (modeled on `continuations` claim/lease):
      table in 000001, registered as a tenant table. UNIQUE(org,reservation) makes
      the producer idempotent.
- [x] Delivery drain worker `DrainDeliveries` (for the cmd/runtime tick): claim due
      commands (FOR UPDATE SKIP LOCKED + claim lease), call adapter `Deliver`,
      persist the handle on the reservation, mark delivered; Deliver fault → mark
      failed + teardown. At-least-once + idempotent.
- [x] Idempotent: dedupe at the producer on (org, reservation); the command id IS
      the `delivery_attempt_id` the adapter dedupes Deliver on. A reassignment hop =
      new reservation → new command (never the stale room).
- [x] Tests: enqueue→drain→handle bound + command delivered; idempotent producer;
      second drain is a no-op.
- [x] Route the live accept path through the outbox, gated by DELIVERY_OUTBOX_ENABLED
      (default off → deployed in-process Deliver unchanged until flipped). Both
      accept paths (WS command + HTTP test-double) enqueue in the accept tx when on;
      DrainDeliveries wired into the cmd/runtime tick. Test: outbox-on accept
      enqueues + defers to the drain (no in-process handle), drain binds it.

## Wave 2 — Inbound assignment-event webhook

- [x] Inbound assignment-event webhook (`internal/adapterwebhook`, bare route
      POST /v1/adapter/assignment-events) → `EventSink.OnAssignmentEvent`. The sink
      already carries the ownership fence (handle-match + state) + terminal-final
      idempotency from v0.3.
- [x] Webhook auth: HMAC-SHA256 over "timestamp.body" + replay window; ownership
      fence enforced by the sink (terminal applied only when the event handle
      matches the reservation's current bound handle + still live). Route mounted
      only when ADAPTER_WEBHOOK_SECRET set.
- [x] Adapter health / backpressure: the drain retries a failing Deliver (a down
      adapter) up to maxDeliveryAttempts via the claim lease — bounded, and it does
      NOT abandon the call on a transient blip — only tears down past the cap.
- [~] Remote-mock adapter that drives the lifecycle OVER the webhook → FOLDED INTO
      Wave 3: it is the network-shaped sibling of the real LiveKit adapter and is
      cleanest built alongside it (both are out-of-process webhook clients).
- [x] Tests: webhook valid/bad-sig/tampered/stale/bad-json/sink-error (DB-free);
      drain retry-cap (pending until cap → failed). Cross-org/wrong-handle no-op is
      the sink's v0.3-tested fence.

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
- [ ] Chaos suite: outbox worker crash mid-flight; adapter killed immediately after
      Deliver; out-of-order / duplicate LiveKit webhooks; a late `room_finished`
      from an OLD room during a new reassignment hop (first-terminal-wins, fenced
      to the handle).
- [ ] Load/soak: many concurrent bridged calls; reassignment under churn.
- [ ] OpenAPI/webhook schema + migrations folded; drift gates; org-scoping audit.
- [ ] Full cross-AI review; fix findings.
- [ ] Merge behavior into `openspec/specs/` (channel-adapters, agent-connectivity,
      delivery-foundation, a new agent-console capability) + archive.

## Deferred to v0.5+

- [ ] Full agent-desktop product (dispositions, supervisor barge, CRM panels).
- [ ] Chat/email real adapters; outbound dialing; recording/QA; multi-region.
