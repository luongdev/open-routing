# Phase 2: UI Sketches — API Shape Validation

**Purpose:** Contract validation only. Per D-33/D-34, these sketches exist so awkward API shapes get
caught before `openapi.yaml` is locked. They are NOT UI implementation — Phase 6 owns real UI.

**ASCII convention:** Box-drawing characters (┌─┐│└┘├┤┬┴┼) for frames; brackets [Button] for
buttons; (•) for radio; [x]/[ ] for checkboxes; `---` for separators. Each sketch is ~30-50 lines.

**Wave gate:** These four sketches must be reviewed and their OPEN QUESTIONS resolved before
Plan 02 (the OpenAPI spec) is committed. D-34 is explicit: sketch → review → spec, never parallel.

---

## Sketch 1: Agent detail/edit

**Validates:** N:M `agent_skills` embedding choice (CAT-01, CAT-03). Decision being tested:
embed skills array on Agent detail response vs. a separate `GET /agents/{id}/skills` call.

```
┌──────────────────────────────────────────────────────────────────────┐
│ [← Back to Agents]          Agent Detail                             │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  [LOADING STATE]                                                     │
│  ░░░░░░░░░░░░░░░░░░░  name skeleton                                  │
│  ░░░░░░░░░░░░  email skeleton          ░░░  enabled toggle           │
│  ░░░░░░░░░░░░░░░░░░░░░░░░░  skills skeleton                          │
│                                                                      │
│  [POPULATED STATE]                                                   │
│  name:         [ Alice Nguyen                          ]             │
│  email:        [ alice@example.com                     ]             │
│  external_id:  [ EMP-0042                              ] (read-only) │
│  enabled:      [x] Enabled                                           │
│  version:      <hidden: 7>    (sent on PATCH for CAT-08)             │
│                                                                      │
│  ── Skills ───────────────────────────────────────────────────────── │
│  | Skill name          | Proficiency (1-10) | Actions               |│
│  |--------------------|--------------------|-----------------------|│
│  | Billing Support     | [8          ▾]     | [Remove]              |│
│  | Technical Triage    | [6          ▾]     | [Remove]              |│
│  | (empty row)         |                    |                       |│
│  |--------------------|--------------------|-----------------------|│
│  [+ Add skill  ▾]  → dropdown: paginated org skills (cursor-based)  │
│                       proficiency input: min=1 max=10                │
│  [Save changes]   [Cancel]                                           │
│                                                                      │
│  [ERROR STATE — canonical envelope]                                  │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ Error: not_found                                            │    │
│  │ The agent does not exist in this org.                       │    │
│  │ request_id: 01901b2c-7f3a-7abc-8d4e-5f6a7b8c9d0e           │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  [EMPTY SKILLS STATE]                                                │
│  No skills assigned. Use [+ Add skill] to assign one.               │
│                                                                      │
│  [409 VERSION CONFLICT]                                              │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ This agent was updated by someone else.                     │    │
│  │ error: version_conflict  request_id: 01901b2c-…            │    │
│  │ [Reload latest]   [Keep my edits — merge manually]         │    │
│  └─────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────┘
```

**API shape implied:**
- `GET /v1/orgs/{org_id}/agents/{id}` returns agent with `skills: [{skill_id, name, proficiency}]`
  embedded (single call). Does NOT require a separate `GET /agents/{id}/skills` for the detail view.
- `PATCH /v1/orgs/{org_id}/agents/{id}` body includes `{ name, email, enabled, version, skills: [{skill_id, proficiency}] }`.
  Skills replace the full set (PUT-semantics on the join table, not PATCH-on-individual-rows).
- Skill dropdown uses `GET /v1/orgs/{org_id}/skills?cursor=<token>` (cursor pagination, CAT-10).
- 409 body: `{ error: "version_conflict", reason, request_id, current: <AgentSchema> }` per D-37.

### OPEN QUESTIONS

