# Design Notes & Open Questions — v0.4 Real Voice Media

Reference patterns to reuse (do NOT reinvent):
- Durable delivery → model on `internal/db` `agent_outbox` (Append/Read/LockSeq).
- Idempotency → the v0.3 command dedupe (`request_hash` + CorrelationID).
- Teardown/reclaim → v0.3 `teardownRouteTx` + `reclaimAbandonedCall`; reassignment
  is reclaim WITHOUT the abandon — re-queue instead.
- Webhook auth → mirror the trusted-host org model; events carry org + correlation.

## Identity mapping refinement (during Wave 0 build)

The 4 agreed identities mostly already exist — only ONE new table is needed:
- `interaction_id` → existing `route_requests.id` (the interaction spine).
- `reservation_attempt_id` → existing `reservations.id` (each offer is already a
  distinct row with `attempt`; a reassignment hop = a NEW reservations row).
- `media_session_id` → existing `reservations.adapter_handle` (the room handle).
- `delivery_attempt_id` → **NEW**: the `delivery_command` durable outbox row id
  (the idempotency key the adapter dedupes Deliver on).

So Wave 0's DB work shrinks to: confirm this mapping + add ownership-fence columns
where the webhook needs them; the genuinely new schema (the `delivery_command`
outbox) lands in Wave 1. Folded into migration 000001 (user decision 2026-06-05:
keep single-migration purity, accept the cluster DB reset + re-seed on v0.4 deploy).

## Resolved by cross-AI discussion round 1 (see discussion.md)

- **Q2** caller leg → external stack owns the trunk; OR receives an inbound ref.
- **Q3** idempotency key → `delivery_attempt_id`, not reservation_id.
- **Q4** reassignment → new reservation per hop; interaction carries context; 2–3 hops.
- **Q5** webhook → HMAC + replay window + ownership fence (org+attempt+handle).
- **Q6** deployment → in-repo `cmd/voiceadapter`, run as a separate container.
- **Q7** events → adapter is the anti-corruption layer; emits coarse contract events.
- **Q8** terminal race → optimistic `UPDATE ... WHERE state != terminal`, handle-fenced.
- **NEW Wave 0** → formalize identities + state-transition table + late-event
  quarantine BEFORE the outbox (both flagged identity confusion as the #1 risk).
- **NEW** adapter health/backpressure + a chaos wave.
- **Q1 (console scope)** → DECIDED by user: minimal console IS in v0.4 (codex's
  position) so a human can answer a real call end-to-end; full desktop is v0.5+.

## Open questions (original seed — ALL resolved above; kept for rationale/context)

### Q1 — Agent desktop: in v0.4 or v0.5?
The minimal console (Wave 5) needs a LiveKit *client* audio integration, which is
real frontend work. Option A: include a minimal console in v0.4 (you can't take a
real call without one). Option B: v0.4 ships the server-side bridge + a CLI/audio
test harness; the console is v0.5. Trade-off: A makes v0.4 demoable end-to-end but
widens scope; B keeps v0.4 backend-pure but leaves "no human can answer" until v0.5.

### Q2 — Caller-leg ownership / where the call starts
Does Open Routing originate the inbound caller leg (own a SIP trunk), or does the
existing opentts/phone stack own the trunk and merely NOTIFY Open Routing of an
inbound interaction (which then routes + bridges the agent leg)? The product
boundary says we bridge, not own media — favoring the latter. This decides whether
v0.4 needs SIP trunk integration at all, or just LiveKit room + agent-leg bridging.

### Q3 — Delivery outbox exactly-once semantics
Deliver is at-least-once (the worker may redeliver after a crash). Idempotency key
= reservation id → the adapter must map a redelivered Deliver to the SAME room
(create-or-get). Events stay exactly-once-observable via CorrelationID. Is
reservation-id-as-idempotency-key sufficient, or do we need a delivery attempt/
epoch to handle a reservation that was reclaimed + reassigned (new room, same
reservation? or new reservation per hop)?

### Q4 — Reassignment: new reservation per hop, or reused?
On a mid-call drop, does the re-queued interaction get a NEW reservation (clean
lifecycle, clear audit) for the next agent, or reuse the old one? New-per-hop is
cleaner for the decision spine + capacity accounting; it interacts with Q3's
idempotency key. How much caller context (interaction_input, partial notes,
elapsed handling time, a "transferred" flag) must survive a hop, and how many hops
before the flow's no_agent fallback?

### Q5 — Webhook security + LiveKit credentials
How does the bridge authenticate to the engine webhook (shared secret header /
mTLS / signed payload) and how does the engine/adapter authenticate to LiveKit
(API key/secret, room tokens)? Where do secrets live (k8s Secret) and how is the
webhook protected from spoofed terminal events (a forged caller_abandoned could
tear down a live call — correlation + handle ownership fence needed)?

### Q6 — Adapter deployment shape
Is the voice adapter a new `cmd/voiceadapter` in this repo (one more Deployment,
shares the module + contract types), or a fully separate service? In-repo eases
the contract coupling + CI; separate eases independent scaling. Given the bridge
is stateless (room state lives in LiveKit), a new in-repo cmd seems right — confirm.

### Q7 — Mapping LiveKit room events to the contract
LiveKit emits room/participant webhooks (participant_joined/left, room_finished).
Mapping: agent participant joined = established; agent left before caller =
disconnected (→ reassign); caller left = caller_abandoned (→ teardown); room
empty/finished = completed-or-failed depending on prior state. Edge: who emits
`established` — the adapter on both-joined, or LiveKit? Need a deterministic mapping
table.

### Q8 — Capacity + room lifecycle race
A reclaim frees the slot when an agent vanishes; if the agent's LiveKit leg
actually drops a beat later, we must not double-teardown. The v0.3 terminal-final
fence (CorrelationID) should cover it, but the cross-process timing (reclaim sweep
vs late LiveKit room_finished webhook) needs an explicit "first terminal wins"
proof.

## Non-goals reaffirmed
Open Routing never holds call audio, owns a PSTN trunk's media, or becomes an
agent-desktop product. v0.4 bridges and observes; LiveKit/sip hold the media.
