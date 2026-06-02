# Routing Runtime Delta

## ADDED Requirements

### Requirement: Published flow execution

Open Routing SHALL execute route requests against the currently published flow version for the relevant channel or routing entry point.

#### Scenario: Routing with a published flow

- **WHEN** a route request arrives for an org and channel with a published flow
- **THEN** the runtime executes the compiled plan for that published version
- **AND** the execution result records the flow version used

#### Scenario: Missing published flow

- **WHEN** no published flow can handle the route request
- **THEN** the runtime returns a typed routing failure
- **AND** no reservation is created

### Requirement: Deterministic execution inputs

Runtime execution SHALL be deterministic for a fixed flow version, catalog snapshot, interaction input, and initial state snapshot.

#### Scenario: Capturing an execution snapshot

- **WHEN** runtime starts a route request or simulation
- **THEN** it captures the relevant catalog and initial state values used by the execution
- **AND** replay does not depend on mutable current catalog rows or Redis cache contents

#### Scenario: Replaying the same inputs

- **WHEN** the same pinned inputs are replayed
- **THEN** the runtime produces the same selected path and routing outcome
- **AND** non-deterministic effect outputs are read from recorded or mocked values

### Requirement: Reservation lifecycle

Open Routing SHALL model routing assignment as a reservation lifecycle rather than a one-shot selected agent.

#### Scenario: Offering a reservation

- **WHEN** runtime selects an eligible agent
- **THEN** it creates an offered reservation for that agent and interaction
- **AND** the offer can become accepted, rejected, timeout, cancelled, or completed

#### Scenario: Reservation timeout

- **WHEN** an offered reservation times out
- **THEN** runtime records the timeout
- **AND** the flow can create another offered reservation attempt or follow a fallback path according to the published plan

#### Scenario: Test double accepts an offer

- **WHEN** the v0.2 test-double adapter or simulator accepts an offered reservation
- **THEN** runtime records the accepted state
- **AND** agent-state transition handling can move the agent to Engaged

### Requirement: Runtime event traceability

Open Routing SHALL persist runtime events and expose trace records that explain routing outcomes.

#### Scenario: Inspecting a routing trace

- **WHEN** a user opens a trace for a route request
- **THEN** the trace shows ordered steps, inputs, outputs, selected catalog records, effect status, and timing
- **AND** the trace remains scoped to the request org

### Requirement: Adapter command boundary

Runtime SHALL output adapter commands without owning channel session execution or media.

#### Scenario: Routing to a channel system

- **WHEN** runtime decides an interaction should be offered through a channel adapter
- **THEN** it emits an adapter command with normalized routing context
- **AND** the external adapter remains responsible for channel execution and session lifecycle

### Requirement: Route-decision performance target

Open Routing SHALL measure route-decision latency and report whether v0.2 runtime paths stay within the project p95 target.

#### Scenario: Running runtime performance gate

- **WHEN** runtime test fixtures execute route decisions under the supported v0.2 node set
- **THEN** p95 route-decision latency is reported
- **AND** regressions beyond the agreed budget are flagged for review before the metric becomes a blocking gate
