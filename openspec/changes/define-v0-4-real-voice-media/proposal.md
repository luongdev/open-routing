# Proposal: v0.4 Real Voice Media (LiveKit/SIP bridge)

## Why

v0.3 shipped the live routing engine end to end — WS transport, live presence +
capacity, the bidirectional matcher, the reservation lifecycle, and a
**channel-neutral assignment contract** — but the only adapter is the in-process
`MockVoice`: it performs NO media. The engine can match a live interaction to a
connected agent and run the full match → offer → accept → handle → complete path,
but no real call audio is ever bridged.

v0.4 makes a real voice call flow through the engine: a customer on a phone/SIP
leg is bridged to the matched agent over LiveKit, the call's real lifecycle
(answered, established, hung-up, failed) drives the reservation, and a dropped
call is re-matched to another agent instead of being abandoned.

Crucially, this stays inside the product boundary: **Open Routing bridges to a
media stack; it does not own media.** The adapter is a thin out-of-process bridge
to the existing `opentts` stack (livekit-server, sip, tts, phone); Open Routing
holds routing state, not call audio.

## The architectural shift (why this is non-trivial)

v0.3's `MockVoice` is in-process and synchronous: `Deliver` is a direct call and
the `EventSink` is a Go interface the engine implements. A REAL adapter is a
separate process/service, which breaks three assumptions the mock hid:

1. **Delivery must be durable + out-of-process.** Today `Deliver` runs post-commit
   in-process; a crash between accept-commit and Deliver silently loses the
   delivery (documented v0.3 gap). v0.4 needs a **durable delivery outbox** (model
   it on the existing `agent_outbox`): accept commits a delivery command row, a
   worker drains it to the bridge, survives crashes/replays.
2. **Events come back over the network, not a Go call.** The real bridge reports
   `delivered → connecting → established → terminal` via an **inbound HTTP webhook**
   that maps onto `EventSink.OnAssignmentEvent`, idempotent by `CorrelationID`.
   This replaces the in-process sink for real adapters.
3. **Release is cross-process.** v0.3's reclaim already calls `Release`, but the
   in-process mock keeps handles per-process so it no-ops across api/runtime. With
   LiveKit the handle is the room id, so `Release` (delete room / hang up leg)
   works cross-process — once delivery is durable and the handle is persisted.

## Boundary (to confirm in the cross-AI discussion)

- **Bridge, don't own.** The voice adapter is an out-of-process service that asks
  livekit-server/sip to bridge the caller leg and the agent leg, and reports
  lifecycle. Open Routing stores the room/handle id and routing state only.
- **Voice first, contract unchanged.** The `internal/adapter.ChannelAdapter`
  contract from v0.3 does not change shape; v0.4 implements it for real and adds
  the durable-delivery + webhook plumbing AROUND it. Chat/email still fit later.
- **Agent desktop: minimal, not a product.** The agent needs a real surface to be
  rung, answer, and see the active call. PROPOSED: a minimal agent console in
  `web/packages/ui` (presence toggle, incoming-offer accept/reject, active-call
  panel, wrap-up) reusing the WS client — NOT a full agent-desktop product. (Open
  question: is the desktop in v0.4 or its own v0.5? See design.md.)

## What ships in v0.4

1. **Durable delivery outbox** — accept commits a `delivery_command` row; a worker
   drains it to the adapter and persists the returned handle on the reservation;
   at-least-once with idempotent Deliver (dedupe by reservation/correlation).
2. **Real voice adapter service** — bridges caller↔agent over LiveKit/SIP, returns
   a room handle, and drives the call. Out-of-process; talks the same contract.
3. **Inbound assignment-event webhook** — an authenticated, idempotent endpoint
   the bridge calls to report lifecycle; maps to `OnAssignmentEvent`. Org-scoped,
   correlation-deduped, terminal-final.
4. **Automatic mid-call reassignment** — when an established call drops (agent
   disconnect / fail) the interaction is re-queued for re-matching to another
   agent (preserving caller context), not abandoned — completing the v0.3 reclaim
   path. Bounded retries + a give-up fallback.
5. **Minimal agent console** — a real (small) agent UI to actually take a call.

## Explicitly out of scope (v0.5+)

- A full agent-desktop product (CRM panels, dispositions, supervisor barge).
- Chat/email real adapters (the contract already admits them; voice proves it).
- Outbound/proactive dialing; multi-region active/active; recording/QA.
- Owning media, call control, or session execution (permanent boundary).

## Success criteria

- A real inbound call (SIP/phone) is matched to a connected agent, bridged over
  LiveKit, and both parties hear each other; on hang-up the reservation completes
  and the agent goes to WrapUp — with no test-double endpoint involved.
- A crash between accept and delivery does NOT lose the call: the durable delivery
  outbox redelivers on restart, exactly-once-observably.
- An agent dropping mid-call causes the interaction to be re-matched to another
  available agent within the SLA, not abandoned.
- The whole real-media path is observable on the v0.3 event/trace spine and the
  Live Ops view (real established/abandon/reassign events).
