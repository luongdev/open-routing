# Phase 5: Bulk Import (Go) - Context

**Gathered:** 2026-05-17
**Status:** Ready for planning

> **Decision-ID convention:** Phase-local prefix `D5-NN`. Phase 5 is being discussed
> out of roadmap order (Phase 3 and Phase 4 still to plan). Phase-local prefixing
> prevents collisions when Phase 3/4 later allocate their own decision IDs from
> the continuous Phase 1 (D-01..D-31) + Phase 2 (D-32..D-48) sequence.

<domain>
## Phase Boundary

Implement a single synchronous bulk import endpoint that upserts JSON or CSV
rows for any of the 6 catalog entities (agents, skills, queues, channels,
adapters, break_reasons) into Phase 3's entity tables, returns
HTTP 200/207/422 with the locked `BulkImportResult` envelope, and persists
results in a new `import_jobs` table for GET retrieval. The endpoint is the
**first** concrete consumer of Phase 2's `BulkImportCatalog` /
`GetImportJob` strict-server stubs and **must not** introduce any new path
into `openapi.yaml` — schema extensions to existing components are allowed
(and required: `ImportJob.status`, `Idempotency-Key` header,
`Import*Request` per-entity schemas).

**In scope:**
- `services/api/internal/import_pkg/` (or analog) — JSON streaming decoder,
  CSV parser with BOM/CRLF/UTF-8 enforcement, typed preprocessing pipeline
  (`trim → lower → split → parse` dispatched by sqlc field type),
  per-entity row validators, upsert orchestration with batched-savepoint
  transactions (50-row chunks).
- New migration: `import_jobs` table (id, org_id, entity_type, status, total_rows,
  succeeded_rows, failed_rows, errors JSONB, idempotency_key NULLABLE,
  created_at, updated_at) with `UNIQUE (org_id, idempotency_key)` partial index.
- sqlc queries authoring the explicit `org_id = $N` filter for `import_jobs`
  and any per-entity upsert helpers Phase 5 introduces (or reuses from Phase 3).
- Concrete `BulkImportCatalog` + `GetImportJob` handlers in
  `services/api/internal/server/stubs.go` (replacing the
  `not_implemented` 500 returns).
- `openapi.yaml` schema additions:
  `ImportAgentRequest`, `ImportSkillRequest`, `ImportQueueRequest`,
  `ImportChannelRequest`, `ImportAdapterRequest`, `ImportBreakReasonRequest`
  (mirror `Create*Request` but reference FKs by `code` — the universal
  user-facing identifier introduced by Phase 04.1 / D04_1-01; `external_id`
  remains as optional integration-mapping metadata per D04_1-07); add
  `status` enum to `ImportJob`; add optional `Idempotency-Key` header
  parameter to the `BulkImportCatalog` operation. Run `task gen` and commit
  generated diff.
- Background goroutine in `cmd/api` that sweeps `import_jobs` rows where
  `status='pending'` AND `updated_at < now() - interval '24 hours'` →
  flips to `status='failed'` with `errors` containing one synthetic entry
  `{row: 0, error: 'import_failed', reason: 'server_crash'}`. Pattern
  mirrors Phase 4 STATE-07 WrapUp TTL goroutine.
- Integration tests in `services/api/test/import/` (one suite per
  entity-type × format matrix; minimum: agents+JSON, agents+CSV,
  break_reasons+CSV, full-failure 422 case, partial-success 207 case,
  oversized 413 case, idempotency-key replay case, BOM/CRLF/Excel
  `testdata/windows-excel.csv` golden file).

**Out of scope:**
- Async import pathway (IMP-09 deferred to v0.2 — 413 message points users
  to the deferred capability).
- Dry-run import mode (IMP-10).
- Error CSV download (IMP-11 — `BulkImportFailedRow` deliberately does not
  echo raw row content per Phase 2 OQ-2A).
- Multi-entity batches (one entity-type per POST via `?entity=`).
- Real-time progress polling (synchronous in v0.1; `ImportJob.status` enum
  is added for forward-compat with v0.2 async but only the terminal
  states are visible to v0.1 admins).
- Cross-org imports (orgDB rejects any SQL lacking `org_id = $N` —
  `cmd/migrate` is the only WithBypass caller and import does not use it).
- Cache invalidation for Phase 3's Redis layer (Phase 3 owns CAT-11
  cache; Phase 5 calls the per-entity write helper which will already
  invalidate `or:{orgId}:{entity}:{id}` keys per CAT-11 semantics —
  no separate import-cache logic).
- Distributed lock for the TTL cleanup goroutine (single-replica only in
  v0.1; flagged for v1 multi-replica per STATE.md blockers).

</domain>

<decisions>
## Implementation Decisions

### CSV Cell Type Coercion (Area 1)

- **D5-01:** **Preprocessing pipeline pattern.** Every CSV cell goes through a
  typed pipeline `trim → lower → split → parse` BEFORE validation.
  The pipeline is dispatched by the DB field type (resolved from sqlc-generated
  Go types — `bool`, `int`, `string`, `pgtype.Text`, etc.). One central
  dispatcher; per-type coercer; per-entity validator runs only on
  post-coercion typed values. Goal: shrink the "weird input" surface so
  validators only see well-shaped data and can focus on business rules.
- **D5-02:** **Bool coercion is loose.** Accept `true|false|TRUE|FALSE|1|0|yes|no`
  (case-insensitive after `trim → lower`). Any other value → per-row
  failure with `reason: invalid_bool`. Friendly to admins exporting from
  Excel/Google Sheets where bool conventions vary.
