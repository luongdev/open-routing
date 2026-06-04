# Admin UI Specification

## Purpose

This specification captures the shipped v0.1 shared Lit UI library and standalone admin application behavior.

## Requirements

### Requirement: Standalone admin SPA

Open Routing SHALL provide a Vite, Lit, Shoelace, and TypeScript standalone admin app for direct catalog management.

#### Scenario: Running the admin app

- **WHEN** a platform or solution engineer opens the admin app
- **THEN** they can manage v0.1 catalog entities through the shared UI components
- **AND** API calls include the org context selected or configured in the app

### Requirement: Shared UI package

Catalog UI behavior SHALL be implemented in `web/packages/ui` so admin and embed surfaces share the same core components and API client wrapper.

#### Scenario: Reusing catalog shell

- **WHEN** a catalog management surface renders entity navigation and CRUD screens
- **THEN** it uses the shared catalog shell and entity components
- **AND** behavior remains consistent between standalone and embedded delivery surfaces

### Requirement: CRUD screens for catalog entities

The admin UI SHALL provide list, create, edit, and delete or disable workflows for agents, skills, queues, channels, adapters, and break reasons.

#### Scenario: Editing a catalog record

- **WHEN** an admin edits an existing record
- **THEN** the form sends the version from the loaded record
- **AND** stale updates surface a reload affordance instead of silently overwriting server state

### Requirement: Theme and localization foundations

The admin UI SHALL support theme tokens and locale-ready text handling consistent with the shared component library.

#### Scenario: Applying theme tokens

- **WHEN** theme variables are configured for the admin surface
- **THEN** shared UI components render using those tokens
- **AND** the configuration does not require changing component code

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

#### Scenario: Embedding the flow builder

- **WHEN** a host product mounts the flow builder as a Web Component
- **THEN** the builder canvas works inside Shadow DOM without event-retargeting or measurement regressions
- **AND** it uses the same trusted-host org context as the v0.1 catalog embed

### Requirement: Playground remains non-contractual

The playground SHALL remain a design and regression fixture surface unless a screen is explicitly backed by a v0.2 API contract.

#### Scenario: Viewing vNext preview screens

- **WHEN** a developer opens playground vNext screens
- **THEN** the screens may use local fixtures for design review
- **AND** those fixtures are not treated as shipped product behavior
