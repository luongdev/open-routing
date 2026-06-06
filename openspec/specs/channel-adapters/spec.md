# Channel Adapters Specification

## Purpose

This specification captures the v0.3 channel-neutral assignment contract and its
mock voice implementation: deliver an accepted interaction to an agent, report
assignment lifecycle events, and release the delivery — with an opaque handle so
non-media channels (chat, email) and, in v0.4, real LiveKit/SIP media fit the same
contract without engine changes.

## Requirements

### Requirement: Channel-neutral assignment contract

Open Routing SHALL define a CHANNEL-NEUTRAL assignment-event contract (not a
media contract): deliver an accepted interaction to an agent, report assignment
lifecycle events, and release the delivery. The delivery `handle` SHALL be opaque
to the engine so non-media channels (chat, email) fit the same contract.

#### Scenario: Delivering an accepted interaction

- **WHEN** an agent accepts an offered interaction
- **THEN** the adapter is asked to deliver it and returns an opaque handle
- **AND** the engine treats the handle as opaque (no media assumptions)

#### Scenario: Assignment lifecycle drives the reservation

- **WHEN** the adapter reports accepted → connecting → established → completed
- **THEN** the reservation advances accordingly and the agent moves to WrapUp on
  completion
- **AND** a `failed` or `disconnected` event surfaces a typed handling failure
- **AND** every event is idempotent under a correlation id (safe to redeliver)

#### Scenario: Caller abandonment

- **WHEN** the customer abandons (hangs up) while queued or while the offer rings
- **THEN** the adapter emits `caller_abandoned`
- **AND** the engine tears down the in-flight reservation and route immediately
  with a recorded terminal reason (no ghost ringing)

#### Scenario: Engine-initiated release

- **WHEN** the engine ends a delivery (completed or cancelled, including a
  reclaimed vanished-agent call)
- **THEN** the adapter releases the handle
- **AND** the release is idempotent (a no-op if the delivery already ended)

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
