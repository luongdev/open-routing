# Delivery Foundation Delta

## ADDED Requirements

### Requirement: Realtime push transport

Open Routing SHALL provide a realtime push transport for runtime events, built on
the v0.2 outbox-first foundation: events are persisted before they are pushed, so
a dropped connection never loses a decision.

#### Scenario: Pushing from the durable source

- **WHEN** a runtime event must reach a connected agent or adapter
- **THEN** it is persisted to the event/outbox foundation first
- **AND** then pushed over the realtime transport

#### Scenario: Recovering after a dropped connection

- **WHEN** a transport connection drops and reconnects
- **THEN** in-flight state (e.g. an outstanding offer) is recovered from the
  durable rows, not from socket memory

### Requirement: Stateless, horizontally scalable gateway

The realtime gateway SHALL be horizontally scalable: connection state lives in a
shared store (Redis) and durable rows, so any gateway replica can serve any agent
and replicas can be added or removed without losing offers.

#### Scenario: Scaling gateways

- **WHEN** gateway replicas are added or removed under load
- **THEN** agents reconnect to any replica and resume presence + in-flight offers
- **AND** no offer is lost or double-delivered as a result
