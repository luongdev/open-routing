# Agent State Delta

## MODIFIED Requirements

### Requirement: Engaged and wrap-up behavior

System transitions SHALL support runtime-owned channel-specific Engaged state, post-interaction target selection, and server-owned WrapUp expiry.

#### Scenario: Runtime offers an interaction

- **WHEN** the v0.2 test-double adapter or simulator accepts an offered reservation for a Ready agent
- **THEN** the agent moves to Engaged with the routed channel recorded
- **AND** the state version increments monotonically

#### Scenario: Applying wrap-up expiry

- **WHEN** an agent reaches WrapUp with a deadline
- **THEN** the server moves the agent to the configured post-interaction state after expiry
- **AND** the transition does not depend on browser connectivity

#### Scenario: Runtime completes an interaction

- **WHEN** runtime records interaction completion for an Engaged agent
- **THEN** the agent moves to WrapUp with a server-owned deadline
- **AND** WrapUp expiry later applies the post-interaction target without depending on browser connectivity

#### Scenario: Preserving invalid transition safety

- **WHEN** runtime attempts a system transition that violates the allowed matrix
- **THEN** the transition is rejected
- **AND** the reservation or route trace records the failure reason
