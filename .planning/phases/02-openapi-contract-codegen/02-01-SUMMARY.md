---
phase: 02-openapi-contract-codegen
plan: "01"
subsystem: planning-artifact
tags: [sketches, api-shape-validation, openapi, contract-first]
dependency_graph:
  requires:
    - .planning/REQUIREMENTS.md
    - .planning/phases/02-openapi-contract-codegen/02-CONTEXT.md
  provides:
    - .planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md
  affects:
    - openapi/openapi.yaml (Plan 02 input — shape decisions locked here)
tech_stack:
  added: []
  patterns:
    - ASCII markdown sketches as contract validation artifacts (D-33 pattern)
    - OPEN QUESTIONS per sketch gates the spec author (D-34 sequencing rule)
key_files:
  created:
    - .planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md
  modified: []
decisions:
  - "Skills embedded on Agent detail GET response; omitted from Agent list GET"
  - "PATCH /agents/{id} skills field is full-replacement array, not delta"
  - "Bulk import failed[] entries contain (row, field, error, reason) only — no original row echo"
  - "schema_version is a query parameter per IMP-08 (?schema_version=v0.1)"
  - "413 response should use canonical envelope via request-body-size-limit middleware"
  - "State transition PATCH accepts single {to, break_reason_id?} per call, not batched"
  - "engaged_channel required in PATCH body when to=Engaged; system-initiated only"
  - "PATCH success returns full agent-state object to avoid follow-up GET"
  - "Cursor is opaque base64 encoding {created_at, id} composite"
  - "name search is case-insensitive substring (ILIKE) not prefix-only"
metrics:
  duration: "8 minutes"
  completed: "2026-05-15"
  tasks_completed: 1
  files_created: 1
---

# Phase 2 Plan 01: UI Sketches — API Shape Validation Summary

**One-liner:** Four ASCII paper-sketches validating N:M skills embedding, 207 BulkImportResult shape, state-transition request bodies, and cursor-paginated list envelope before openapi.yaml is locked.

## What Was Built

`02-UI-SKETCHES.md` — 515 lines, 4 sketches, 4 OPEN QUESTIONS sections, 1 Spec Author Checklist.

Each sketch covers a specific API-shape risk identified in D-33:

| Sketch | Screen | Shape risk validated |
|--------|--------|---------------------|
| 1 | Agent detail/edit | N:M agent_skills embedding vs separate endpoint; version field for CAT-08 |
| 2 | Bulk import result | 207 BulkImportResult schema; failed[] tuple shape; 413 envelope question |
| 3 | Agent status panel | STATE-02..STATE-07 transition request bodies; engaged_channel; wrapup_until; post_interaction_state; 409 invalid_transition shape |
| 4 | Catalog list | Cursor pagination envelope; include_disabled; case-insensitive name search |

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Author four ASCII paper-sketches in 02-UI-SKETCHES.md | fe55252 | .planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md |

## Sketch Decisions Made

### Sketch 1 — Agent detail/edit

- **Skills embedded on GET detail, not separate endpoint.** Single call returns full agent record
  with `skills: [{skill_id, name, proficiency}]`. The list endpoint (`GET /agents`) omits skills.
- **PATCH skills is full-replacement.** `skills` array on `PATCH /agents/{id}` replaces the entire
  join table set, not individual delta rows. Simpler server logic; atomically consistent.
- **Proficiency validated server-side and described client-side.** Client shows `min=1 max=10`
  hint; server enforces and returns `invalid_body` if violated.
- **No skills pagination on detail view.** Agents with >20 skills are rare in v0.1 catalog size.
  The org's skill list (for "Add skill" dropdown) uses cursor pagination.

### Sketch 2 — Bulk import result

- **`failed[]` entries are `{row, field, error, reason}` only.** Original row content is NOT echoed
  (deferred to v0.2 IMP-11 error CSV download).
- **`schema_version` is a query parameter.** `?schema_version=v0.1` per IMP-08.
- **413 should use canonical envelope.** Requires request-body-size-limit middleware firing before
  routing. Without this, `http.MaxBytesReader` returns a non-envelope response. Spec author must
  decide whether to document 413 as a special case or add middleware.
- **Content-type dispatch on same endpoint.** `Content-Type: application/json` and `text/csv`
  both go to `POST /v1/orgs/{org_id}/catalog/import`; handler dispatches on Content-Type.

### Sketch 3 — Agent status panel

- **Single PATCH per transition.** `{ to, break_reason_id? }` only. No batching.
- **`engaged_channel` required when `to=Engaged`.** Same endpoint; discriminated by required field.
  System-initiated only (agent cannot self-initiate Engaged). Authorization deferred to v1.
- **PATCH success returns full agent-state object.** Avoids follow-up GET to refresh the panel.
- **UI polls for state changes every ~5s.** REST polling only (v0.1 decision). No SSE endpoint needed.

