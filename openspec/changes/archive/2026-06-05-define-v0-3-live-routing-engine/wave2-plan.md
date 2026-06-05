# Wave 2 Plan — Realtime transport (WS gateway)

Builds on design.md D1/D7/D8 + the round-2 blueprint (ws_command_dedupe,
agent_outbox, envelope, reconnect replay) and on W1's `internal/adapter` contract.

## Scope / boundary

W2 delivers the **transport only**: a WebSocket gateway that pushes offers down
and carries agent commands up, durably and idempotently. It does NOT decide *who*
to offer to (matcher = W4) and does NOT make an agent offerable (lease presence =
W3). W2's "presence" is just the socket lifecycle (connect/heartbeat/close); the
*offerability* lease is W3. Commands reuse the existing v0.2 reservation handlers
(AcceptReservation/RejectReservation/CompleteReservation) — the gateway only
transports; it does not implement transitions.

## Architecture

```
agent client ──WS──> ws gateway (cmd/api, chi route /v1/agent/ws)
                         │  inbound: hello, heartbeat, accept/reject/complete
                         ▼
                   command dispatcher ──> runtime-owned txn handlers (v2 Accept/Reject/Complete)
                         ▲                         │ (these already park/resume via the worker)
                         │ outbound: offer.created, assignment events, ack
                   agent_outbox (server_seq) + reservations(state='offered')
```