**OQ-1A: Embed skills or separate endpoint?**
This sketch assumes embed. The list view (`GET /agents`) does NOT need skills (too expensive). If
the skills array is only needed on detail, embed is correct — one fewer round-trip. If any other
screen needs skills without the agent record, a separate endpoint makes sense.
**Resolution for Plan 02:** Embed skills on `GET /agents/{id}`; omit from `GET /agents` list.
`POST/PATCH /agents/{id}` accepts `skills` as a full-replacement array.

**OQ-1B: Does `proficiency` validation (1–10) happen server-side only, or also client-side?**
Both: client shows `min=1 max=10` to block obvious mistakes; server enforces and returns
`invalid_body` if violated (CAT-03 requires server enforcement). Two-layer validation is safe.

**OQ-1C: Does the skills list on the detail view need its own cursor-pagination?**
No — agents with >20 skills are rare in v0.1 catalog scope. Return all skills on the agent
detail response. The org's skill list (for the "Add skill" dropdown) uses cursor pagination.

---

## Sketch 2: Bulk import result

**Validates:** HTTP 207 `BulkImportResult` schema (IMP-04, IMP-05, IMP-07, IMP-08).
Decision being tested: what the `failed[]` array entries contain; whether original row is echoed.

```
┌──────────────────────────────────────────────────────────────────────┐
│ [← Back to Catalog]         Bulk Import                              │
├──────────────────────────────────────────────────────────────────────┤
│  [PRE-UPLOAD STATE]                                                  │
│  Entity type:  (•) Agents  ( ) Skills  ( ) Queues                   │
│                ( ) Channels  ( ) Adapters  ( ) Break Reasons         │
│                                                                      │
│  File:         [Choose file…]   (JSON or CSV)                        │
│  helper text:  Max 50 MB / 500 rows. Over that? Use v0.2 async path. │
│  schema_version: v0.1  (hidden — sent as ?schema_version=v0.1)       │
│                                                                      │
│  [Import]                                                            │
│                                                                      │
│  [413 ERROR — oversized]                                             │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ File too large (56 MB). Limit: 50 MB / 500 rows.            │    │
│  │ For larger imports, use the v0.2 async import path.         │    │
│  │ error: not specified (413 body may be non-envelope)         │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  [400 ERROR — schema_version mismatch]                               │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ Unsupported schema version 'v0.2'.                          │    │
│  │ Supported versions: ["v0.1"]                                │    │
│  │ error: invalid_body  request_id: 01901b2c-…                │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  [207 RESULT — mixed success/failure]                                │
│  ── Import complete ──────────────────────────────────────────────── │
│  Total rows: 10   Succeeded: 7   Failed: 3                           │
│                                                                      │
│  Succeeded IDs: [▶ Show 7 IDs]   (collapsible)                      │
│    01901b2c-0001, 01901b2c-0002, …                                   │
│                                                                      │
│  Failed rows:                                                        │
│  | Row | Field      | Error code   | Reason                         |│
│  |-----|------------|--------------|-------------------------------|│
│  |  2  | email      | invalid_body | Not a valid email address.    |│
│  |  5  | proficiency| invalid_body | Must be between 1 and 10.     |│
│  |  9  | external_id| import_failed| Duplicate in same batch.      |│
│                                                                      │
│  [Download failed rows as CSV]   [Retry failed rows]                │
│                                                                      │
│  [200 RESULT — all succeeded]                                        │
│  Total rows: 5   Succeeded: 5   Failed: 0                            │
│  Succeeded IDs: [▶ Show 5 IDs]                                       │
└──────────────────────────────────────────────────────────────────────┘
```

**API shape implied:**
- `POST /v1/orgs/{org_id}/catalog/import?entity=agents&schema_version=v0.1`
  body: `multipart/form-data` or `application/json` / `text/csv`
