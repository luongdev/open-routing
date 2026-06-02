# Delivery Foundation Specification

## Purpose

This specification describes the shipped engineering foundation for the v0.1 catalog milestone.

## Requirements

### Requirement: Polyglot monorepo structure

Open Routing SHALL be organized as a polyglot monorepo with backend, frontend, API contract, migrations, scripts, and CI assets in one repository.

#### Scenario: Finding major project areas

- **WHEN** a developer checks out the repository
- **THEN** Go API code is under `services/api/`
- **AND** frontend workspaces are under `web/apps/admin`, `web/apps/embed`, and `web/packages/ui`
- **AND** the OpenAPI contract is under `openapi/openapi.yaml`
- **AND** database migrations are under `migrations/`

### Requirement: Org isolation foundation

Org-scoped behavior SHALL derive org identity from `X-Org-Id` and enforce isolation through request context, database access patterns, schema constraints, and tests.

#### Scenario: Handling missing org context

- **WHEN** an org-scoped API request omits `X-Org-Id` or provides a malformed value
- **THEN** the API returns HTTP 400
- **AND** the request does not reach handlers that can read org-scoped data

#### Scenario: Preventing cross-org reads

- **WHEN** two orgs contain overlapping user-facing identifiers
- **THEN** API list and read operations for one org do not return records from the other org
- **AND** tests seed overlapping identifiers to verify the boundary

### Requirement: Contract-first API generation

The REST management API SHALL use `openapi/openapi.yaml` as the source of truth for Go server stubs and TypeScript clients.

#### Scenario: Detecting generated-code drift

- **WHEN** code generation is run from the committed OpenAPI contract
- **THEN** generated Go and TypeScript artifacts match the committed files
- **AND** CI fails if generation produces uncommitted differences

### Requirement: Local and CI task orchestration

The repository SHALL provide repeatable tasks for development, code generation, linting, testing, building, and migration operations.

#### Scenario: Running the local stack

- **WHEN** a developer follows the root quickstart
- **THEN** Docker Compose starts PostgreSQL 17 and Redis
- **AND** explicit migration tasks apply schema changes
- **AND** application services run through the documented task commands
