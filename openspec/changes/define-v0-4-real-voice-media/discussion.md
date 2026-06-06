# Cross-AI Discussion Room — v0.4 Real Voice Media

Participants: Claude (author), codex, gemini. Round 1 = independent positions on
scope, the 8 open questions, biggest risk, and gaps. Below: consensus, the one
real disagreement, and the resulting plan changes.

## Consensus (both codex + gemini agree)

- **Q2 — caller leg**: the existing opentts/phone stack OWNS the trunk; Open
  Routing receives an inbound-interaction reference, routes it, and tells the
  adapter to bridge the agent leg. Owning a trunk would make us a PBX — boundary
  violation. → Plan: add an explicit **inbound-interaction contract** from the
  phone stack (codex's missing piece).
- **Q3 — delivery idempotency**: `reservation_id` ALONE is insufficient. Use an
  explicit **`delivery_attempt_id`** (a.k.a. delivery_command_id) as the adapter
  idempotency key; keep a stable `interaction_id` across hops. A reclaimed/
  reassigned call must never "create-or-get" the stale room.
- **Q4 — reassignment**: a **NEW reservation per hop**, strictly. Reservations are
  ephemeral per-agent offers; reusing them corrupts audit, AHT metrics, and
  capacity. The **interaction** carries context (caller-leg ref, input, elapsed,
  `transfer_count`, `previous_agent`, transfer flag). Cap hops low (2–3) → flow
  `no_agent` fallback.
- **Q5 — webhook security**: **HMAC-SHA256** signed payloads, per-env/org secret,
  timestamp/nonce replay window; mTLS is overkill for v0.4. codex adds: terminal
  events accepted ONLY if `org + interaction/attempt + media_session/handle` match
  current ownership (correlation alone is too weak — a forged caller_abandoned
  could kill a live call).
- **Q6 — deployment**: in-repo **`cmd/voiceadapter`** (keeps contract + impl in
  sync, CI-testable) but run as a SEPARATE binary/container to enforce the
  out-of-process boundary. Split to its own repo later only under scaling pressure.
- **Q7 — event mapping**: the adapter is an **anti-corruption layer**. LiveKit
  participant semantics NEVER leak into routing; the adapter buffers raw room
  events and emits coarse contract events (both legs joined → `established`; agent
  left while caller remains → disconnected; caller left → caller_abandoned; room
  finished interpreted from prior state).
- **Q8 — terminal race**: **first-terminal-wins via optimistic concurrency** —
  whichever of {engine reclaim sweep, adapter webhook} acts first runs
  `UPDATE ... WHERE state != terminal`; 0 rows ⇒ already handled. Fence the
  teardown to the specific handle/media_session so a late `room_finished` from an
  OLD room can't disturb a new reassignment hop.

## Shared biggest risk (both, same root)

**Lifecycle identity confusion / non-monotonic events.** The plan leaned on v0.3's
reservation+correlation fences stretching across real media, retries, late
webhooks, and reassignment — they can't, unless v0.4 FIRST formalizes distinct
identities (interaction vs reservation_attempt vs delivery_command vs
media_session) and a state machine that tolerates out-of-order / skipped /
duplicate events without leaking capacity or deadlocking.

## Shared missing pieces

- codex: explicit inbound-interaction contract; a deterministic assignment/media
  **state-transition table**; a **late-event quarantine** rule — all BEFORE LiveKit.
- gemini: **adapter health / backpressure** — if `cmd/voiceadapter` is dead, the
  outbox worker must not endlessly claim+fail; short-circuit routing (fail/queue)
  via a circuit-breaker or adapter-registration. Plus a dedicated **chaos/edge**
  wave (worker crash mid-flight, out-of-order webhooks, adapter killed post-Deliver).

## The one real disagreement — Q1 (agent console in v0.4?)

- **codex → Option A (include a minimal console in v0.4)**: "you can't take a real
  call without one"; a CLI harness isn't a milestone. Keep it tiny.
- **gemini → Option B (defer console to v0.5)**: browser WebRTC has different
  failure modes (autoplay, mic perm/network); prove the backend + bridge with a
  headless/auto-answer client first; keep v0.4 backend-pure.

This is a product-scope judgment (demoable-end-to-end vs backend-pure), not a
technical deadlock — escalated to the user. **DECIDED: codex's Option A — the
minimal console ships in v0.4** so a human can answer a real routed call end to
end; the full desktop stays v0.5+.

## Resulting plan changes (folded in)

1. **Add Wave 0 — Identity & state model** (before the outbox): formalize
   interaction / reservation_attempt / delivery_command / media_session ids; the
   inbound-interaction contract; the assignment/media state-transition table; the
   late-event quarantine rule.
2. Wave 1: delivery idempotency key = `delivery_attempt_id` (not reservation_id).
3. Wave 2: webhook = HMAC + ownership fence; out-of-order/duplicate tolerance is a
   first-class requirement; add **adapter health/backpressure** (circuit-breaker /
   registration → short-circuit when the adapter is down).
4. Wave 4: new reservation per hop; interaction-carried context; bounded hops.
5. Wave 6 (or new Wave): explicit **chaos/edge** suite (worker crash, out-of-order
   webhooks, adapter-killed-post-Deliver, late room_finished vs reassignment).
6. Q1 (console scope) DECIDED: minimal console ships in v0.4 (Wave 5 stays).
