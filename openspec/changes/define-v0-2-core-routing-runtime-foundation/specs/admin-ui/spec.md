# Admin UI Delta

## ADDED Requirements

### Requirement: Flow workspace surfaces

The admin UI SHALL provide API-backed flow list, flow builder, simulator, publish, rollback, and trace viewer surfaces for v0.2 flow/runtime behavior.

#### Scenario: Listing flows

- **WHEN** an org admin opens the flow list
- **THEN** the UI loads flow drafts and published status from the API
- **AND** it does not use playground mock data for production routes

#### Scenario: Building and simulating a flow

- **WHEN** an org admin edits a flow graph
- **THEN** the UI can validate and simulate the draft before publish
- **AND** validation errors map back to the relevant node, edge, or field

#### Scenario: Inspecting a trace

- **WHEN** an org admin opens a runtime or simulation trace
- **THEN** the UI shows ordered execution steps and step details from the API
- **AND** the trace viewer remains usable without exposing cross-org records

### Requirement: Playground remains non-contractual

The playground SHALL remain a design and regression fixture surface unless a screen is explicitly backed by a v0.2 API contract.

#### Scenario: Viewing vNext preview screens

- **WHEN** a developer opens playground vNext screens
- **THEN** the screens may use local fixtures for design review
- **AND** those fixtures are not treated as shipped product behavior
