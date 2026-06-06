# Routing Engine Specification

## Purpose

This specification captures the v0.3 live routing engine: live candidate selection,
bidirectional matching with aging fairness, assignment safety under concurrency
across replicas, reclaim of a vanished agent's capacity, and explainable live
decisions on the runtime event + trace spine. It builds on the v0.2 deterministic
runtime (see `routing-runtime`); simulation keeps the deterministic snapshot path.

## Requirements

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
- **THEN** the engine pulls the best-ranked waiting route from a queue the agent
  serves and offers it
- **AND** ranking applies priority WITH AGING (waiting time raises effective
  priority), required skills, and a deterministic longest-idle tie-break — so no
  waiting route is starved

#### Scenario: Ring-no-answer (RONA)

- **WHEN** an offer to an agent times out unanswered
- **THEN** the agent is moved to a non-routable (Missed/RONA) state
- **AND** the engine does NOT immediately re-offer the same agent

### Requirement: Assignment safety under concurrency

The engine SHALL never assign one interaction to two agents and never exceed an
agent's channel capacity, including across multiple runtime replicas.

#### Scenario: Concurrent matcher replicas

- **WHEN** two replicas attempt to pull the same waiting route
- **THEN** exactly one acquires it and offers; the other skips it
- **AND** waiting routes are claimed with `FOR UPDATE SKIP LOCKED` plus the route
  run-lock and a match-offer token fence

#### Scenario: Capacity race

- **WHEN** two interactions could be offered to a one-capacity agent at once
- **THEN** at most one offer is created (guarded by transactional per-channel
  capacity slots)

### Requirement: Reclaiming a vanished agent's capacity

The engine SHALL reclaim a capacity slot held by an accepted reservation whose
agent has vanished mid-call, so a crashed or disconnected agent does not leak
capacity. Reclaim SHALL be grace-gated on the agent's last-seen heartbeat so a
brief reconnect blip does not tear down a live call. Automatic reassignment of the
reclaimed interaction to another agent is out of scope for v0.3 (it is media-
coupled) and is deferred to v0.4; v0.3 abandons the dropped interaction.

#### Scenario: Agent vanishes mid-call

- **WHEN** an agent holding an accepted reservation is unseen on any session past
  the reclaim grace window
- **THEN** the matcher tick cancels the reservation (reason agent_lost), frees the
  capacity slot, moves the agent out of the engaged state, and tears down the
  route if it is still live
- **AND** the reclaim is idempotent and fenced so a reconnect that lands first
  wins (no reclaim of a recovered call)

#### Scenario: Brief reconnect blip is not reclaimed

- **WHEN** an agent's transport drops and reconnects within the grace window
- **THEN** the reconnect refreshes the agent's last-seen heartbeat
- **AND** the live call is NOT reclaimed

### Requirement: Explainable live decisions

Every live routing decision SHALL be recorded on the runtime event + trace spine
so a live route is auditable after the fact, even though live routing is not
bit-for-bit replayable.

#### Scenario: Auditing a live route

- **WHEN** a live route completes or fails
- **THEN** a `route_decision` record captures the eligibility inputs, candidates
  considered, candidates EXCLUDED with their reasons, the ranking values, the
  capacity snapshot, and a decision version
- **AND** the decision timeline can be reconstructed from runtime events

#### Scenario: Reading a route's event timeline

- **WHEN** an operator opens a route request
- **THEN** the ordered runtime event log for that route is queryable
  (route.created → reservation.offered → accepted → agent.engaged → completed,
  and terminal/abandon/agent_lost events) and scoped to the request org
