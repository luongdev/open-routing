# Agent Connectivity Delta

## ADDED Requirements

### Requirement: Realtime agent transport

Open Routing SHALL provide a realtime (WebSocket) transport over which a connected
agent receives offers and interaction events and sends presence, accept, and
reject — without polling.

#### Scenario: Receiving an offer in real time

- **WHEN** the engine offers an interaction to a connected agent
- **THEN** the offer is pushed to the agent's socket
- **AND** the agent can accept or reject over the same socket

#### Scenario: Authenticated, org-scoped session

- **WHEN** an agent client opens a socket
- **THEN** the session is authenticated under the trusted-host org model
- **AND** it can only observe offers and events for its own org and agent

### Requirement: Offer durability across reconnect

An in-flight offer SHALL survive a transport reconnect: the offer lives on the
durable reservation row, not only in the socket.

#### Scenario: Reconnect replays an in-flight offer

- **WHEN** an agent reconnects while an offer is still outstanding
- **THEN** the gateway replays the in-flight offer from the reservation row
- **AND** no offer is lost or duplicated

#### Scenario: Disconnect during an offer

- **WHEN** an agent disconnects while an offer is outstanding
- **THEN** the offer is released (treated as a reject) so the engine can re-offer

### Requirement: Idempotent agent commands

Agent commands (accept, reject, complete) SHALL be idempotent under reconnect
replay: each carries a message id and the reservation version/lease token, is
acknowledged, and is server-side deduplicated so a replayed command cannot
double-apply.

#### Scenario: Replayed accept after reconnect

- **WHEN** an agent reconnects and the client replays an accept it already sent
- **THEN** the server recognizes the duplicate (version/lease token) and applies it
  at most once
- **AND** the agent receives an ack either way

### Requirement: Lease-based live presence and capacity

Open Routing SHALL track connectivity as a TTL-renewed lease (heartbeat), treat a
Ready agent whose lease has expired as not offerable, and gate offers on free
per-channel capacity. The lease (not a DB flag) is authoritative for liveness so a
crashed gateway does not leave agents falsely connected.

#### Scenario: Crashed gateway expires offerability

- **WHEN** a gateway crashes without a clean disconnect
- **THEN** the affected agents' presence leases expire by TTL
- **AND** the engine stops offering to them until they reconnect and renew

#### Scenario: Presence gates offerability

- **WHEN** an agent is Ready in the state machine but holds no live lease
- **THEN** the engine does not offer to that agent

#### Scenario: Per-channel capacity

- **WHEN** an agent's active assignments on a channel reach its capacity
- **THEN** the agent is excluded from that channel's pool until capacity frees
- **AND** capacity is configurable per agent/queue (e.g. voice = 1, chat = N)
