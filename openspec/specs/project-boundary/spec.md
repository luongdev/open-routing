# Project Boundary Specification

## Purpose

This specification captures the product boundary that keeps Open Routing focused on embeddable routing and configuration rather than becoming a full contact-center suite.

## Requirements

### Requirement: Routing-only product boundary

Open Routing SHALL own routing configuration, routing decisions, flow governance, catalog foundations, state projections, and adapter contracts, while excluding media, CRM, ticketing, WFM, and agent desktop ownership.

#### Scenario: Keeping media outside core

- **WHEN** a voice, chat, or email interaction needs channel execution
- **THEN** Open Routing returns routing decisions and adapter commands
- **AND** the external channel system owns holding, call control, chat execution, email execution, and session lifecycle

#### Scenario: Avoiding agent desktop scope

- **WHEN** an agent needs to work an assigned interaction
- **THEN** the host product or downstream system provides the work surface
- **AND** Open Routing only exposes routing, state, and configuration contracts

### Requirement: Embedded-first product surface

Open Routing SHALL be usable by host product teams through embeddable configuration surfaces while remaining directly usable by platform and solution engineers.

#### Scenario: Host product embeds catalog configuration

- **WHEN** a host product mounts an Open Routing configuration surface
- **THEN** it can provide org context, API base URL, theme, and module visibility through explicit integration inputs
- **AND** the embedded surface does not rely on global browser state for org identity

### Requirement: Flow-centered future evolution

Open Routing SHALL evolve toward visual flow authoring, deterministic simulation, publish governance, and runtime execution after the v0.1 catalog foundation is stable. In v0.2, adapter reference validation is limited to the existing adapter catalog rows unless a capabilities entity is explicitly added.

#### Scenario: Publishing a flow in v0.2

- **WHEN** a product team publishes a routing flow
- **THEN** the visual graph is the source of truth
- **AND** publish validates graph structure, export consistency, runtime plan shape, catalog references, and adapter references supported by the v0.2 data model
- **AND** adapter capability validation remains deferred until capabilities are formalized
