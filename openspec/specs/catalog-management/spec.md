# Catalog Management Specification

## Purpose

This specification captures the shipped v0.1 catalog management behavior for agents, skills, queues, channels, adapters, break reasons, and agent skill assignments.

## Requirements

### Requirement: Six primary catalog entities

Open Routing SHALL manage agents, skills, queues, channels, adapters, and break reasons as org-scoped catalog entities.

#### Scenario: Managing catalog records

- **WHEN** an org admin creates, reads, updates, lists, or disables a catalog record
- **THEN** the operation is scoped to the org from `X-Org-Id`
- **AND** disabled records are excluded from default list responses unless explicitly requested

### Requirement: Catalog identity model

Each primary catalog entity SHALL use `code` as the required user-facing identifier and optional `external_id` as an integration mapping.

#### Scenario: Preventing duplicate codes

- **WHEN** a create or import operation uses a `code` that already exists in the same org
- **THEN** the operation reports `duplicate_code`
- **AND** another org may use the same `code`

#### Scenario: Handling optional external IDs

- **WHEN** an `external_id` is provided
- **THEN** it is unique within the same org only when non-null
- **AND** duplicate non-null values report `duplicate_external_id`

### Requirement: Optimistic update safety

Catalog updates SHALL require a version value and reject stale writes.

#### Scenario: Updating with a stale version

- **WHEN** a client updates a record with a version older than the server record
- **THEN** the API returns HTTP 409
- **AND** the response includes the current server-side record where the endpoint contract supports it

### Requirement: List filtering and pagination

Catalog list endpoints SHALL support cursor pagination, enabled filtering, disabled inclusion, and case-insensitive name search.

#### Scenario: Searching enabled records

- **WHEN** a client lists records with a name query and default options
- **THEN** matching enabled records are returned
- **AND** disabled records remain hidden unless disabled inclusion is requested

### Requirement: Agent skill assignment

Open Routing SHALL allow assigning and unassigning skills to agents with validated proficiency.

#### Scenario: Rejecting invalid proficiency

- **WHEN** a client assigns a skill to an agent with proficiency outside the allowed range
- **THEN** the API returns HTTP 422
- **AND** no invalid join row is persisted

### Requirement: Hot-path catalog cache

Single-entity catalog reads and state lookups SHALL use Redis cache keys namespaced by org, entity, and record ID with invalidation on writes.

#### Scenario: Invalidating after write

- **WHEN** a catalog entity is changed
- **THEN** the matching cache entry is invalidated during the same request path
- **AND** subsequent reads observe the updated persisted record
