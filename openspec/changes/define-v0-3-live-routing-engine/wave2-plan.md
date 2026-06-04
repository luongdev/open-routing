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