- 207 body (D-37 `BulkImportResult`):
  ```json
  {
    "succeeded": ["01901b2c-0001", "01901b2c-0002"],
    "failed": [
      { "row": 2, "field": "email", "error": "invalid_body", "reason": "Not a valid email." },
      { "row": 5, "field": "proficiency", "error": "invalid_body", "reason": "Must be 1-10." }
    ]
  }
  ```
- 200 body: same schema with empty `failed: []`.
- 400 body: canonical `{ error, reason, request_id }` with `reason` listing supported versions.
- 413 body: may NOT be the canonical envelope (nginx/Go http.MaxBytesReader fires before handler).

### OPEN QUESTIONS

**OQ-2A: Should `failed[]` entries echo the original row content?**
The sketch above does NOT include original row in the `failed[]` entry. For a CSV import this would
be awkward to normalise. The (row, field, error, reason) tuple is sufficient for re-editing.
Echoing the original row content is deferred to v0.2 "error CSV download" (IMP-11).
**Resolution for Plan 02:** `failed[]` entries contain `{row, field, error, reason}` only.

**OQ-2B: Is `schema_version` a query parameter or a request header?**
IMP-08 says `?schema_version=v0.1`. The sketch treats it as a query param. Headers would be
cleaner but query params are more cacheable and visible in browser history/logs.
**Resolution for Plan 02:** Query param `?schema_version=v0.1` as specified by IMP-08.

**OQ-2C: Does the 413 response use the canonical error envelope?**
Go's `http.MaxBytesReader` returns a non-Go-handler error; the response may be generated by the
framework before the handler runs. Plan 02 must decide: wrap in the canonical envelope (requires
custom middleware), or document 413 as a non-envelope special case.
**Resolution for Plan 02 (spec author to confirm):** Add request-body-size-limit middleware that
fires before routing and returns the canonical envelope. Simplifies client error parsing.

**OQ-2D: What's the content-type negotiation for JSON vs CSV uploads?**
`Content-Type: application/json` for JSON input; `Content-Type: text/csv` for CSV input. Both
go to the same endpoint, handler dispatches on Content-Type. The `openapi.yaml` requestBody
should use `oneOf` or separate content entries (`application/json`, `text/csv`).

---

## Sketch 3: Agent status panel

**Validates:** STATE-02..STATE-07 transition request body, `engaged_channel`, `wrapup_until`,
`post_interaction_state`, `state_version`, and 409 invalid-transition error shape.

```
┌──────────────────────────────────────────────────────────────────────┐
│ [← Back to Agent Detail]    Agent Status Panel                       │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Agent:    Alice Nguyen  (alice@example.com)                         │
│  State version: 42   (STATE-08: monotonic; stale check compares to  │
│                        last known version — UI drops update if       │
│                        received version < last known)                │
│                                                                      │
│  ── Current status: READY ─────────────────────────────────────────  │
│  [Set Not Ready]   [Go on Break ▾]   (* system-set: Set Engaged)     │
│                                                                      │
│  ── Current status: BREAK (Billing Support • routable) ────────────  │
│  IsRoutable: YES (break reason has routable=true per STATE-10)       │
│  [Back to Ready]   [Set Not Ready]                                   │
│                                                                      │
│  ── Current status: ENGAGED (channel: voice) ───────────────────────  │
│  engaged_channel: voice   (STATE-05 — set by system on Ready→Engaged)│
│  post_interaction_state:                                             │
│    (•) Ready after wrap-up   ( ) Not Ready after wrap-up  [Set]      │
│    (STATE-06 — agent sets while Engaged; system applies after WrapUp)│
│                                                                      │
│  ── Current status: WRAPUP ────────────────────────────────────────  │
│  wrapup_until: 2026-05-15T14:55:00Z   countdown: 2m 17s remaining   │
│  (STATE-07: server-owned TTL; goroutine auto-transitions on expiry)  │
│  Auto-transition to: Ready  (based on post_interaction_state)        │
│                                                                      │
│  ── Current status: NOTREADY ──────────────────────────────────────  │
│  [Set Ready]                                                         │
│                                                                      │
│  [TRANSITION — Go on Break dropdown]                                 │
│  Break reason:  ( ) Billing Support (routable)                       │
│                 (•) Lunch Break     (not routable)                   │
│                 ( ) Team Huddle     (routable)                       │
│  [Confirm Break]   [Cancel]                                          │
│                                                                      │
│  [409 INVALID TRANSITION]                                            │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ Transition not allowed:  Engaged → NotReady                 │    │
│  │ error: invalid_transition  request_id: 01901b2c-…           │    │
│  │ from: Engaged              to: NotReady                     │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  [OFFLINE STATE]                                                     │
│  Status: Offline. Agent is not currently logged in.                  │
│  (* transitions to NotReady on next login — system-initiated)        │
└──────────────────────────────────────────────────────────────────────┘
```

