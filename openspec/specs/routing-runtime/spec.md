# Routing Runtime Specification

## Purpose

This specification captures the v0.2 deterministic routing runtime: published-flow execution, candidate selection, reservations, durable continuations, route requests, runtime events, and traces.

## Requirements


### Requirement: Published flow execution

Open Routing SHALL execute route requests against the currently published flow version for the relevant channel or routing entry point.

#### Scenario: Routing with a published flow

- **WHEN** a route request arrives for an org and channel with a published flow
- **THEN** the runtime executes the compiled plan for that published version
- **AND** the execution result records the flow version used

#### Scenario: Resolving a flow entry point

- **WHEN** a route request includes an org, channel, and entry code
- **THEN** the runtime resolves exactly one active published flow version for that binding
- **AND** zero active matches return a typed missing-published-flow failure before execution begins
- **AND** multiple active matches return a typed binding-invariant failure before execution begins

#### Scenario: Missing published flow

- **WHEN** no published flow can handle the route request
- **THEN** the runtime returns a typed routing failure
- **AND** no reservation is created

#### Scenario: Missing catalog reference during execution

- **WHEN** a published flow version references a queue, skill, channel, adapter, or break reason that is no longer available in the request org
- **THEN** runtime records a typed missing-catalog-reference failure in the trace
- **AND** no reservation is created from the unavailable reference
- **AND** the flow follows a fallback path only when the published plan explicitly defines one for that failure

#### Scenario: Rolling back during an in-flight route

- **WHEN** a route request has already pinned a published flow version
- **THEN** rollback does not interrupt that in-flight execution
- **AND** only later route requests use the newly active published version

### Requirement: Deterministic execution inputs

Runtime execution SHALL be deterministic for a fixed flow version, catalog snapshot, interaction input, initial state snapshot, and clock input.

#### Scenario: Capturing an execution snapshot

- **WHEN** runtime starts a route request or simulation
- **THEN** it captures the relevant catalog and initial state values used by the execution
- **AND** replay does not depend on mutable current catalog rows or Redis cache contents
- **AND** live continuation fire times and virtual-clock advances used by the execution are part of the replay inputs
- **AND** external reservation outcome signals are recorded with logical timestamps when they influence execution

#### Scenario: Replaying the same inputs

- **WHEN** the same pinned inputs are replayed
- **THEN** the runtime produces the same logical path and routing outcome
- **AND** non-deterministic effect outputs are read from recorded or mocked values
- **AND** generated identifiers and wall-clock timestamps are not part of trace equality

#### Scenario: Replaying time-driven nodes

- **WHEN** replay executes a wait node or reservation timeout path
- **THEN** elapsed time is driven by the pinned virtual clock
- **AND** recorded continuation fire times are replayed instead of recomputed from wall-clock time
- **AND** wall-clock timing does not change the replayed trace

#### Scenario: Selecting candidates deterministically

- **WHEN** multiple eligible agents are tied by the configured routing criteria
- **THEN** runtime applies a stable total ordering with an explicit final tie-break key
- **AND** the trace records the ranking inputs and final selected candidate

### Requirement: Skill-based candidate selection

Open Routing SHALL select route candidates by skill eligibility and proficiency, with a deterministic tie-break, before offering a reservation.

#### Scenario: Selecting an eligible agent by skill

- **WHEN** runtime evaluates candidates for a route request
- **THEN** it keeps only agents whose skills satisfy the flow's required skills and who are in a routable state
- **AND** it ranks the remaining agents by proficiency, then by longest-available, then by a stable final tie-break key
- **AND** weighted scoring strategies are deferred to a later milestone

#### Scenario: No eligible candidate

- **WHEN** no agent satisfies the required skills in a routable state
- **THEN** runtime returns a typed no-eligible-candidate failure or follows the published fallback path
- **AND** no reservation is created

### Requirement: Reservation lifecycle

Open Routing SHALL model routing assignment as a reservation lifecycle rather than a one-shot selected agent.