- The gateway is a **separate chi route**, not a spec/strict-handler operation
  (WS upgrade doesn't fit oapi-codegen). Registered in `server.NewMux` like
  `/metrics`, but BEHIND auth (it must resolve org + agent). Reuses the OrgContext
  org resolution; adds agent identity.
- WS library: **coder/websocket** (`github.com/coder/websocket`) — context-aware
  read/write, simple close semantics, std-net based. (Decision D-W2-1; alt:
  gorilla/websocket.)
- One goroutine-pair per connection (read loop + write pump); a per-connection
  bounded send channel for backpressure.

## Message envelope (round-2)

Every frame is JSON:
```json
{ "id":"uuid", "type":"reservation.accept", "org_id":"…", "agent_id":"…",
  "session_id":"…", "seq":42, "sent_at":"…", "payload":{}, "ack":{"reply_to":"uuid"} }
```
- Client→server `type`: `hello` (`{last_server_seq}`), `heartbeat`,
  `reservation.accept|reject|complete` (payload carries `reservation_id`,
  `version`, `lease_token`).
- Server→client `type`: `welcome`, `offer.created`
  (`{reservation_id, route_request_id, version, lease_token, expires_at,
  interaction}`), `assignment.event` (from the W1 adapter lifecycle), `ack`
  (`{reply_to, status}`), `error`.

## Inbound: commands are idempotent + runtime-owned

- Each client command carries a client `id`. Dedup table (round-2):
  ```sql
  CREATE TABLE ws_command_dedupe (
    org_id uuid NOT NULL, agent_id uuid NOT NULL, client_msg_id uuid NOT NULL,
    command_type text NOT NULL, result jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, agent_id, client_msg_id)
  );
  ```
  On receipt: in one tx, INSERT the dedupe row (ON CONFLICT DO NOTHING); if the
  row already existed, return its stored `result` (the ack) WITHOUT re-running the
  command. Otherwise run the v2 handler, store its result, ack.
- The command maps to the existing AcceptReservation/RejectReservation/
  CompleteReservation (which already hold the route lock + resume via the worker).
  The gateway calls them through an in-process dispatcher (NOT HTTP) carrying the
  org/agent context. Every transition stays conditional on reservation
  state+version (the v0.2 guards + the hardening H3 check).

## Outbound: durable push + reconnect replay

- Offers and assignment events are written to `agent_outbox` with a monotonic
  per-agent `server_seq`, THEN pushed (outbox-first, design D delivery):
  ```sql
  CREATE TABLE agent_outbox (
    org_id uuid NOT NULL, agent_id uuid NOT NULL,
    server_seq bigint GENERATED ALWAYS AS IDENTITY,
    type text NOT NULL, reservation_id uuid, payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, agent_id, server_seq)
  );
  ```
- On `hello{last_server_seq}` (reconnect), the gateway replays
  `server_seq > last_server_seq` in order, AND re-derives any current
  `reservations.state='offered'` for the agent (so an in-flight offer survives
  even if outbox retention expired) — no offer lost, no dup (dedupe + seq).
- Where do offers get enqueued? In W2 there is no matcher yet, so offer push is
  exercised via the existing reservation flow / Route Tester and a test seam; W4
  wires the matcher to enqueue.

## Auth (round-2 LOW, brought forward minimally)

- Connect authenticates a scoped agent identity bound to its org (extend the
  trusted-host model: the host vouches `org_id` + `agent_id`). `session_id`
  minted at connect. Per-command: re-check the session is still valid (revocation
  list / a `terminated` flag) before applying. Full queue/skill authz lands with
  the matcher (W4).

## Backpressure & ops

- Per-connection bounded send buffer; on overflow, close with a policy code
  (client reconnects + replays from `server_seq`). Per-org connection cap.
- Structured logs (org_id, agent_id, session_id, seq) + metrics (connections,
  offers pushed, command latency, dedupe hits, reconnect replays).

## Data model (folded into migration 000001 per the single-migration rule)
`ws_command_dedupe`, `agent_outbox` (+ `agent_sessions` if a DB-visible session
inventory / revocation is needed). Org-scoped; sqlc + Go regen drift gates.

## Tests
- connect → welcome; offer enqueued → pushed; accept upstream → reservation
  accepted (in-process dispatcher); reconnect with last_server_seq replays the
  in-flight offer exactly once; duplicate command (same client id) deduped (one
  apply, ack replayed); backpressure overflow closes cleanly; per-org cap.

## Open decisions
- D-W2-1 WS lib: coder/websocket (recommended) vs gorilla.
- D-W2-2 in-process command dispatch: call the flowrt Endpoints methods directly
  vs a thin command interface (prefer a small CommandHandler interface so the
  gateway doesn't import the full handler surface).
- D-W2-3 agent identity source in the trusted-host model (header vs signed token).
- D-W2-4 outbox retention / pruning cadence.

## Review revisions (codex W2 plan review — corrected architecture)

The naive plan above has real bugs; the implementation follows these instead:

- **Outbox is the ONLY delivery source; commit-then-relay** (BLOCK): never push
  from producer memory. A relay SELECTs committed `agent_outbox` rows ordered by
  seq and pushes; NOTIFY is only a wake-up hint. Fixes the push-before-commit hole.
- **Per-agent seq under a lock, NOT `GENERATED IDENTITY`** (BLOCK): IDENTITY has a
  commit-order gap (seq 10 assigned to tx A, 11 to tx B; B commits+relays first;
  reconnect at 11 skips 10 forever). Allocate `server_seq` per agent under a
  per-agent advisory/row lock inside the state-change tx; the relay only reads
  committed rows in seq order, so a gap means "wait", never "skip".
- **One command transaction owns dedupe + transition** (BLOCK): a
  `ReservationCommandService` runs `ws_command_dedupe` insert + the reservation
  transition + result store in ONE tx (used by BOTH the HTTP handlers and WS — the
  WS layer must NOT wrap tx-owning HTTP handlers). Dedupe row gets
  `status` + nullable `result` + `request_hash`; a same `client_msg_id` with a
  different payload is rejected; concurrent duplicates lock/wait on the row.
- **Identity from the authenticated connection, never the envelope** (HIGH):
  ignore client-sent org_id/agent_id/session_id (spoofable in a trusted-host
  model); derive them at connect.
- **agent_sessions mandatory** (HIGH): `terminated_at`/`expires_at`; per-command
  revocation check; revoked session pauses outbound + rejects commands.
- **Command CAS carries org+agent+reservation state+version+offer-token** (HIGH):
  an agent cannot accept another agent's offered reservation by guessing the id.
- **Offer token, not lease_token, in W2** (MED): the capacity/presence lease is
  W3; W2 uses a per-offer token owned by reservation state.
- **Stable outbox event keys** + `UNIQUE(org_id, agent_id, event_key)` (HIGH) so a
  reconnect re-derivation of an offered reservation can't duplicate an outbox row.
- **ack semantics**: ack is derived from the dedupe result (retry-safe), not a
  separate racy frame.
- **Backpressure**: advance "delivered seq" only AFTER a successful Write; on
  overflow close `1013` and let reconnect replay. Protocol hardening: max frame,
  heartbeat timeout, hello-first, protocol version, read/write deadlines.
- **Retention**: define a replay window; if `hello.last_server_seq` < retention
  floor, send a `reset` marker (client re-syncs) rather than silently dropping.

Testability (no W3/W4 needed): seed an offered reservation via the v0.2 flow,
enqueue outbox rows via a test producer seam, and assert transport / replay /
command dispatch + the two race tests (concurrent duplicate command; reversed
outbox commit order).