**API shape implied:**
- `PATCH /v1/orgs/{org_id}/agents/{id}/status` body:
  ```json
  { "to": "Break", "break_reason_id": "01901b2c-…" }
  ```
  or for post-interaction state while Engaged:
  ```json
  { "post_interaction_state": "ready" }
  ```
- Response on success: `{ status: "Break", state_version: 43, break_reason_id: "…" }` or full agent-state object.
- 409 body (D-37 STATE-03): `{ error: "invalid_transition", reason, request_id, from: "Engaged", to: "NotReady" }`.
- 422 body (STATE-04 missing/invalid break_reason): `{ error: "not_found", reason, request_id }` (cross-org reason returns 404 not 422 per FOUND-08 leakage guard).

### OPEN QUESTIONS

**OQ-3A: Does `PATCH /agents/{id}/status` accept a single transition or batched transitions?**
Single transition per request. The state machine is a deterministic graph; batching adds no value
in v0.1 and complicates the transition-matrix enforcement in STATE-02. Batch may be needed for
v1 system-initiated multi-agent transitions, but defer.
**Resolution for Plan 02:** Single `{ to, break_reason_id? }` per PATCH call.

**OQ-3B: How is `engaged_channel` shaped on system-initiated Ready → Engaged transitions?**
System-initiated means an external adapter calls the API (or an internal goroutine acts). The
endpoint shape must accept `engaged_channel` in the PATCH body:
```json
{ "to": "Engaged", "engaged_channel": "voice" }
```
This is NOT an agent-togglable transition (STATE-02) but the same endpoint is used. The spec must
mark `engaged_channel` as required when `to == "Engaged"` (discriminated union or description note).
**Resolution for Plan 02:** Single PATCH endpoint; `engaged_channel` required when `to="Engaged"`;
spec note clarifies this is system-initiated only; API authorization for this variant deferred to v1.

**OQ-3C: What does the PATCH success response body look like?**
Two options: (a) return only the new `{ status, state_version, … }` delta; (b) return the full
agent-state object. Option (b) is simpler for UI — one response populates the whole panel.
**Resolution for Plan 02:** Return full agent-state object on success to avoid a follow-up GET.

**OQ-3D: How does the UI poll for state changes from other sources?**
REST polling only in v0.1 (STATE.md decision). UI polls `GET /agents/{id}/status` every ~5s and
compares `state_version`. This is not a spec shape question — no additional endpoint needed.

---

## Sketch 4: Catalog list (generic)

**Validates:** Cursor pagination envelope, `include_disabled`, case-insensitive name search
(CAT-09, CAT-10). Using `agents` as the canonical example entity.