- **D5-03:** **Empty cells = null/skip.** An empty cell (`,,`) means "field
  absent." On create: the target entity's spec default applies
  (e.g., `enabled` defaults to `true`). On update: the field is left
  unchanged. On a required-field column: per-row failure with
  `reason: missing_required`. No explicit `\N` / `NULL` marker — empty
  is the universal signal.
- **D5-04:** **Multi-value cells are delimited.** Primary separator is `|`;
  fallback parsers also accept `;` and `,` (priority `|` > `;` > `,`).
  The CSV parser already unescapes the field-level `,` (RFC 4180 quoted
  fields), so sub-splitting by `,` only happens inside the unescaped
  cell value. Document the convention in the OpenAPI description for any
  multi-value field (`channels.handled_kinds`, agent `skills`, etc.).
- **D5-05:** **Header policy is strict, batch-level.** Missing a required
  column for the target entity → HTTP 400 `{error: invalid_body, reason:
  missing_columns, missing: [...]}` — no rows processed. Unknown column
  → HTTP 400 `{error: invalid_body, reason: unknown_columns, unknown:
  [...]}`. Header validation happens BEFORE the first data row is read.
  Rationale: PITFALLS 5.4 explicitly warns against silently dropping
  v0.1 columns when an admin's saved v0.2 template arrives.
- **D5-06:** **Enums are case-insensitive auto-lowered.** `voice`, `Voice`,
  `VOICE` all coerce to `voice` before matching the closed enum set
  (`voice|chat|email` for channel kind, `true|false` for
  break_reason.routable, etc.). No-match → per-row failure with
  `reason: invalid_enum`.
- **D5-07:** **Integers tolerate leading float.** Pipeline:
  `trim → strconv.ParseFloat → math.Trunc → int`. `"7"`, `" 7 "`, `"7.0"`,
  `"7.5"` all coerce to `7` (truncation, never rounding). Pure-string
  `"seven"` → per-row failure `reason: invalid_int`. Rationale: Excel
  silently rewrites `7` to `7.0` when re-saving an `.xlsx` as CSV;
  strict `strconv.Atoi` would fail otherwise-valid columns. Truncation is
  documented in the OpenAPI description for any int field that admits
  CSV import.
- **D5-08:** **Charset is UTF-8 only.** After BOM stripping (IMP-02), the
  remaining bytes are validated with `utf8.Valid([]byte)`. Invalid
  sequences → HTTP 400 `{error: invalid_body, reason: csv_not_utf8}`.
  No `iso-8859-1` fallback. PITFALLS 5.1 lists charset auto-detect as
  the #1 silent-corruption source.

### Transactional Model + Jobs Lifecycle (Area 2)

- **D5-09:** **Batched-savepoint transactions; chunk size = 50 rows.** The
  500-row max body splits into ≤ 10 chunks. Each chunk opens one
  transaction, processes 50 rows with one SAVEPOINT each
  (`SAVEPOINT row_{i}` → upsert → `RELEASE` on success / `ROLLBACK TO`
  on failure), then commits. Failed rows do not abort the chunk; chunk
  abort happens only on infrastructure error (connection drop, deadlock
  after retry). Rationale: ~10× faster than per-row autocommit due to
  network round-trip amortisation, and atomic with respect to chunk
  boundaries (admin sees coherent partial results).
- **D5-10:** **`import_jobs` row created at start, finalised at end.**
  INSERT `import_jobs (id, org_id, entity_type, status='pending',
  total_rows=N, succeeded_rows=0, failed_rows=0, errors=NULL,
  idempotency_key, created_at, updated_at)` BEFORE chunk 1 opens.
  After chunk 10 commits, UPDATE row with final counters, `errors`
  JSONB, `status='completed'`, `updated_at`. Job INSERT is its own
  short transaction (independent of chunk tx-es) so admin GET can see
  the job even mid-import.
- **D5-11:** **Crash-recovery sweep goroutine; 24h TTL.** Reuse Phase 4
  STATE-07 background-goroutine pattern (single-replica `time.Ticker`
  in `cmd/api`; v1 multi-replica needs a distributed lock — flagged in
  STATE.md). Tick interval: 1 hour. Query: `UPDATE import_jobs SET
  status='failed', errors='[{"row":0,"error":"import_failed",
  "reason":"server_crash"}]', updated_at=now() WHERE status='pending'
  AND updated_at < now() - interval '24 hours' RETURNING id, org_id`.
  Log each transition at `slog.Warn` with `event=import_crash_sweep`.
- **D5-12:** **`errors` JSONB shape mirrors `BulkImportResult.failed[]`
  exactly.** `[{"row": 3, "field": "email", "error": "import_failed",
  "reason": "invalid email format"}, ...]`. No timestamps, no
  raw_value, no attempt count. Rationale: GET endpoint returns the
  same shape the POST returned — zero transformation, zero divergence.
- **D5-13:** **`Idempotency-Key` header is optional and persisted.** Header
  semantics: client-generated UUIDv7 (per RFC-style idempotency-key
  pattern). Column `import_jobs.idempotency_key TEXT NULL` with partial
  unique index `CREATE UNIQUE INDEX ON import_jobs (org_id,
  idempotency_key) WHERE idempotency_key IS NOT NULL`. Behaviour: if
  client sends `Idempotency-Key: K`:
  1. Look up `(org_id, K)`. Hit → return the persisted job's final
     `BulkImportResult` payload (re-serialise from `errors` JSONB +
     `succeeded_rows`/`failed_rows`); do NOT re-import.
     **Important:** the persisted job does NOT carry the full
     `succeeded` IDs (we store counters only). The replay response
     returns `succeeded: [...]` reconstructed from… see deferred (this
     is a known limitation; v0.1 returns `{succeeded: [], failed:
     [...as-stored], idempotent_replay: true}` to avoid lying about the
     ID set — planner should confirm shape extension on `BulkImportResult`).
  2. Miss → proceed normally; persist key on the new row.
  No header → new job each POST (data is still idempotent at the row
  level via `ON CONFLICT (org_id, external_id) DO UPDATE`; only the
  `import_jobs` row count grows).
