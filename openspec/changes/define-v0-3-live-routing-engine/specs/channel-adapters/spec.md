# Channel Adapters Delta

## ADDED Requirements

### Requirement: Channel adapter contract

Open Routing SHALL define a channel adapter contract that delivers an accepted
interaction to an agent, reports adapter lifecycle events (answered, ended,
failed), and releases the delivery. The contract SHALL be shaped for a media-room
bridge (LiveKit/SIP) so a real voice adapter implements it without contract
changes.

#### Scenario: Delivering an accepted interaction

- **WHEN** an agent accepts an offered interaction
- **THEN** the adapter is asked to deliver it (bridge the agent and the
  interaction)
- **AND** the adapter returns a handle the engine can later release

#### Scenario: Adapter lifecycle drives the reservation

- **WHEN** the adapter reports the interaction answered then ended
- **THEN** the reservation moves accepted → completed and the agent moves to WrapUp
- **AND** an adapter failure surfaces as a typed routing/handling failure

### Requirement: Mock voice adapter for v0.3

v0.3 SHALL ship a mock voice adapter satisfying the contract so the live engine
runs end to end without real media; real LiveKit/SIP media is a v0.4 concern
behind the same contract.

#### Scenario: Running the engine on the mock adapter

- **WHEN** the engine routes a live interaction with the mock voice adapter
- **THEN** the mock reports answered on accept and ended on complete
- **AND** the full match → offer → accept → handle → complete path runs without
  media infrastructure

#### Scenario: Swapping in real media later

- **WHEN** v0.4 implements the contract against livekit-server/sip
- **THEN** the engine, transport, and reservation lifecycle require no changes