```
┌──────────────────────────────────────────────────────────────────────┐
│  Agents                                    [+ Create agent]          │
├──────────────────────────────────────────────────────────────────────┤
│  Search: [ alice                    ]   [ ] Include disabled          │
│  (case-insensitive substring — CAT-10)   (default: enabled only)     │
│                                                                      │
│  [LOADING STATE]                                                     │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │ ░░░░░░░ | ░░░░░░░░░░░ | ░░░░░░░░░░░░░░ | ░░░ |  ░░░         │   │
│  │ ░░░░░░░ | ░░░░░░░░░░░ | ░░░░░░░░░░░░░░ | ░░░ |  ░░░         │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  [POPULATED STATE]                                                   │
│  | id (truncated) | external_id | name           |enabled| ver | … │
│  |----------------|-------------|----------------|-------|-----|---│
│  | 01901b2c-…     | EMP-0042    | Alice Nguyen   | true  |  7  |[⋮]│
│  | 01901b2d-…     | EMP-0043    | Bob Chen       | true  | 12  |[⋮]│
│  | 01901b2e-…     | EMP-0051    | Carol Davis    | false |  3  |[⋮]│
│  (Carol visible because "Include disabled" is checked)               │
│                                                                      │
│  Showing 3 of 47.  [← Previous]   [Next page →]                     │
│  (cursor-based: Next button enabled when has_more=true;              │
│   Previous needs client-side cursor stack, not server-side)          │
│                                                                      │
│  [EMPTY STATE — no results]                                          │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │ No agents found matching "alice".                            │   │
│  │ [Clear search]                                               │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  [ERROR STATE]                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │ Error loading agents.                                        │   │
│  │ error: internal  request_id: 01901b2c-…                     │   │
│  │ [Retry]                                                      │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  Row context menu [⋮]:  [Edit]   [Disable]   [Delete]               │
└──────────────────────────────────────────────────────────────────────┘
```

**API shape implied:**
- `GET /v1/orgs/{org_id}/agents?name=alice&include_disabled=true&cursor=<opaque>&limit=20`
- Response envelope:
  ```json
  {
    "items": [ { "id", "external_id", "name", "email", "enabled", "version", "created_at" } ],
    "next_cursor": "eyJjcmVhdGVkX2F0IjoiMjAyNi0wNS0xNVQxNDo0MzozN1oiLCJpZCI6IjAxOTAxYjJjLSJ9",
    "has_more": true
  }
  ```
- `next_cursor` is opaque base64 (encodes `{created_at, id}` composite for stable keyset pagination).
- `include_disabled=false` is the default; omitting the param excludes disabled (CAT-09).
- `name` search is case-insensitive substring (see OQ-4B).
- Previous-page navigation: client maintains a stack of cursors seen; no server-side back cursor.

### OPEN QUESTIONS

**OQ-4A: Is the cursor opaque base64 or a structured `?after=<created_at>,<id>` pair?**
Opaque base64 is safer — implementation can change cursor internals without client breakage.
Structured cursor leaks schema (sort column names). The sketch assumes opaque base64.
**Resolution for Plan 02:** Opaque cursor, base64-encoded, no structure guarantee to clients.

**OQ-4B: Is `name` search prefix-only or substring?**
CAT-10 says "case-insensitive name search" but does not specify prefix vs substring.
Substring (`ILIKE '%alice%'`) is more useful but slower. Prefix (`ILIKE 'alice%'`) allows a
trigram/B-tree index. For v0.1 catalog size (hundreds of rows per entity per org), either is fast.
**Resolution for Plan 02:** Substring search (`ILIKE '%?%'`). Spec describes it as
"case-insensitive substring match on name field". Add a `pg_trgm` index recommendation note.

**OQ-4C: How does the `items` array look for entities with nested data (e.g., agents with skills)?**
List endpoint does NOT embed skills. Skills are only embedded on the detail view (OQ-1A resolution).
List returns the flat agent record: `{ id, external_id, name, email, enabled, version, created_at }`.

**OQ-4D: Is there a common envelope schema for all 6 entity list responses, or per-entity responses?**
Common envelope via generic `ListResponse` with `items` typed per entity. In OpenAPI 3.1, use a
reusable `PaginatedList` wrapper in `components/schemas` with `items` typed via the entity schema.

---

## Spec Author Checklist

The following invariants MUST be encoded in `openapi/openapi.yaml`. These are cross-sketch
conclusions — the spec author cannot proceed to Plan 02 without resolving each item.

