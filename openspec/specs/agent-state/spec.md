# Agent State Specification

## Purpose

This specification captures the shipped v0.1 agent status model, transition rules, and routability semantics.

## Requirements

### Requirement: Persisted agent status model

Open Routing SHALL persist one agent state row per agent with status, state version, optional engaged channel, optional break reason, post-interaction target, and optional wrap-up deadline.

#### Scenario: Creating an agent

- **WHEN** an agent record is created
- **THEN** the system creates the corresponding agent state foundation
- **AND** later status reads can retrieve that state within the same org

### Requirement: Allowed transition matrix

Agent status changes SHALL follow the allowed transition matrix for agent-initiated and system-initiated transitions.

#### Scenario: Rejecting invalid transition

- **WHEN** a client requests a transition that is not allowed from the current status
- **THEN** the API returns HTTP 409 with `invalid_transition`
- **AND** the persisted state does not change

#### Scenario: Accepting agent-initiated transition

- **WHEN** an agent moves between allowed Ready, NotReady, Break, or WrapUp targets
- **THEN** the API persists the new status
- **AND** the state version increments monotonically

### Requirement: Break reason validation

Ready to Break transitions SHALL require a break reason that exists in the same org.

#### Scenario: Rejecting cross-org break reason

- **WHEN** an agent enters Break with a break reason from another org
- **THEN** the API returns HTTP 422
- **AND** the state remains unchanged

### Requirement: Engaged and wrap-up behavior

System transitions SHALL support channel-specific Engaged state, post-interaction target selection, and server-owned WrapUp expiry.

#### Scenario: Applying wrap-up expiry

- **WHEN** an agent reaches WrapUp with a deadline
- **THEN** the server moves the agent to the configured post-interaction state after expiry
- **AND** the transition does not depend on browser connectivity

### Requirement: Routability helper

Open Routing SHALL treat an agent as routable only when status is Ready or status is Break with a routable break reason.

#### Scenario: Evaluating Break routability

- **WHEN** an agent is in Break
- **THEN** routability follows the associated break reason `routable` value
- **AND** missing or unroutable reasons do not make the agent routable