#### Scenario: Offering a reservation

- **WHEN** runtime selects an eligible agent
- **THEN** it creates an offered reservation for that agent and interaction
- **AND** the offer can become accepted, rejected, timeout, or cancelled
- **AND** accepted reservations can later become completed

#### Scenario: Sequential offer policy

- **WHEN** a v0.2 route request creates reservation attempts
- **THEN** runtime offers one candidate at a time unless Wave 0 explicitly scopes broadcast offers
- **AND** duplicate or stale outcome signals from prior attempts are handled by the same guarded transition rules

#### Scenario: Legal reservation transitions

- **WHEN** a reservation transition is requested
- **THEN** `offered` can transition only to `accepted`, `rejected`, `timeout`, or `cancelled`
- **AND** `accepted` can transition to `completed`
- **AND** terminal states cannot transition to another terminal state
- **AND** post-accept cancellation is deferred unless Wave 0 explicitly adds it

#### Scenario: Reservation timeout

- **WHEN** an offered reservation times out
- **THEN** runtime records the timeout
- **AND** the flow can create another offered reservation attempt or follow a fallback path according to the published plan

#### Scenario: Reservation cancellation

- **WHEN** an offered reservation is cancelled because the route request is cancelled or abandoned before accept
- **THEN** runtime records the cancelled state and cancellation reason
- **AND** agent-state transition handling does not move the agent to Engaged for that reservation

#### Scenario: Test double accepts an offer

- **WHEN** the v0.2 test-double adapter or simulator accepts an offered reservation
- **THEN** runtime records the accepted state
- **AND** agent-state transition handling can move the agent to Engaged

#### Scenario: Test double completes an accepted reservation

- **WHEN** the v0.2 test-double adapter or simulator completes an accepted reservation
- **THEN** runtime records the completed state
- **AND** agent-state transition handling can move the agent from Engaged to WrapUp

#### Scenario: Guarding accept and timeout race

- **WHEN** an accept path and a timeout continuation race for the same offered reservation
- **THEN** only one guarded transition from offered to accepted or timeout succeeds transactionally
- **AND** the losing transition observes the non-offered state and is rejected or treated as an idempotent no-op with a recorded reason

#### Scenario: Guarding competing terminal outcomes

- **WHEN** accept, reject, timeout, or cancellation outcomes race for the same offered reservation
- **THEN** each transition is guarded on the reservation still being offered
- **AND** only one terminal outcome is recorded

#### Scenario: Preventing double-booking

- **WHEN** concurrent route requests try to offer the same agent
- **THEN** only one active offered or accepted reservation can exist for that agent
- **AND** the losing request must retry another candidate or follow fallback behavior

#### Scenario: Preventing double-acceptance

- **WHEN** multiple reservation outcomes race for the same route request
- **THEN** at most one reservation can become accepted
- **AND** stale accepts are rejected or treated as idempotent no-ops for the already accepted reservation

### Requirement: Durable delayed continuation

Runtime SHALL persist live wait and timeout continuations so route execution can resume after process restart.

#### Scenario: Resuming a wait node

- **WHEN** a live execution reaches a wait node
- **THEN** runtime stores a due-at continuation
- **AND** a server-side worker resumes the execution when the continuation is due

#### Scenario: Resuming a reservation timeout

- **WHEN** an offered reservation reaches its timeout deadline
- **THEN** runtime claims the due continuation transactionally
- **AND** the timeout transition only succeeds if the reservation is still offered
- **AND** records timeout or fallback behavior exactly once

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

Open Routing SHALL measure route-decision latency and report whether v0.2 runtime paths stay within the project p95 under 50 ms target.

#### Scenario: Running runtime performance gate

- **WHEN** runtime test fixtures execute route decisions under the supported v0.2 node set
- **THEN** p95 route-decision latency is reported
- **AND** regressions beyond the agreed budget are flagged for review before the metric becomes a blocking gate