- **D5-14:** **`?entity=` query is canonical; `Content-Type` selects parser.**
  Generated `BulkImportCatalogRequestObject` already exposes `JSONBody
  *BulkImportCatalogJSONRequestBody` AND `Body io.Reader` — oapi-codegen
  populates `JSONBody` for `application/json` and `Body` for `text/csv`.
  Handler dispatches: `if req.JSONBody != nil` → JSON path; else → CSV
  path on `req.Body`. Unsupported Content-Type → strict-server's built-in
  415 (verify in planning; add to spec `requestBody.content` map if
  oapi-codegen does not auto-reject).

### N:M Skills in Import (Area 3)

- **D5-15:** **Import uses per-entity `Import*Request` schemas — NOT
  `Create*Request`.** New `components.schemas`: `ImportAgentRequest`,
  `ImportSkillRequest`, `ImportQueueRequest`, `ImportChannelRequest`,
  `ImportAdapterRequest`, `ImportBreakReasonRequest`. Each mirrors its
  `Create*Request` BUT references FK relationships by the target
  entity's `code` (the universal user-facing canonical identifier per
  Phase 04.1 / D04_1-01) instead of by `id` (server-minted UUIDv7).
  Rationale: "tư duy humanable" — admins coming from external systems
  don't have Open Routing UUIDs; they have their own stable codes.
  Phase 04.1 promoted `code` from a per-entity convention to a universal
  contract (composite `UNIQUE (org_id, code)` across all 6 catalog
  entities), making it the natural upsert key. `external_id` remains
  supported on `Create*Request` as optional integration-mapping metadata
  (D04_1-07), but the Phase 5 import upsert key is `code` via
  `UpsertXByCode` (authored in Phase 04.1 Plan 03). Forcing UUIDs would
  make CSV imports impossible without a pre-import lookup pass.
- **D5-16:** **JSON agent imports support nested `skills[]`.**
  `ImportAgentRequest.skills: [{skill_code: "skill_voice",
  proficiency: 7}, ...]`. Per row: lookup `(org_id, skill_code)`
  → skill UUID. Unknown skill → per-row failure
  `{field: "skills[0].skill_code", reason: "unknown_skill"}` —
  the agent itself does NOT import; the whole row fails (skills are
  intrinsic to a complete agent record). Note: `skill_code` values
  follow Phase 04.1's regex `^[a-z][a-z0-9_]{0,63}$` (D04_1-03); illegal
  token shape → per-row failure
  `{field: "skills[N].skill_code", reason: "invalid_code_format"}`
  (handler validates via `validateCodeFormat` from Phase 04.1 Plan 03).
  Re-running after creating the missing skill is the documented
  recovery path.
- **D5-17:** **CSV agent imports support skills via a single `skills`
  column with `code:prof|code:prof` syntax.** Example cell:
  `"skill_voice:7|skill_chat:9|skill_email:5"` (lowercase per Phase 04.1
  D04_1-03 regex). Pipeline: trim → split by D5-04 separator priority →
  for each token, split on `:` → validate `[code, prof]`. Missing `:` →
  per-row `reason: invalid_skill_token`. Non-int proficiency →
  `reason: invalid_skill_token`. Token whose `code` part fails the
  `^[a-z][a-z0-9_]{0,63}$` regex → `reason: invalid_code_format` (handler
  reuses `validateCodeFormat` from Phase 04.1 Plan 03). Same
  `unknown_skill` resolution rule as D5-16. Document the lowercase
  convention in the CSV column description AND in the admin
  import-screen UI copy (Phase 6).
- **D5-18:** **Skill merge semantics on agent update (PATCH-like, NOT
  PUT).** When an import row updates an existing agent (matched by
  `code` via `UpsertAgentByCode` from Phase 04.1 Plan 03), the `skills`
  array MERGES into the agent's existing `agent_skills` join rows —
  existing skills NOT in the import payload are LEFT INTACT. This is a
  documented divergence from `UpdateAgentRequest` (which replaces the
  whole set, per Phase 2 OQ-1A). Rationale: bulk imports often arrive
  partial (HR system exports a delta); preserving existing assignments
  avoids unintended skill loss. **Trade-off:** admins cannot REMOVE a
  skill via import — they must use UI or the dedicated agent PATCH API.
  **Phase 04.1 alignment:** `code` is immutable post-create (D04_1-02);
  `external_id` may be added or rebound on update via
  `ImportAgentRequest.external_id` (optional field) — admins can use the
  import to attach a new external-system binding to a code-keyed agent.
- **D5-19:** **Proficiency conflict on existing skill → import value wins.**
  SQL: `INSERT INTO agent_skills (agent_id, skill_id, proficiency)
  VALUES (...) ON CONFLICT (agent_id, skill_id) DO UPDATE SET
  proficiency = EXCLUDED.proficiency`. Rationale: import is treated as
  the authoritative source for proficiency on the skills it includes
  (per D5-18 merge model); the import author explicitly chose to
  include this skill with this proficiency.
- **D5-20:** **Other entity types have no N:M in v0.1 import scope.**
  Skills/queues/channels/adapters/break_reasons are flat in import
  (no nested arrays). Future Queue↔Channel and Channel↔Adapter
  relationships (if any land in v0.2) follow the same pattern: nested
  by `external_id` in JSON, pipe-delimited in CSV. v0.1 keeps the
  surface minimal.

