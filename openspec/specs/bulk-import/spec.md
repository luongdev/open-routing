# Bulk Import Specification

## Purpose

This specification captures the shipped v0.1 bulk import behavior for catalog seed data.

## Requirements

### Requirement: Import supported catalog entities

Open Routing SHALL import JSON or CSV data for agents, skills, queues, channels, adapters, and break reasons.

#### Scenario: Importing a supported entity

- **WHEN** an org admin posts an import for a supported entity type
- **THEN** valid rows are applied within the request org
- **AND** unsupported entity types are rejected

### Requirement: CSV normalization

CSV import SHALL handle common customer export shapes including BOMs, CRLF or LF line endings, quoted fields, commas, quotes, and embedded newlines.

#### Scenario: Parsing quoted CSV

- **WHEN** a CSV row contains quoted fields with commas or line breaks
- **THEN** the parser treats the quoted content as a single field
- **AND** row numbers in validation errors still point to the source row

### Requirement: Code-keyed upsert

Import SHALL upsert catalog rows by `(org_id, code)`.

#### Scenario: Re-running the same import

- **WHEN** the same valid input is imported twice for the same org
- **THEN** the second run updates matching records rather than creating duplicates
- **AND** another org can import rows with the same codes independently

### Requirement: Partial success response

Import SHALL report valid and invalid rows separately.

#### Scenario: Mixed valid and invalid rows

- **WHEN** an import contains both valid and invalid rows
- **THEN** valid rows are persisted
- **AND** invalid rows are returned with row number, field name, and message
- **AND** the response uses HTTP 207 when partial failure occurs

### Requirement: Import limits and schema version

Import SHALL enforce size and row-count limits and require CSV schema version negotiation.

#### Scenario: Rejecting oversized imports

- **WHEN** an import exceeds the v0.1 body or row-count limit
- **THEN** the API returns HTTP 413
- **AND** the response points clients toward the future async import path
