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

## v0.4 implementation cross-review (W0–W2) — outcome

codex + gemini reviewed the W0–W2 code. Fixed:
- BLOCK (codex): delivery finalize is now lease-fenced (MarkDelivery* fenced on
  claimed_by + status) and atomic (mark + handle-bind in one tx) — a stale worker
  whose lease was re-claimed gets 0 rows and releases its orphan room instead of
  clobbering the peer's handle (multi-replica safety).
- BLOCK (codex): adapter.Assignment now carries IdempotencyKey (the delivery-attempt
  id) so a real adapter dedupes Deliver to one media session across retries.
- HIGH (codex): a missing adapter for the channel now fails + tears down (was
  silently marked delivered → media-less stranded call).
- HIGH (codex) / MED (gemini): finalize errors propagate (retry) instead of being
  swallowed; cap-failure tears down BEFORE marking failed (no 'failed'-command /
  'accepted'-route split-brain); lost-claim + reservation-terminal orphans Release.
- MED (codex): webhook uses MaxBytesReader (413, no HMAC-over-truncated-prefix) +
  rejects future-dated timestamps beyond a small skew.

Deferred (documented, not bugs):
- FSM wiring + CorrelationID dedup + rejected→reoffer / disconnected→reassign
  (codex HIGH) → **Wave 4** (reassignment is the wave that consumes the FSM's
  ActionReassign/ActionReoffer; until then the v0.3 terminal-teardown + state-based
  idempotency hold).
- Per-adapter/org webhook secret (codex MED) → **Wave 3** hardening when the real,
  externally-operated adapter lands (one global secret is fine for the in-repo mock;
  the sink's handle ownership-fence already bounds blast radius).

## W4 cross-review (codex + gemini) — outcome

Fixed:
- BLOCK (codex): ReassignRouteForMatch no longer resurrects a terminal route —
  reassign is gated on a non-terminal (live) route in reassignRoute.
- BLOCK (codex): an inline-offered route (no matcher metadata → empty
  required_skills, un-bumped attempt seq) is NOT re-matched (it would ring
  unqualified agents / collide the reservation attempt unique) — gated on
  required_skills present; such a drop abandons instead.
- HIGH (both): reassignRoute locks the route FIRST (LockRouteForReclaim) — same
  top-down order as teardown/reclaim, no AB-BA deadlock. This also makes the
  read-count → ReassignRouteForMatch hop-cap check atomic (MED).
- HIGH (both): EnqueueRouteForMatch's excluded_agent_ids now includes 'cancelled'
  so a re-parked reassigned route won't re-ring the dropped agent.
- LOW (codex): route.reassigned reports reassign_count+1; or_matcher_reassigns_total
  counts only an actual re-queue, not a give-up.

Deferred to Wave 3 (real-media flow), documented:
- HIGH (codex): a reassigned route's resume_cursor still points at the post-accept
  call/wait node, so (a) the re-matched agent resumes that node rather than
  re-entering the reservation/bridge node, and (b) an SLA-expired reassign resumes
  no_candidate at a wait node (taken as elapsed) instead of the reservation's
  no_candidate fallback. Correct re-bridge needs a real-media "bridge" flow node +
  a cursor reset to it — that lands with W3. Until then reassignment frees the slot
  + re-queues + re-matches (slot accounting + a fresh offer are correct); the mock
  has no media to re-bridge.
- MED (codex): a handle rebind between the sink's pre-tx handle-match and
  ReassignStaleReservation's state fence — minor; the real fix is the cursor/
  bridge rework above.
- Inline-route reassignment (skill re-derivation from the graph) → W3.

## W4-delta + W5-backend cross-review (round 2) — outcome

gemini: clean. codex (stricter):
- HIGH (fixed): the reassign gate used `len(required_skills)>0`, but empty
  required_skills is a VALID matcher state (skill-less queue) — it would wrongly
  abandon a skill-less queued route. Re-gated on `match_attempt_seq > 0` (the
  matcher's CommitMatchOffer bumps it; an inline offer never does) — the precise
  "came through the matcher" signal.
- LOW (fixed): ReassignRouteForMatch SQL now also excludes 'completed' (matches the
  Go gate / terminal set); the ReassignRouteForMatch ErrNoRows path no longer
  claims `reassigned`.
- BLOCK (acknowledged, W3): the re-matched agent resumes the route's post-accept
  cursor (the wait/call node), not the reservation node, so it doesn't re-consume
  "accepted" — correct re-entry needs a reservation-node cursor reset, which lands
  with the W3 real-media bridge flow. Pinned by TestReassign_RematchesToAnotherAgent
  (proves the re-queue → re-offer-to-a-new-agent works; the full replacement-accept-
  through-the-call is the W3 piece). Until then, reassignment frees the slot +
  re-queues + re-offers (correct routing/accounting); the mock has no media to
  re-bridge.
- MED (acknowledged): GET agents/{id}/reservations is org-scoped (consistent with
  the trusted-host X-Org-Id model) but not self-agent-scoped — any org caller can
  read any agent's offers. Per-agent identity enforcement is the same browser-auth
  decision deferred with the WS browser path.