### Body Limit + Parse Strategy (Area 4)

- **D5-21:** **50 MB cap enforced via Content-Length pre-flight +
  `http.MaxBytesReader` wrap.** Order: (a) read `Content-Length`
  header; if present AND > `50<<20` → HTTP 413 immediately, no body
  read; (b) regardless of `Content-Length`, wrap `r.Body` in
  `http.MaxBytesReader(w, r.Body, 50<<20)` BEFORE any parser sees it.
  This protects against missing/lying `Content-Length` and chunked
  transfer encoding. Mid-stream over-limit → `MaxBytesError` mapped to
  HTTP 413. Reusable as a middleware factory in
  `services/api/internal/middleware/bodylimit.go` so future write
  endpoints can opt in.
- **D5-22:** **500-row cap enforced inline during streaming parse.** JSON:
  `json.Decoder` in token-mode — open `[` token, then loop with
  `decoder.More()` and `decoder.Decode(&item)`; increment row counter;
  exit immediately on row 501 with HTTP 413. CSV: `csv.Reader.Read()`
  loop counts data rows (skip header); exit on row 501 with HTTP 413.
  Neither path ever loads the full body into memory at once. Rationale:
  spec mandates 500-row hard cap; failing fast at row 501 (vs parsing
  all then counting) means a malicious 50MB CSV that's 1M rows of 50
  bytes each terminates at ~25KB read.
- **D5-23:** **JSON parsing is streaming, not eager.** `json.Decoder`
  token-mode (D5-22). Memory ~O(one row's struct). Per-row error
  handling: decode failure on row N → record `{row: N, field: "", error:
  "import_failed", reason: "invalid_json: <err>"}` in `failed[]`,
  continue with row N+1 IF the JSON stream is still recoverable
  (`decoder.Decode` returning a non-syntactic err keeps the stream
  pointer); on syntactic error (e.g., missing closing bracket) abort
  with HTTP 400 `{error: invalid_body, reason: malformed_json}`.
- **D5-24:** **Content-Type dispatch is header-only (strict).** Handler
  looks at `req.JSONBody` (oapi-codegen-filled for `application/json`)
  vs `req.Body` (CSV path). Missing/unrecognised Content-Type → 415
  Unsupported Media Type from the strict-server layer (verify
  generated behaviour). NO magic-byte sniffing, NO `?format=` query.
  Rationale: minimal ambiguity, REST-conformant.

### OpenAPI Spec Extensions Required (Implied Follow-On)

The decisions above require these `openapi/openapi.yaml` changes; planner
must include them in Wave 1 (BEFORE handler implementation, so codegen
flows the new shapes to `server.gen.go` and `types.gen.go`):

- **D5-25:** Add 6 new request schemas:
  `ImportAgentRequest`, `ImportSkillRequest`, `ImportQueueRequest`,
  `ImportChannelRequest`, `ImportAdapterRequest`,
  `ImportBreakReasonRequest`. Each mirrors `Create*Request` with FK
  fields swapped to `*_code` (string, regex `^[a-z][a-z0-9_]{0,63}$`
  per Phase 04.1 D04_1-03). Concretely: replace each
  `agent_external_id`, `skill_external_id`, `queue_external_id`,
  `channel_external_id`, `adapter_external_id`,
  `break_reason_external_id` with `agent_code`, `skill_code`,
  `queue_code`, `channel_code`, `adapter_code`, `break_reason_code`
  respectively. `ImportAgentRequest` also carries
  `skills: [{skill_code, proficiency}]` (was
  `{skill_external_id, proficiency}`). Each Import*Request retains an
  optional `external_id` field for integration-mapping (mutable per
  D04_1-07; not the FK target). Update the `BulkImportCatalog`
  `requestBody.content.application/json` schema from `items: {}` to
  `oneOf: [<6 schemas>]` (or per-entity discriminator — planner picks;
  `oneOf` is the OAS 3.0-compatible path since the spec was downgraded
  from 3.1). **Note (oapi-codegen oneOf concern):** Phase 5's
  05-RESEARCH.md flagged that oapi-codegen v2 has limitations with
  oneOf in strict-server mode; the planner must address this
  (per-entity discriminator may be necessary). This is independent of
  the identity-model rename.
- **D5-26:** Extend `ImportJob` schema with
  `status: { type: string, enum: [pending, completed, failed] }`
  (required). Forward-compat with v0.2 async; v0.1 returns
  `completed` or `failed` only (sync path), `pending` is only visible
  during a server-crash window or via the cleanup goroutine sweep.
- **D5-27:** Add optional `Idempotency-Key` header parameter to the
  `BulkImportCatalog` operation:
  `name: Idempotency-Key, in: header, required: false, schema: {type:
  string, format: uuid, description: "Client-generated UUIDv7 for
  retry-safe POST"}`. Add `idempotent_replay: { type: boolean, default:
  false }` field to `BulkImportResult` so callers can detect a
  replayed response (per D5-13 known limitation around succeeded[] ID
  set).

### Claude's Discretion

- Exact internal package layout: `services/api/internal/import_pkg/` vs
  `services/api/internal/catalog/import/` — planner picks based on
  Phase 3's emerging catalog package layout (Phase 3 hasn't planned
  yet; coordination needed).
- Goroutine sweep tick interval (recommend 1h — balance freshness vs DB load).
- `import_jobs` migration filename: planner chooses sequence number per
  golang-migrate convention (likely `000NNN_create_import_jobs.up.sql`
  AFTER Phase 3's entity migrations land — depends on Phase 3 plan
  ordering).
