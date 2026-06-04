# Routing Engine Delta

## ADDED Requirements

### Requirement: Live candidate selection

For a LIVE route the engine SHALL select agents from the current eligible,
available, connected, and under-capacity pool at offer time, rather than a
snapshot pinned at route start. Simulation SHALL keep the pinned snapshot for
determinism.

#### Scenario: Offering to a live agent

- **WHEN** the engine needs an agent for a live route
- **THEN** it queries the current pool (eligible by skill, status Ready, a live
  connection present, under channel capacity)
- **AND** a snapshot-pinned simulation of the same flow is unaffected

#### Scenario: Excluding unavailable agents

- **WHEN** an otherwise-eligible agent is disconnected or at capacity
- **THEN** the engine does not offer to that agent

### Requirement: Bidirectional matching

The engine SHALL match in both directions: an arriving interaction to an
available agent, and a newly-available agent to the highest-priority waiting
interaction from a queue it serves.

#### Scenario: Interaction-driven offer

- **WHEN** a route needs an agent and one is available
- **THEN** the engine offers the interaction to the top-ranked candidate

#### Scenario: Availability-driven pull

- **WHEN** an agent becomes Ready or frees capacity
- **THEN** the engine pulls the highest-priority waiting route from a queue the
  agent serves and offers it
- **AND** ranking applies priority, required skills, and a longest-idle tie-break

### Requirement: Assignment safety under concurrency

The engine SHALL never assign one interaction to two agents and never exceed an
agent's channel capacity, including across multiple runtime replicas.

#### Scenario: Concurrent matcher replicas

- **WHEN** two replicas attempt to pull the same waiting route
- **THEN** exactly one acquires it and offers; the other skips it
- **AND** waiting routes are claimed with `FOR UPDATE SKIP LOCKED` plus the route
  run-lock

#### Scenario: Capacity race

- **WHEN** two interactions could be offered to a one-capacity agent at once
- **THEN** at most one offer is created (guarded by the per-agent active-reservation
  invariant)

### Requirement: Explainable live decisions

Every live routing decision SHALL be recorded on the runtime event + trace spine
so a live route is auditable after the fact, even though live routing is not
bit-for-bit replayable.

#### Scenario: Auditing a live route

- **WHEN** a live route completes or fails
- **THEN** its trace records the pool queried, candidate chosen, offer, and outcome
- **AND** the decision timeline can be reconstructed from runtime events