### 1. Canonical Error Envelope (D-35)

Every `4xx` and `5xx` response (except 413 where middleware may fire before handler — see OQ-2C)
uses this schema, defined once in `components/schemas/ErrorResponse`:

```yaml
ErrorResponse:
  type: object
  required: [error, reason, request_id]
  properties:
    error:
      $ref: "#/components/schemas/ErrorCode"
    reason:
      type: string
      description: Human-readable detail for support and logging.
    request_id:
      type: string
      format: uuid
      description: UUIDv7 minted per-request by RequestID middleware (D-28). Embedded in body for copy-paste DX.
```

### 2. Closed ErrorCode Enum (D-36)

Exactly 10 values — no additions without a spec PR:

```yaml
ErrorCode:
  type: string
  enum:
    - invalid_body
    - invalid_id
    - not_found
    - internal
    - version_conflict
    - cross_org
    - invalid_org_id
    - invalid_transition
    - import_failed
    - rate_limited
```

`rate_limited` is reserved for v0.2; it appears in the enum now so client codegen does not require
rework when rate-limiting lands.

### 3. Three D-37 Special-Case Response Shapes

| Trigger | HTTP Status | Additional fields beyond ErrorResponse |
|---------|------------|----------------------------------------|
| CAT-08 version mismatch | 409 | `current: <EntitySchema>` — server-side record so client shows diff/merge |
| STATE-03 invalid transition | 409 | `from: <AgentState>`, `to: <AgentState>` — the attempted transition pair |
| IMP-05 mixed import result | 207 | NOT an error envelope — separate `BulkImportResult` schema |

`BulkImportResult`:
```yaml
BulkImportResult:
  type: object
  required: [succeeded, failed]
  properties:
    succeeded:
      type: array
      items: { type: string, format: uuid }
    failed:
      type: array
      items:
        type: object
        required: [row, field, error, reason]
        properties:
          row:    { type: integer, minimum: 1 }
          field:  { type: string }
          error:  { $ref: "#/components/schemas/ErrorCode" }
          reason: { type: string }
```

### 4. Pagination Envelope (applies to all 6 entity list endpoints)

```yaml
# Reuse via allOf or $ref per entity-specific items type
PaginatedList:
  type: object
  required: [items, has_more]
  properties:
    items:
      type: array
      items: {}   # overridden per endpoint via allOf
    next_cursor:
      type: string
      nullable: true
      description: Opaque base64 cursor. Null when has_more is false.
    has_more:
      type: boolean
```

### 5. `proficiency` Integer Constraints (CAT-03)

Every schema that carries a skill assignment MUST include:
```yaml
proficiency:
  type: integer
  minimum: 1
  maximum: 10
```
Applies to: `AgentSkillAssignment` (embedded in Agent detail), `PATCH /agents/{id}` request body
`skills[]` items, and the `BulkImportResult` `failed[]` reason text when the field is `proficiency`.

### 6. UUIDv7 Identifier Convention (D-19, D-20)

All `id`, `org_id`, `external_id` (when UUID) fields use:
```yaml
type: string
format: uuid
description: "UUIDv7 (RFC 9562 §5.7). Server-minted at write boundary. Clients treat as opaque."
```
The spec does NOT enforce UUIDv7 via a custom JSON Schema format — that would require a custom
format validator. The server enforces it at the handler level (D-20 pattern).

### 7. `X-Org-Id` Header (FOUND-03, D-21)

Every `/v1/…` path requires:
```yaml
security:
  - OrgHeader: []
components:
  securitySchemes:
    OrgHeader:
      type: apiKey
      in: header
      name: X-Org-Id
```
Bypass routes (`/healthz`, `/readyz`, `/metrics`, `/openapi.yaml`, `/docs`) do NOT include
this security requirement in their path definitions.

---

*Sketches completed: 2026-05-15*
*Wave gate: Resolve all OPEN QUESTIONS above before committing `openapi/openapi.yaml` (Plan 02).*
