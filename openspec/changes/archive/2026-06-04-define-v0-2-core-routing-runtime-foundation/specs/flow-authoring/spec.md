# Flow Authoring Delta

## ADDED Requirements

### Requirement: Flow draft lifecycle

Open Routing SHALL let org admins create, update, list, disable, and version routing flow drafts with stable per-org `code` identity.

#### Scenario: Creating a flow draft

- **WHEN** an org admin creates a flow draft with a unique code
- **THEN** the draft is persisted inside the request org
- **AND** another org may use the same flow code independently

#### Scenario: Rejecting stale draft update

- **WHEN** an org admin updates a flow draft with a stale version
- **THEN** the API rejects the write with an optimistic-lock conflict
- **AND** the existing draft graph remains unchanged

### Requirement: Visual graph as source of truth

Open Routing SHALL treat the flow graph as the canonical authoring model for routing behavior.

#### Scenario: Saving graph changes

- **WHEN** a user edits nodes, edges, or node configuration in the flow builder
- **THEN** the persisted draft stores the graph as the source of truth
- **AND** generated DSL or compiled plans do not replace the graph as the authored artifact

### Requirement: Flow validation

Open Routing SHALL validate graph structure, supported node kinds, edge connectivity, graph export consistency, catalog references, and adapter references before simulation or publish.

#### Scenario: Rejecting invalid catalog reference

- **WHEN** a flow node references a queue, skill, channel, adapter, or break reason that does not exist in the same org
- **THEN** validation fails with a field-level error
- **AND** the flow cannot be published until the reference is fixed

### Requirement: Publish and rollback governance

Open Routing SHALL publish immutable flow versions and allow rollback to a prior published version.

#### Scenario: Publishing a valid flow

- **WHEN** validation passes for a flow draft
- **THEN** publish creates an immutable flow version
- **AND** runtime route requests use the currently published version

#### Scenario: Activating a flow entry binding

- **WHEN** publish or rollback activates a flow version for an org, channel, and entry code
- **THEN** the data model enforces at most one active published binding for that org, channel, and entry code
- **AND** a successful publish or rollback leaves one active published binding for its target org, channel, and entry code
- **AND** runtime route requests cannot observe multiple active published versions for the same binding

#### Scenario: Simulating before publish

- **WHEN** a user simulates a valid flow draft
- **THEN** the system returns a deterministic trace for the supplied inputs
- **AND** the simulation result is advisory unless product policy later makes a saved passing scenario mandatory

#### Scenario: Rolling back

- **WHEN** an org admin rolls back a flow
- **THEN** a previous published version becomes active
- **AND** trace output records which version handled each route request

### Requirement: DSL import and export surface

Open Routing SHALL support a reviewable textual representation of a flow without making it the primary authoring source.

#### Scenario: Exporting a flow

- **WHEN** a user exports a flow for review
- **THEN** the system returns a textual representation derived from the graph
- **AND** the graph version remains the canonical source for publish