### Sketch 4 — Catalog list

- **Cursor is opaque base64.** Encodes `{created_at, id}` composite. Implementation can change
  internals without client contract break.
- **Name search is substring (`ILIKE '%?%'`).** More useful than prefix; v0.1 catalog size makes
  performance equivalent. Add `pg_trgm` index recommendation in spec notes.
- **List items are flat agent records.** Skills are NOT embedded on list (OQ-1A resolution).
- **Common `PaginatedList` envelope.** Reused across all 6 entity list endpoints via `allOf`
  with entity-specific `items` type.

## OPEN QUESTIONS Resolution

All 12 open questions across the 4 sketches were resolved inline. Summary of Plan 02 implications:

| OQ | Resolved as | Plan 02 spec action |
|----|-------------|---------------------|
| OQ-1A | Skills embedded on GET detail, not separate endpoint | Spec embeds `skills[]` on AgentDetail schema |
| OQ-1B | Both client hint and server enforcement | `minimum: 1, maximum: 10` in schema |
| OQ-1C | No pagination on skills within detail | Return all agent skills on detail response |
| OQ-2A | `failed[]` is `{row, field, error, reason}` tuple | BulkImportResult schema omits original row |
| OQ-2B | `schema_version` is query param | `?schema_version=v0.1` in requestBody description |
| OQ-2C | Add request-body-size-limit middleware for 413 | Spec author to decide canonical vs special-case 413 |
| OQ-2D | Content-type dispatch on one endpoint | openapi.yaml requestBody has `application/json` + `text/csv` entries |
| OQ-3A | Single transition per PATCH | Body: `{ to, break_reason_id? }` |
| OQ-3B | `engaged_channel` required when `to=Engaged` | Add discriminated required field in spec |
| OQ-3C | Full agent-state object on success | Response schema is full AgentState not just delta |
| OQ-3D | REST polling, no SSE endpoint | No new spec endpoints for polling |
| OQ-4A | Opaque base64 cursor | `next_cursor: string, nullable: true` |
| OQ-4B | Substring ILIKE search | Spec description clarifies substring match |
| OQ-4C | List items are flat | AgentList schema omits skills array |
| OQ-4D | Common PaginatedList envelope | `components/schemas/PaginatedList` with allOf per entity |

## Spec Author Checklist Coverage

The Spec Author Checklist in `02-UI-SKETCHES.md` enumerates:

1. **Canonical ErrorResponse schema** — `{error: ErrorCode, reason: string, request_id: uuid}` per D-35
2. **Closed ErrorCode enum** — exactly 10 values per D-36 (`invalid_body`, `invalid_id`, `not_found`, `internal`, `version_conflict`, `cross_org`, `missing_org_id`, `invalid_transition`, `import_failed`, `rate_limited`)
3. **Three D-37 special-case shapes** — CAT-08 with `current`, STATE-03 with `from`/`to`, IMP-05 with `BulkImportResult`
4. **Pagination envelope** — `PaginatedList` with `items`, `next_cursor`, `has_more` reused across all 6 entity list endpoints
5. **`proficiency` integer constraints** — `minimum: 1, maximum: 10` in every AgentSkillAssignment schema
6. **UUIDv7 identifier convention** — `string, format: uuid` with description note; no custom format validator
7. **`X-Org-Id` security scheme** — `OrgHeader: apiKey in header` applied to all `/v1/` paths

## Deviations from Plan

None — plan executed exactly as written. The plan specified all four sketches, OPEN QUESTIONS per sketch, and Spec Author Checklist. All were produced.

## Threat Flags

None. Pure planning artifact (markdown) committed to git. No runtime surface, no network endpoints, no secrets. Consistent with threat register (T-02-01: information disclosure accepted; T-02-02: tampering caught by code review).

## Self-Check: PASSED

- [x] `02-UI-SKETCHES.md` exists: `/Users/luong/workspace/dev/open-solutions/.claude/worktrees/agent-af34047b97a48ae56/.planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md`
- [x] Line count >= 150: 515 lines
- [x] 4 sketch headings (`## Sketch [1-4]:`)
- [x] 4 OPEN QUESTIONS subsections (`### OPEN QUESTIONS`)
- [x] Spec Author Checklist section present
- [x] `request_id` referenced (11 times)
- [x] `ErrorResponse` and `error.*reason` referenced (13 times)
- [x] `proficiency` referenced (10 times)
- [x] `207`, `multi-status`, `failed` referenced (19 times)
- [x] `engaged_channel`, `wrapup_until`, `post_interaction_state`, `state_version` referenced (15 times)
- [x] `cursor`, `include_disabled` referenced (18 times)
- [x] Task commit `fe55252` exists in git log
