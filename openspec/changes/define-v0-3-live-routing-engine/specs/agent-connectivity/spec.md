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

### Requirement: Live presence and capacity

Open Routing SHALL track, in real time, whether an agent is connected and how much
channel capacity is free, and SHALL treat a Ready agent with no live connection as
not offerable.

#### Scenario: Presence gates offerability

- **WHEN** an agent is Ready in the state machine but has no live socket
- **THEN** the engine does not offer to that agent

#### Scenario: Per-channel capacity

- **WHEN** an agent's active assignments on a channel reach its capacity
- **THEN** the agent is excluded from that channel's pool until capacity frees
- **AND** capacity is configurable per agent/queue (e.g. voice = 1, chat = N)
