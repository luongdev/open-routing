# Embed Catalog Delta Specification

## ADDED Requirements

### Requirement: Web Component registration

Open Routing SHALL provide an ES module embed bundle that registers `<open-routing-catalog>` as a Custom Element.

#### Scenario: Loading the embed module

- **WHEN** a host page imports the embed module
- **THEN** `<open-routing-catalog>` is registered
- **AND** the host can mount the catalog configuration surface without using a specific frontend framework

### Requirement: Attribute-based embed context

The embed element SHALL read org, API, theme, module, and locale configuration from HTML attributes.

#### Scenario: Mounting with required attributes

- **WHEN** a host mounts `<open-routing-catalog org-id="..." api-base-url="...">`
- **THEN** catalog API calls use `X-Org-Id` from `org-id`
- **AND** API requests target the configured base URL

#### Scenario: Handling invalid org attribute

- **WHEN** the required org attribute is missing or malformed
- **THEN** the element renders an error state inside Shadow DOM
- **AND** it does not fall back to global browser state

### Requirement: Shadow DOM isolation

The embed surface SHALL render inside Shadow DOM and prevent style leakage between host page and catalog UI.

#### Scenario: Host page uses aggressive reset

- **WHEN** the host page applies broad CSS reset rules
- **THEN** embedded catalog UI remains usable and visually coherent
- **AND** catalog styles do not alter host page elements

### Requirement: Shared catalog shell reuse

The embed surface SHALL reuse the shared catalog shell and entity components from `web/packages/ui`.

#### Scenario: Rendering catalog navigation

- **WHEN** the embed surface renders catalog navigation
- **THEN** it uses the same shared component behavior as the standalone admin app
- **AND** entity screens are not forked for embed-only behavior

### Requirement: Hash routing for embed

The embed surface SHALL use hash-based routing so host applications do not need to coordinate server history routes.

#### Scenario: Navigating inside embed

- **WHEN** a user navigates between embedded catalog modules
- **THEN** route state is reflected through the embed hash routing mode
- **AND** the host application does not need to serve embed-specific history paths

### Requirement: Module visibility filtering

The embed element SHALL support limiting visible catalog modules through a comma-separated `modules` attribute.

#### Scenario: Host limits modules

- **WHEN** a host sets `modules="agents,skills,queues"`
- **THEN** only those modules appear in embedded navigation
- **AND** hidden modules are not reachable through normal embed navigation

### Requirement: Public embed events

The embed element SHALL dispatch composed, bubbling events for request context announcement and auth expiry.

#### Scenario: Announcing request context

- **WHEN** the element connects
- **THEN** it dispatches `open-routing:request-context`
- **AND** event detail includes org ID, API base URL, theme, and modules

#### Scenario: Reporting auth expiry

- **WHEN** an embedded API request receives HTTP 401
- **THEN** the element dispatches `open-routing:auth-expired`
- **AND** event detail includes status code, request ID when available, and request path

### Requirement: Embed bundle budget

The eager embed bundle SHALL stay at or below 70 KB gzipped.

#### Scenario: Checking bundle size in CI

- **WHEN** CI builds the embed package
- **THEN** the eager gzipped bundle size is measured
- **AND** the build fails when it exceeds 70 KB

### Requirement: Host integration matrix

Embed behavior SHALL be verified in React 18, Vue 3, and plain HTML host pages.

#### Scenario: Running host matrix tests

- **WHEN** embed Playwright tests run
- **THEN** each supported host type mounts the element
- **AND** tests verify Shadow DOM isolation, org header propagation, auth expiry events, and module filtering

### Requirement: CORS support for embedded hosts

The API SHALL support configured cross-origin embed requests.

#### Scenario: Handling preflight

- **WHEN** a host sends an OPTIONS preflight request for an org-scoped endpoint
- **THEN** CORS middleware handles the preflight before org context enforcement
- **AND** the response allows the configured origin, method, and headers
