# Delivery Foundation Delta

## ADDED Requirements

### Requirement: Runtime/control-plane boundary

Open Routing SHALL separate runtime execution concerns from catalog and authoring control-plane concerns through explicit package and API boundaries.

#### Scenario: Executing a route request

- **WHEN** runtime handles a route request
- **THEN** it uses a runtime-facing contract for published plans, catalog snapshots, reservations, and events
- **AND** it does not directly depend on UI component behavior

#### Scenario: Running runtime as its own process

- **WHEN** v0.2 deploys Open Routing
- **THEN** live route execution runs in a separate `cmd/runtime` process from the `cmd/api` control plane
- **AND** both share one `internal/runtime` library and the database
- **AND** simulation runs in-process inside `cmd/api` against the same library without an inter-process call

#### Scenario: Claiming time-driven work safely

- **WHEN** the runtime worker processes due continuations, reservation timeouts, or WrapUp expiry
- **THEN** it claims due rows transactionally with `FOR UPDATE SKIP LOCKED`
- **AND** multiple runtime replicas can run the worker without double-firing the same row

### Requirement: Canonical runtime event envelope

Runtime and adapter-facing events SHALL use a canonical envelope that includes org, event identity, source, type, timestamp, correlation, and payload fields.

#### Scenario: Persisting a runtime event

- **WHEN** runtime records a route step, reservation update, adapter command, or state transition
- **THEN** the event is stored with the canonical envelope
- **AND** downstream trace or projection reads can correlate related events

#### Scenario: Avoiding audit-contract creep

- **WHEN** v0.2 records runtime events
- **THEN** those events support debugging, trace, and replay behavior
- **AND** they do not define the org-facing audit retention contract

### Requirement: Outbox-first delivery

Open Routing SHALL persist runtime events before exposing them to external delivery mechanisms.

#### Scenario: Preparing event fan-out

- **WHEN** a runtime event is created
- **THEN** it is written to the database/outbox foundation first
- **AND** later realtime or event-bus bridges can consume from that durable source