- Whether per-entity `Import*Request` lives in `openapi/openapi.yaml`
  inline or in a separate `openapi/components/imports.yaml` bundle file
  (Phase 2 deferred spec-splitting; revisit if `openapi.yaml` crosses
  ~3000 lines).
- Test data file convention: `testdata/<entity>-<scenario>.{json,csv}`
  vs `testdata/<scenario>/<entity>.{json,csv}` — planner picks. MUST
  include a real Excel-exported `testdata/windows-excel-agents.csv`
  with UTF-8 BOM + CRLF per PITFALLS 5.1.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level locks
- `.planning/PROJECT.md` — Locked stack (Go + chi + pgx + sqlc + golang-migrate
  + slog + OTel). `org_id` naming locked. REST polling only in v0.1; async
  import (IMP-09) and worker-based CSV preview (WORK-02) deferred to v0.2.
- `.planning/REQUIREMENTS.md` §Bulk Import — **IMP-01 through IMP-08**.
  Acceptance criteria for this phase. Note `?schema_version=v0.1` (IMP-08),
  50 MB / 500 row cap (IMP-07), HTTP 207 partial-success contract (IMP-05),
  `import_jobs` persistence shape (IMP-06).
- `.planning/REQUIREMENTS.md` §Future Requirements — **IMP-09, IMP-10, IMP-11**.
  These are the v0.2 deferrals Phase 5 explicitly does NOT implement; the 413
  message points users to IMP-09's async pathway.
- `.planning/ROADMAP.md` §Phase 5 — Goal statement and 5 success criteria.
- `.planning/STATE.md` §Blockers/Concerns — flagged: "Import schema version
  communication" (no customer notification mechanism for v0.2 schema
  changes) and "WrapUp TTL with multi-replica" (same goroutine pattern
  Phase 5 reuses for crash sweep).

### Phase 1 carry-forward (LOCKED — do not rewrite)
- `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` —
  **All D-01 through D-31 are locked.** Especially:
  - **D-01..D-05:** `orgDB` wrapper validates every SQL for `org_id = $N`;
    panics in dev/test, errors in prod; `cmd/migrate` is the ONLY legitimate
    `WithBypass` caller. Phase 5 NEVER calls `WithBypass`.
  - **D-19, D-20:** UUIDv7+ everywhere; `import_jobs.id` minted with
    `uuid.NewV7()` at handler entry.
  - **D-21:** Middleware bypass list `{/healthz, /readyz, /metrics}` —
    `/v1/orgs/{org_id}/catalog/import` and `/v1/orgs/{org_id}/imports/{id}`
    are BOTH gated by OrgContext middleware (no exception).
  - **D-23:** Internal package layout (`internal/api/`, `internal/db/`,
    `internal/middleware/`, etc.) — Phase 5 follows the same convention
    for any new package (e.g., `internal/import_pkg/`).
  - **D-28, D-29:** RequestID middleware mints UUIDv7; slog handler
    injects `trace_id`/`span_id`. `BulkImportFailedRow` `reason` field
    should reference the request_id is already in the ErrorResponse
    envelope at the batch level.

### Phase 2 carry-forward (LOCKED contract)
- `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` —
  **All D-32 through D-48 are locked.** Especially:
  - **D-35..D-37:** Error envelope `{error, reason, request_id}` for batch-level
    errors. `BulkImportResult` (D-37) is its own schema, NOT an error
    envelope; `failed[]` items use the per-row error shape. Phase 5
    handler MUST emit batch-level errors via the existing
    `middleware.WriteError(ctx, w, status, code, reason)` helper for
    400/413/415/500 paths; the 200/207/422 success-ish paths emit
    `BulkImportResult` directly via the generated
    `BulkImportCatalog{200,207,422}JSONResponse` types.
  - **D-41..D-44:** Strict-server flavor — Phase 5 implements
    `StrictServerInterface.BulkImportCatalog(ctx, req)` and
    `GetImportJob(ctx, req)` in `services/api/internal/server/stubs.go`
    (replacing the `not_implemented` 500 returns). Generated request
    objects already carry `JSONBody *BulkImportCatalogJSONRequestBody`
    AND `Body io.Reader` for CSV — handler dispatches on which is
    non-nil per D5-14/D5-24.
  - **D-46:** Scaffold migration pattern — Phase 5's `import_jobs`
    handlers proves another full pipeline (spec change → codegen →
    handler) end-to-end after Phase 3's catalog migrations land. Phase
    5 does NOT touch the scaffold (Phase 3 deletes scaffold).
  - **D-47, D-48:** CI drift gate (`task gen` + `git diff --exit-code`)
    + Redocly spec lint. Phase 5's spec additions (D5-25..D5-27) MUST
    pass both. Wave 1 task: edit `openapi.yaml`, run `task gen`,
    commit generated diff together with the spec change.

### Generated code (READ before implementing handler)
- `services/api/internal/api/server.gen.go` lines ~95, 1375–1430, 3887–3920,
  5145–5934 — `BulkImportCatalog` strict-server signature,
  `BulkImportCatalogRequestObject` (carries `JSONBody` + `Body io.Reader`),
  response objects (`BulkImportCatalog200JSONResponse`,
  `BulkImportCatalog207JSONResponse`, `BulkImportCatalog413JSONResponse`,
  etc.). Phase 5 implements against these EXACT signatures; the strict-server
  layer auto-validates Content-Type + auto-marshals responses.
- `services/api/internal/api/types.gen.go` — `BulkImportResult`,
  `BulkImportFailedRow`, `ImportEntityType`, `ImportJob` Go types
  derived from the OpenAPI schemas. Phase 5's `Import*Request` schemas
  (D5-25) will materialise here after `task gen` runs.
