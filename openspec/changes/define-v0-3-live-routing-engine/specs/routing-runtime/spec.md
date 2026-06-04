# Routing Runtime Delta

## ADDED Requirements

### Requirement: Reservation lifecycle from live signals

The reservation lifecycle SHALL be drivable by real agent and adapter signals in
addition to the v0.2 test-double endpoints, using the same transitions and route
resume. The test-double endpoints SHALL remain for simulation, CI, and the Route
Tester.

#### Scenario: Live accept

- **WHEN** a connected agent accepts an offered reservation over the realtime
  transport
- **THEN** the reservation moves offered → accepted via the same guarded
  transition as the test-double path
- **AND** the route resumes from its cursor

#### Scenario: Live reject and re-offer

- **WHEN** a connected agent rejects an offer
- **THEN** the reservation moves offered → rejected and the engine re-offers per
  the matcher rules

#### Scenario: Disconnect mid-handling

- **WHEN** a connected agent disconnects while handling an accepted interaction
- **THEN** after a configurable grace window the interaction is reassigned
- **AND** the original reservation is resolved so it cannot double-complete

### Requirement: Live timeout via durable continuations

A live offer's timeout SHALL fire through the existing durable continuation worker
so a missed accept is bounded without depending on any live connection.

#### Scenario: Unanswered live offer

- **WHEN** a live offer is not accepted before its timeout
- **THEN** the continuation worker times it out and the engine re-offers or falls
  back per the flow
