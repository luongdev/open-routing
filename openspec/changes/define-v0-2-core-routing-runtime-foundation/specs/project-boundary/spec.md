# Project Boundary Delta

## MODIFIED Requirements

### Requirement: Flow-centered future evolution

Open Routing SHALL evolve toward visual flow authoring, deterministic simulation, publish governance, and runtime execution after the v0.1 catalog foundation is stable. In v0.2, adapter reference validation is limited to the existing adapter catalog rows unless a capabilities entity is explicitly added.

#### Scenario: Publishing a flow in v0.2

- **WHEN** a product team publishes a routing flow
- **THEN** the visual graph is the source of truth
- **AND** publish validates graph structure, export consistency, runtime plan shape, catalog references, and adapter references supported by the v0.2 data model
- **AND** adapter capability validation remains deferred until capabilities are formalized