- `services/api/internal/server/stubs.go:230` + `:254` — current
  `not_implemented` stubs for `BulkImportCatalog` and `GetImportJob`.
  Replace with real implementations.

### Reusable infrastructure (READ for patterns)
- `services/api/internal/middleware/httputil.go` —
  `WriteError(ctx, w, status, code, reason)` is the canonical 4xx/5xx
  writer for batch-level errors. The doc comment EXPLICITLY mentions
  Phase 5's per-row shape as the "non-extending follow-on pattern" —
  do NOT add per-row fields to `errorBody`; emit
  `BulkImportResult.failed[]` separately.
- `services/api/internal/middleware/requestid.go` — UUIDv7 mint
  pattern; Phase 5 uses `uuid.NewV7()` directly for `import_jobs.id`.
- `services/api/internal/db/orgdb.go` — `OrgDB.Exec/Query/QueryRow`
  preflight. Phase 5's new sqlc-generated `*Queries` for `import_jobs`
  MUST be constructed against `OrgDB`, NOT raw pgx pool. Test asserts
  this by attempting an `import_jobs` insert without `org_id` in ctx
  and expecting `ErrOrgIDMissingFromContext`.
- `services/api/internal/db/queries/scaffold.sql` — sqlc query
  authoring convention (explicit `WHERE org_id = $N` in every read,
  `(id, org_id, ...)` in every insert). Phase 5 follows for
  `import_jobs.sql`.
- `services/api/internal/testsupport/postgres.go` + `migrate.go` +
  `seed.go` + `httpclient.go` — testcontainers-go integration test
  harness. Phase 5 reuses for the entity-type × format matrix.

### Research (LOCKED — these analyse exact pitfalls Phase 5 must avoid)
- `.planning/research/PITFALLS.md` §5 (Bulk Import Pitfalls):
  - **5.1 (Encoding / BOM / CRLF Surprises)** — drives D5-08 (UTF-8 only),
    locks `testdata/windows-excel-agents.csv` requirement.
  - **5.2 (Partial-Success Semantics Not Communicated)** — drives D5-10
    (job-row at start) + D5-12 (errors JSONB shape).
  - **5.3 (Large File Upload Times Out at Proxy / CDN Layer)** — drives
    D5-21 (Content-Length pre-flight + MaxBytesReader). NOTE: research
    says "5MB sync limit"; spec locks 50MB. Reverse-proxy config
    (Nginx `client_max_body_size 55m`, ALB read timeout) is OUT OF
    SCOPE for Phase 5 (no proxy in v0.1 docker-compose) but should be
    documented as a deploy-time concern.
  - **5.4 (Schema Drift Between v0.1 and v0.2)** — drives D5-05
    (strict header policy). Also `?schema_version=v0.1` (IMP-08).
  - **1.3 (Bulk Import Writes Cross-Org Rows)** — covered by D-01..D-05
    orgDB wrapper; Phase 5 integration test MUST include the two-org
    isolation case for `import_jobs` (POST as org A, assert org B's
    GET returns 404).
  - **4.4 (`code` Collisions During Bulk Import)** — covered by
    Phase 04.1's schema `UNIQUE (org_id, code)` per entity (D04_1-01).
    Phase 5's integration test asserts org A and org B can both
    import the same `code` values without conflict (cross-org
    isolation; mirrors Phase 04.1 Plan 05's
    `TestCatalog_CrossOrgSameCode_BothSucceed` in
    `services/api/test/isolation/catalog_test.go`). The legacy
    `(org_id, external_id)` constraint was demoted to a partial unique
    index `WHERE external_id IS NOT NULL` (D04_1-06) and is enforced
    only when a row carries an explicit external-system mapping.
- `.planning/research/SUMMARY.md` §Bulk Import (v0.1 Scope) — context
  on the sync-only / partial-success / fast-csv (now Go `encoding/csv`)
  decision lineage.

