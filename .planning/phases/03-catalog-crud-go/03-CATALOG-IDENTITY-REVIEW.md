---
phase: 03-catalog-crud-go
reviewed: 2026-05-17
topic: catalog identity contract
status: needs_design_decision
severity: HIGH
review_type: architecture_contract_review
---

# Catalog Identity Contract Review

## Summary

The current catalog identity model conflates two different concepts:

- `code`: user/business-facing stable key used in files, imports, exports, DSL, debugging, and manual operations.
- `external_id`: identifier from an outside system, used for integration/sync mapping.

The current implementation uses `external_id` as the required unique key for some catalog entities and omits both `code` and `external_id` for others. This will block or distort Phase 5 import semantics because import currently plans to upsert by `(org_id, external_id)`, but `external_id` is not the correct user-facing key.

No code has been changed for this review.

## Evidence

### Requirements Drift

- `.planning/REQUIREMENTS.md:26` says every catalog table enforces `UNIQUE (org_id, external_id)`.
- `.planning/REQUIREMENTS.md:70` says import upserts by `(org_id, external_id)`.
- `.planning/REQUIREMENTS.md:41-45` list `external_id` on agents, skills, queues, channels, but do not mention `code`.

These requirements encode `external_id` as the universal catalog identity. That conflicts with the product rule that user-operated file data needs a human/business key.

### Schema Drift

`migrations/000002_catalog_v0_1.up.sql` currently has:

- `agents`: `external_id TEXT NOT NULL`, `UNIQUE (org_id, external_id)`
- `skills`: `external_id TEXT NOT NULL`, `UNIQUE (org_id, external_id)`
- `queues`: `external_id TEXT NOT NULL`, `UNIQUE (org_id, external_id)`
- `channels`: `external_id TEXT NOT NULL`, `UNIQUE (org_id, external_id)`
- `adapters`: no `code`, no `external_id`
- `break_reasons`: no `code`, no `external_id`; unique key is `(org_id, name)`

This means:

- four entities force external-system semantics onto user-facing identity;
- two entities have no stable file/import identity except display name;
- `break_reasons.name` is treated as identity even though names are labels and are likely to be edited.

### OpenAPI Drift

`openapi/openapi.yaml` mirrors the same split:

- agents, skills, queues, channels require `external_id`;
- adapters and break reasons do not expose `external_id` or `code`;
- import endpoint docs describe upsert by `(org_id, external_id)`.

The public contract therefore trains clients to use `external_id` as the primary file key.

## Why This Matters

### `code` and `external_id` have different ownership

`code` is owned by Open Routing users/operators. It should be stable across imports, exports, support conversations, docs, and DSL references.

`external_id` is owned by another system. It may be absent, may collide across external sources, may change when connectors change, and may not exist for manually-created catalog rows.

Using `external_id` as the import key makes manually-authored files awkward and makes external sync semantics leak into core catalog design.

### `name` is not a safe identity

`break_reasons` currently rely on `(org_id, name)`. That is fragile because display labels are user-facing copy. Users will reasonably rename "Lunch" to "Meal break" without intending to create a different logical reason.

### Phase 5 import depends on this

If Phase 5 imports by `(org_id, external_id)`, it either:

- forces users to invent fake external IDs for hand-authored files; or
- makes rows without external mappings impossible to upsert cleanly; or
- treats external mappings as canonical identity, which is wrong for a product-owned catalog.

## Recommended Target Contract

### Universal catalog fields

Every primary catalog entity should have:

```text
id UUID PRIMARY KEY
org_id UUID NOT NULL
code TEXT NOT NULL
name TEXT NOT NULL
enabled BOOLEAN NOT NULL DEFAULT TRUE
version INTEGER NOT NULL DEFAULT 1
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
UNIQUE (org_id, code)
```

Apply `code` to:

- agents
- skills
- queues
- channels
- adapters
- break_reasons

`agent_skills` and `agent_states` are not primary catalog entities and should not get `code`.

### External mapping fields

Preferred design:

```text
external_source TEXT NULL
external_id TEXT NULL
CHECK (
  (external_source IS NULL AND external_id IS NULL)
  OR
  (external_source IS NOT NULL AND external_id IS NOT NULL)
)
UNIQUE (org_id, external_source, external_id)
  WHERE external_id IS NOT NULL
```

This supports multiple external systems without collisions.

Simpler v0.1 fallback:

```text
external_id TEXT NULL
UNIQUE (org_id, external_id) WHERE external_id IS NOT NULL
```

This is only safe if v0.1 explicitly supports at most one external source per entity/org.

## API Semantics

Recommended:

- create requests require `code` and `name`;
- create requests may accept `external_source` + `external_id`;
- update requests should not allow ordinary sparse PATCH of `code` unless product explicitly wants renames;
- imports upsert by `(org_id, code)`;
- external sync may upsert by `(org_id, external_source, external_id)`, but that should be a sync-specific path, not the generic import path.

## Import Semantics

Generic user file import should use:

```text
ON CONFLICT (org_id, code) DO UPDATE
```

Example CSV:

```csv
code,name,external_source,external_id
BREAK_LUNCH,Lunch,wfm,AUX_001
QUEUE_BILLING,Billing Queue,crm,Q-771
SKILL_VIP,VIP Support,hr_skill,S-42
```

References between rows should also prefer `code`, not UUID:

- agent skill assignment references skill by `skill_code`
- channel default queue references queue by `queue_code`
- future DSL references catalog rows by code

## Migration Timing

This should be corrected before Phase 5 import. The migration is still documented as editable during v0.1, so the cheapest fix is to amend `000002_catalog_v0_1.up.sql` and regenerate sqlc/OpenAPI outputs before freezing v0.1.

If PR #4 is merged before this decision, Phase 5 must start with a schema/API correction phase before writing import code.

## Suggested Blocking Decision

Block Phase 5 import planning until the team answers:

1. Is `code` required on all six primary catalog entities?
2. Is `code` immutable after create?
3. Does v0.1 support multiple external sources per entity/org?
4. Should `external_id` be optional everywhere?
5. Should generic import upsert by `code` while external sync upserts by `external_source + external_id`?

## Recommended Verdict

`BLOCK PHASE 5`.

Phase 4 can remain technically valid as a state-machine PR, but the catalog identity contract should be fixed before import/export/DSL surfaces are implemented.

## Review Prompt For Claude Opus

Review the current catalog identity design. Focus on whether Open Routing should introduce a universal user-facing `code` field separate from `external_id`.

Please analyze:

- whether `code` should exist on all six catalog entities;
- whether `external_id` should become optional;
- whether external mappings require `external_source`;
- how import/export should key upserts;
- how this affects OpenAPI, sqlc queries, migrations, handlers, tests, and Phase 5 import;
- whether PR #4 should be merged before or after this contract correction.

Expected output:

```text
## Summary
## Concerns [HIGH/MED/LOW]
## Recommended Contract
## Migration/API Impact
## Verdict: MERGE PR #4 FIRST | FIX BEFORE MERGE | BLOCK PHASE 5 ONLY
```
