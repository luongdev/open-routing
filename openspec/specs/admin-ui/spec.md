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