### External standards (READ for Go idioms)
- [Go `encoding/csv` package](https://pkg.go.dev/encoding/csv) — `csv.Reader`
  with `LazyQuotes=false` (strict RFC 4180), `FieldsPerRecord=0` (allow
  variable column count? — planner verifies; recommend pinning to header
  width). BOM handling is NOT built-in; Phase 5 strips manually before
  passing to `csv.Reader`.
- [Go `encoding/json` Decoder token-mode example](https://pkg.go.dev/encoding/json#example-Decoder.Decode-Stream)
  — the canonical streaming-decode pattern for large JSON arrays.
- [`http.MaxBytesReader`](https://pkg.go.dev/net/http#MaxBytesReader) +
  [`MaxBytesError`](https://pkg.go.dev/net/http#MaxBytesError) — Go 1.19+
  typed error for mid-stream over-limit detection.
- [PostgreSQL SAVEPOINT semantics](https://www.postgresql.org/docs/17/sql-savepoint.html)
  — the only correct way to do per-row partial-success inside one
  transaction. `pgx` exposes this via `tx.Exec(ctx, "SAVEPOINT ...")`
  / `"ROLLBACK TO ..."` / `"RELEASE ..."`.
- [RFC 4180 (CSV)](https://datatracker.ietf.org/doc/html/rfc4180) — base
  format; D5-04 multi-value separator hierarchy and D5-08 UTF-8 charset
  layer ON TOP.
- [RFC 9562 §5.7 (UUIDv7)](https://www.rfc-editor.org/rfc/rfc9562.html#section-5.7)
  — `import_jobs.id` minted via `uuid.NewV7()`.
- Idempotency-Key draft RFC pattern:
  [draft-ietf-httpapi-idempotency-key-header](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)
  — D5-13 follows this draft's semantics (client-generated key, server
  persists, replay returns prior response).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`middleware.WriteError(ctx, w, status, code, reason)`** at
  `services/api/internal/middleware/httputil.go` — canonical 4xx/5xx
  emitter; embeds `request_id` from ctx; Phase 5 uses for all batch-level
  errors (400/413/415/500).
- **`middleware.WriteJSON(w, status, body)`** at same file — generic
  success writer; Phase 5 does NOT use directly (strict-server
  `Visit*Response` methods auto-marshal `BulkImportResult` and
  `ImportJob`).
- **`db.OrgDB`** at `services/api/internal/db/orgdb.go` — wraps pgxpool;
  every `Exec/Query/QueryRow` runs SQL preflight against the `org_id`
  validator. Phase 5's new `import_jobs` queries construct sqlc
  `*Queries` against `OrgDB` (NEVER against `*pgxpool.Pool` directly).
- **`compositeServer`** at `services/api/internal/server/stubs.go` —
  the strict-server implementation struct. Phase 5 adds real method
  bodies for `BulkImportCatalog` and `GetImportJob` here (or wires
  them to a sub-handler struct that `compositeServer` embeds).
- **`testsupport.{Postgres, Migrate, Seed, NewHTTPClient}`** at
  `services/api/internal/testsupport/` — testcontainers-go harness +
  migration runner + HTTP test client. Phase 5 integration tests
  follow Phase 1's isolation suite pattern (`test/isolation/`).
- **Generated `BulkImportCatalogRequestObject` + response objects** at
  `services/api/internal/api/server.gen.go` (~lines 3887, 5145, 5914) —
  Phase 5 implements against these; oapi-codegen handles Content-Type
  dispatch, request decoding, response marshaling.

### Established Patterns
- **All-SQL-through-OrgDB** (D-01..D-05) — Phase 5 NEVER calls
  `pool.Exec` directly; every query routes through `OrgDB`. Bypass is
  reserved for `cmd/migrate`.
- **Strict-server handlers as pure functions** (D-41) — handler signature
  is `(ctx, req) → (resp, err)`. No `http.ResponseWriter` plumbing inside
  the handler; oapi-codegen's `Visit*Response` writes the body.
- **UUIDv7 minted at write boundary** (D-19) — `uuid.NewV7()` at handler
  entry for `import_jobs.id`; entity rows get their UUIDs from Phase 3's
  per-entity create helpers (Phase 5 calls those helpers, doesn't mint
  entity UUIDs itself).
- **Canonical error envelope `{error, reason, request_id}`** (D-35) —
  for batch-level errors only. Per-row errors use `BulkImportFailedRow`
  shape (`{row, field, error: "import_failed", reason}`).
- **Background-goroutine TTL sweep** (Phase 4 STATE-07 pattern, to be
  built in Phase 4) — single-replica `time.Ticker` in `cmd/api`. Phase 5
  REUSES the same pattern for the `import_jobs` crash-recovery sweep
  (D5-11). Both have the same multi-replica caveat (STATE.md blockers).
- **Migrations at `/migrations/NNNNNN_name.{up,down}.sql`** (D-10) —
  Phase 5 adds `000NNN_create_import_jobs.up.sql` AFTER Phase 3's
  entity migrations.
- **sqlc query authoring with explicit `org_id = $N`** (scaffold pattern,
  enforced by OrgDB validator) — Phase 5 `import_jobs.sql` follows.

### Integration Points
- **Phase 3 (Catalog CRUD)** — Phase 5 depends on Phase 3's entity tables
  and per-entity write helpers. Phase 3 SHOULD expose:
  - `repo.{Agent,Skill,...}.UpsertByCode(ctx, OrgDB, params)` —
    Phase 5's row processor calls this. Phase 04.1 Plan 03 AUTHORED the
    `UpsertXByCode` sqlc queries for all 6 entities
    (`INSERT ... ON CONFLICT (org_id, code) DO UPDATE SET ... RETURNING *`);
    Phase 5 invokes them. Phase 3 owns the per-entity validation;
    Phase 04.1 owns the upsert SQL; Phase 3's Redis cache invalidation
    (per CAT-11) is reused — the import call site invokes the same
    write helper.
  - `repo.Skill.ResolveCodes(ctx, OrgDB, codes []string) →
    map[string]uuid.UUID, []string unknown` — Phase 5's nested skill
    resolver (D5-16, D5-17) batches lookups via Phase 04.1's
    `GetSkillByCode` query primitive (Plan 03).
  - Phase 5 strongly REQUESTS Phase 3 author both helpers; if Phase 3
    plan doesn't include them, Phase 5 plan must (NOTE: Phase 04.1
    Plan 03 already authored `UpsertXByCode` + `GetXByCode` — this
    request is satisfied as of Phase 04.1 merge).
- **Phase 4 (Agent State Machine)** — minimal coupling. Phase 5
  imports do NOT create `agent_states` rows (those are created by
  Phase 4's first state transition or by a default-state trigger at
  agent insert). Note: planner must verify Phase 4's `agent_states`
  default-row behaviour exists before Phase 5 lands; otherwise newly
  imported agents have no state row and `IsRoutable` queries fail.
- **Phase 6 (Standalone Admin)** — `apps/admin` will add a bulk-import
  screen that POSTs CSV/JSON to this endpoint and renders
  `BulkImportResult.failed[]` as a downloadable error table. No
  Phase 5 deliverable; documented integration target.

</code_context>

<specifics>
## Specific Ideas

- **"Tư duy humanable"** — user emphasised: any field admins type into
  CSV/JSON should reference entities by their human-readable code,
  not server-minted UUIDs. Phase 04.1 promoted `code` from a per-entity
  convention to a universal contract (D04_1-01, composite
  `UNIQUE (org_id, code)` across all 6 catalog entities) — this is the
  canonical identifier Phase 5 imports reference. `external_id` remains
  supported as optional integration-mapping metadata (D04_1-07).
  Drove D5-15..D5-17.
- **Preprocessing pipeline before validation** — user explicitly called
  out: "khi biết kiểu trong db thì nên quyết định trước là có trim,
  lower, split, ... trước không rồi mới process tiếp. Như vậy đỡ được
  khối lỗi." This is the philosophy behind D5-01; planner should
  structure the import package as `parse → coerce → validate → upsert`
  with the coerce step shared across all entity types.
- **Tolerant int parsing for Excel** (D5-07) — `7.0` truncates to `7`.
  User accepted tolerant behaviour; planner should add a debug-level
  slog event when truncation discards a non-zero fractional part
  (`slog.Debug("import: int_truncation", "row", N, "field", F, "raw",
  "7.5", "value", 7)`) for forensics.
- **Pipe-delimited multi-value with separator priority** (D5-04) — user
  asked for "chấp nhận 1 số sep như , | ;". Implementation uses priority
  `|` > `;` > `,` because CSV's field-level `,` delimiter creates
  ambiguity if a cell value happens to contain unquoted commas (rare but
  not impossible — quoted cells handle this, unquoted ones don't).
- **PATCH-not-PUT for skill merge in import** (D5-18) — diverges from
  `UpdateAgentRequest`'s PUT semantics. Rationale: imports often arrive
  partial; PUT would silently delete skills the import didn't include.
  Planner must surface this divergence in CSV column descriptions AND
  in the admin import-screen UI copy (Phase 6).
- **Idempotency-Key returning prior payload** (D5-13) — known limitation:
  v0.1 stores counters + errors but not the full `succeeded[]` ID list,
  so a replay response cannot reconstruct the original `succeeded[]`
  UUIDs. Decision: add `idempotent_replay: bool` to `BulkImportResult`
  (D5-27) so callers detect a replay; the `succeeded[]` array is empty
  on replay. Future enhancement (v0.2): persist `succeeded_ids JSONB`
  on `import_jobs` to enable full payload replay.
- **Crash-recovery sweep reuses Phase 4 pattern** (D5-11) — two phases
  using the same `time.Ticker` background-goroutine pattern. Planner
  should consider whether to extract a shared helper in
  `internal/scheduler/` (low priority for v0.1 with only 2 callers).

</specifics>

<deferred>
## Deferred Ideas

- **Async import pathway** (IMP-09) — bodies > 50MB or > 500 rows.
  v0.1 returns HTTP 413 pointing here. v0.2 likely uses Redis-backed
  job queue (`riverqueue/river` or similar).
- **Dry-run import mode** (IMP-10) — `?dry_run=true` returns
  `BulkImportResult` without persisting. Useful for admin sanity-check
  before committing. v0.2.
- **Error CSV download** (IMP-11) — `GET /v1/orgs/{org_id}/imports/{id}/errors.csv`
  returns failed rows as a CSV the admin can fix and re-import. v0.2.
- **`succeeded_ids` JSONB on `import_jobs`** — enables full
  idempotent-replay response (currently D5-13 has a known limitation
  where replays don't return original `succeeded[]` UUIDs). v0.2.
- **Skill REMOVAL via import** — D5-18 locked PATCH/merge semantics.
  Removing a skill via import would require a convention (e.g.,
  `-SKILL_VOICE` prefix or a `skills_to_remove` column). v0.2 can
  introduce if customer demand exists.
- **Multi-entity import in one POST** — currently `?entity=` is single.
  v0.2 could accept a JSON envelope `{agents: [...], skills: [...]}`.
- **Distributed lock for cleanup goroutine** — D5-11 sweep is
  single-replica safe (only one `time.Ticker` runs). When v1 deploys
  multi-replica, two pods running the sweep would race on the UPDATE
  (Postgres `UPDATE ... WHERE status='pending'` is idempotent so the
  race is benign, but the sweep should still elect a single leader to
  avoid duplicate slog warns and DB load). Pattern: PG advisory lock
  or Redis SETNX.
- **Reverse-proxy / ingress body limit alignment** — PITFALLS 5.3 calls
  out Nginx `client_max_body_size` / ALB read timeout. v0.1 has no
  proxy in docker-compose; v1 deploy doc should specify
  `client_max_body_size 55m; proxy_read_timeout 300s` for the import
  route.
- **OTel spans per row** — currently planner can decide whether to
  open a span per row (good for tracing slow rows) or per chunk
  (10× fewer spans, less detail). v0.1 likely chunk-level; v0.2 add
  per-row if admin observability needs it.
- **Per-org rate limiting** (e.g., 1 import per 5 minutes per org) —
  ErrorCode `rate_limited` is reserved in Phase 2 D-36 but not
  implemented. v0.2.
- **Schema_version v0.2 migration story** — `?schema_version=v0.1` is
  required for CSV. When v0.2 changes column names (PITFALLS 5.4),
  planner needs a compatibility-translation layer for at least one
  release cycle so saved admin templates don't break silently.
  STATE.md "Import schema version communication" blocker tracks this.

### Reviewed Todos (not folded)
None — no pending todos matched Phase 5 scope (`gsd-sdk todo.match-phase
5` returned 0 matches).

</deferred>

---

*Phase: 5-Bulk Import (Go)*
*Context gathered: 2026-05-17*
