# Phase 5: Bulk Import (Go) - Research

**Researched:** 2026-05-17 (re-research post Phase 04.1)
**Domain:** HTTP request streaming + CSV/JSON parsing + per-entity SQL upsert orchestration in Go (chi + pgx v5 + sqlc + oapi-codegen v2 strict-server) — keyed on the post-04.1 `(org_id, code)` identity contract
**Confidence:** HIGH (every claim that the planner needs to lock can be verified against the shipped Phase 1-4 + Phase 04.1 code in the working tree + official Go stdlib docs + pgx v5 docs)

> **Re-research provenance.** This file overwrites the pre-Phase-04.1 `05-RESEARCH.md`. Phase 04.1 promoted `code` from a per-entity convention to a universal canonical identifier with `UNIQUE (org_id, code)` on every primary catalog entity and authored `UpsertXByCode` + `GetXByCode` sqlc primitives for all 6 entities — Phase 5 now consumes those primitives instead of authoring its own `UpsertXByExternalId` queries. The three architecture-shaping discoveries from the prior research survive intact (oapi-codegen oneOf infeasibility, strict-server eager-decode, stale `stubs.go` path) and are restated below in §Architectural Findings with their original wording. Every FK-reference passage is reframed from `external_id` to `code`. The new findings introduced by re-research are flagged inline with `[POST-04.1]`.

## Summary

Phase 5 turns the two `not_implemented` strict-server stubs in `services/api/internal/catalog/notimpl.go:28,34` into a real synchronous bulk-import endpoint that accepts JSON or CSV bodies for any of the 6 catalog entities, performs `(org_id, code)` upserts with per-row partial-success accounting via Phase 04.1's `UpsertXByCode` queries, persists results in a new `import_jobs` table, and supports optional `Idempotency-Key` replay. `external_id` remains an optional integration-mapping field on each `Import*Request` (mutable on conflict per D04_1-07) but is **not** the upsert key.

The phase has a small surface of **truly new code**: a new `internal/imports/` package, one new migration (`000003_create_import_jobs`), three new sqlc query files (`import_jobs.sql` plus per-entity agent-skills merge), six new `Import*Request` schemas in `openapi.yaml`, a new chi middleware for body-size enforcement, and a crash-recovery goroutine that mirrors Phase 4 STATE-07's `clockwork`-backed `safetySweep` pattern. Everything else (orgDB validator, strict-server pipeline, request-id injection, cache invalidation, two-org isolation harness, UUIDv7 minting, slog/OTel attribution, `validateCodeFormat` helper, `mapPgError` constraint-name introspection) is inherited from Phases 1-4 + 04.1 without modification.

**Architecture-shaping discoveries the planner MUST plan around** (3 carried + 4 new):

**Carried from pre-04.1 research (still valid):**

1. **The strict-server eagerly decodes JSON.** `services/api/internal/api/server.gen.go` BulkImportCatalog wrapper (verified verbatim — quoted in §Architectural Findings) calls `json.NewDecoder(r.Body).Decode(&body)` on the FULL request body BEFORE the handler runs whenever `Content-Type` starts with `application/json`. This conflicts with D5-23's streaming intent. True streaming requires either (a) `http.MaxBytesReader` middleware that bounds memory regardless of eager decode (Phase 5's only viable v0.1 path), or (b) custom codegen to disable auto-decode (out of scope for v0.1). The 50 MB MaxBytesReader cap bounds live memory to ~250-300 MB worst case, acceptable for v0.1 catalog scale.

2. **oapi-codegen v2 does NOT generate AsX/FromX helpers for `oneOf` request bodies.** `BulkImportCatalogJSONBody = []interface{}` (verified at `services/api/internal/api/types.gen.go:1337`) [VERIFIED: oapi-codegen issue #1620, open since 2024]. The `oneOf: [6 schemas]` approach floated in D5-25 generates a `union json.RawMessage` field with NO accessor methods. Keep the request body schema as `application/json: { type: array, items: {} }` (current state at `openapi/openapi.yaml:3092-3097`) and have the handler dispatch by `?entity=` + re-marshal each `interface{}` element into the right typed struct.

3. **CONTEXT.md path-of-stubs is `services/api/internal/server/stubs.go` — that path is STALE.** Actual stubs live in `services/api/internal/catalog/notimpl.go:28,34` (verified). Phase 4 replaced the old stubs.go pattern with a composite `ApiHandlers` struct in `services/api/cmd/api/main.go:178-194` that embeds `*catalog.Handlers` and `*state.Server`. **Phase 5 MUST add a third embedded type** (e.g., `*imports.Server`) to the `ApiHandlers` composite and delete `catalog/notimpl.go` once the real implementations land.

**New findings from re-research:**

4. **[POST-04.1] Phase 04.1 already authored all 6 `UpsertXByCode` queries** — verified in `services/api/internal/db/queries/{agents,skills,queues,channels,adapters,break_reasons}.sql`. Phase 5 invokes them; it does NOT author parallel upsert SQL. The signatures cover every field on the corresponding `Create*Request`. Phase 04.1 RESEARCH §Pattern 4 shipped intact. The previous research's "Phase 3 GAP" pitfall is RESOLVED.

5. **[POST-04.1] `agent_skills` upsert primitive is MISSING.** Phase 04.1 did NOT author `UpsertAgentSkill` / `MergeAgentSkill` — verified via `grep -n "name: " services/api/internal/db/queries/agent_skills.sql` returning only `InsertAgentSkill`, `DeleteAgentSkills`, `ListSkillsForAgent`, `SkillsPresentInOrg`. Phase 3's `replaceAgentSkills` helper (`catalog/agent_skills.go:74`) uses DELETE-ALL + INSERT-N (PUT semantics) which contradicts D5-18 MERGE semantics. **Phase 5 MUST author one new sqlc query — `MergeAgentSkill :exec`** — with `INSERT ... ON CONFLICT (agent_id, skill_id) DO UPDATE SET proficiency = EXCLUDED.proficiency` (D5-19 import-wins).

6. **[POST-04.1] OpenAPI spec still describes upsert-by-`external_id`** at 3 locations: line 3067 `Upsert semantics: ... keyed by (org_id, external_id)`, line 3212 in the `Import` tag description, and the wire-shape of items in `requestBody` at line 3096 (`items: {}` — no `Import*Request` schemas yet). The 04.1-PHASE5-AMEND-CHECKLIST.md (in Phase 04.1 artifacts) lists exactly which CONTEXT.md blocks to amend on the Phase 5 branch BEFORE plans are generated. The openapi.yaml deltas are Phase 5's Wave 0 (BEFORE codegen). All 3 new error-code constants (`ErrorCodeDuplicateCode`, `ErrorCodeDuplicateExternalId`, `ErrorCodeImmutableField`) are present in `types.gen.go:87-89`.

7. **[POST-04.1] Migration sequence is now `000003`, not editable-in-place.** `000002_catalog_v0_1.up.sql` was amended in place during Phase 04.1 (D04_1-09); per D-61 it remains editable until v0.1 SHIPS. Phase 04.1 has not yet merged to main but its work is in this worktree. Adding `import_jobs` to `000002` mixes concerns; the cleaner path (recommended in §Open Questions Q4) is a new `000003_create_import_jobs.{up,down}.sql`. The Phase 04.1 Plan 06 SUMMARY (line ~20) confirms 04.1 ships the migration as `000002` and Phase 5 should not co-mingle.

**Primary recommendation:** Sequence Phase 5 in 6 waves: (W0) Spec edits per D04_1-25 amend + codegen + new migration 000003 + body-limit middleware skeleton + ApiHandlers composite scaffolding; (W1) `MergeAgentSkill` sqlc query + `import_jobs.sql` queries + body-limit middleware unit tests; (W2) Coercion + typed-parser pipeline (`coerce.go` + `parser_json.go` + `parser_csv.go`); (W3) Per-entity row processors (one file per entity) + `import_jobs` lifecycle; (W4) Handler + ApiHandlers composite re-wiring + idempotency replay; (W5) Crash-recovery goroutine + integration tests (entity × format matrix + isolation tests + golden Excel CSV). Total ≈ 9-11 plans, mirroring Phase 3's catalog plan count.

## User Constraints (from CONTEXT.md)

### Locked Decisions

> All `external_id`-FK references replaced with `code` per D04_1-25 amend (10 blocks applied to CONTEXT.md this session). Decisions D5-15..D5-26 now consistently reference `code` as the upsert/FK key.

**D5-01 (CSV cell type coercion pipeline).** Every CSV cell goes through a typed pipeline `trim → lower → split → parse` BEFORE validation. The pipeline is dispatched by the DB field type (resolved from sqlc-generated Go types — `bool`, `int`, `string`, `pgtype.Text`, etc.). One central dispatcher; per-type coercer; per-entity validator runs only on post-coercion typed values.

**D5-02 (Bool coercion is loose).** Accept `true|false|TRUE|FALSE|1|0|yes|no` (case-insensitive after `trim → lower`). Any other value → per-row failure with `reason: invalid_bool`.

**D5-03 (Empty cells = null/skip).** On create: spec default applies. On update: field left unchanged. On a required column: per-row failure with `reason: missing_required`.

**D5-04 (Multi-value cells delimited by `|`, fallback `;` then `,`).** Priority `|` > `;` > `,`. CSV field-level `,` unescaping happens first (RFC 4180).

**D5-05 (Strict batch-level header policy).** Missing required column → HTTP 400 `{error: invalid_body, reason: missing_columns, missing: [...]}`. Unknown column → HTTP 400 `{error: invalid_body, reason: unknown_columns, unknown: [...]}`. NO rows processed.

**D5-06 (Enums case-insensitive auto-lower).** `voice`/`Voice`/`VOICE` → `voice`. No-match → per-row `reason: invalid_enum`.

**D5-07 (Integers tolerate leading float).** Pipeline: `trim → strconv.ParseFloat → math.Trunc → int`. Excel's `7.0` truncates to `7`. `"seven"` → per-row `reason: invalid_int`. Truncation discarding a non-zero fractional part emits `slog.Debug("import: int_truncation", "row", N, "field", F, "raw", "7.5", "value", 7)`.

**D5-08 (UTF-8 only after BOM strip).** Validate with `utf8.Valid([]byte)`. Invalid → HTTP 400 `{error: invalid_body, reason: csv_not_utf8}`. NO Latin-1 fallback.

**D5-09 (Batched-savepoint transactions; chunk size = 50 rows).** 500-row max → ≤ 10 chunks. Each chunk = 1 tx + per-row savepoint. Failed rows do NOT abort chunk; chunk abort only on infrastructure error.

**D5-10 (`import_jobs` row at start, finalised at end).** INSERT `status='pending'` BEFORE chunk 1. UPDATE counters + `errors` JSONB + `status='completed'` after chunk 10. Job INSERT is its own short tx.

**D5-11 (Crash-recovery sweep goroutine; 24h TTL).** Reuse Phase 4 STATE-07 pattern. Tick interval: 1 hour. UPDATE pending rows older than 24h to `status='failed'` with synthetic error entry. Single-replica; v1 multi-replica needs distributed lock (STATE.md blocker).

**D5-12 (`errors` JSONB shape mirrors `BulkImportResult.failed[]`).** `[{"row": 3, "field": "email", "error": "import_failed", "reason": "invalid email format"}, ...]`. Zero transformation.

**D5-13 (`Idempotency-Key` header optional + persisted).** Column `import_jobs.idempotency_key TEXT NULL` with `UNIQUE (org_id, idempotency_key) WHERE idempotency_key IS NOT NULL`. Hit → return prior job's `BulkImportResult` from `errors` JSONB + counters. Known v0.1 limitation: `succeeded[]` UUID list NOT reconstructable from counters → replay returns `succeeded: []` + `idempotent_replay: true`.

**D5-14 (`?entity=` query is canonical; Content-Type selects parser).** Generated `BulkImportCatalogRequestObject` exposes `JSONBody *BulkImportCatalogJSONRequestBody` AND `Body io.Reader`. Handler dispatches: `if req.JSONBody != nil` → JSON path; else → CSV path on `req.Body`. Unsupported Content-Type → strict-server 415.

**D5-15 (Per-entity `Import*Request` schemas — NOT `Create*Request`).** 6 new schemas. FKs by `code` (the universal user-facing canonical identifier per Phase 04.1 / D04_1-01), NOT by `id` (server-minted UUIDv7). `external_id` remains supported on each `Import*Request` as optional integration-mapping metadata (D04_1-07).

**D5-16 (JSON agent imports support nested `skills[]`).** `ImportAgentRequest.skills: [{skill_code, proficiency}, ...]`. Lookup `(org_id, skill_code)` → skill UUID via Phase 04.1's `GetSkillByCode`. Unknown skill → per-row failure (whole agent row fails). `skill_code` values follow `^[a-z][a-z0-9_]{0,63}$` (D04_1-03); illegal token → `reason: invalid_code_format` (handler reuses `validateCodeFormat` from `services/api/internal/catalog/codecheck.go:33`).

**D5-17 (CSV agent imports support skills via `skills` column with `code:prof|code:prof`).** Single cell `"skill_voice:7|skill_chat:9|skill_email:5"` (lowercase per D04_1-03 regex). Pipeline: trim → split by D5-04 priority → split each token on `:` → validate. Illegal `code` token → `reason: invalid_code_format`.

**D5-18 (Skill MERGE semantics on agent update — PATCH-like, NOT PUT).** When an import row updates an existing agent (matched by `code` via `UpsertAgentByCode`), the `skills` array MERGES into existing `agent_skills`. Existing skills NOT in the payload are LEFT INTACT. Diverges from `UpdateAgentRequest` PUT semantics. **Trade-off:** no skill REMOVAL via import (UI or PATCH only). `code` is immutable post-create (D04_1-02); `external_id` may be added or rebound on update via `ImportAgentRequest.external_id` (optional field).

**D5-19 (Proficiency conflict → import value wins).** `ON CONFLICT (agent_id, skill_id) DO UPDATE SET proficiency = EXCLUDED.proficiency`.

**D5-20 (Other entity types have no N:M in v0.1 import scope).** Skills/queues/channels/adapters/break_reasons are flat in import.

**D5-21 (50 MB cap via Content-Length pre-flight + `http.MaxBytesReader`).** Order: (a) Content-Length > 50<<20 → 413 immediately; (b) wrap r.Body in MaxBytesReader BEFORE any parser sees it. Reusable middleware factory in `services/api/internal/middleware/bodylimit.go`.

**D5-22 (500-row cap enforced inline during streaming parse).** Exit at row 501 with 413. Streaming for both JSON token-mode + CSV reader-loop.

**D5-23 (JSON parsing is streaming, not eager).** `json.Decoder` token-mode. Per-row decode failure → record in `failed[]`, continue. Syntactic error → abort with 400 `malformed_json`. ⚠️ **See §Architectural Findings F1 — this decision is structurally in tension with the generated strict-server wrapper that eagerly decodes the JSON body BEFORE the handler runs. v0.1 accepts eager decode (memory-bounded by D5-21 MaxBytesReader); the planner must surface this for user confirmation.**

**D5-24 (Content-Type dispatch is header-only — strict).** No magic-byte sniffing, no `?format=` query. Missing/unrecognised → strict-server 415.

**D5-25 (6 new request schemas in openapi.yaml).** `ImportAgentRequest`, `ImportSkillRequest`, `ImportQueueRequest`, `ImportChannelRequest`, `ImportAdapterRequest`, `ImportBreakReasonRequest`. Each mirrors `Create*Request` with FK fields swapped to `*_code` (regex `^[a-z][a-z0-9_]{0,63}$`). `ImportAgentRequest` carries `skills: [{skill_code, proficiency}]`. Request body schema becomes `oneOf: [6 schemas]` OR per-entity discriminator OR `items: {}` + handler dispatch. ⚠️ **See §Architectural Findings F2 — `oneOf` is structurally infeasible. Recommendation: keep `items: {}` and dispatch by `?entity=`. The 6 schemas still land in `components.schemas` so clients have typed shapes.**

**D5-26 (`ImportJob.status` enum: pending|completed|failed).** Required. v0.1 returns `completed` or `failed`; `pending` only via crash window / sweep transition.

**D5-27 (Optional `Idempotency-Key` header parameter + `idempotent_replay` boolean on `BulkImportResult`).**

### Claude's Discretion

- Exact internal package layout: `internal/imports/` (recommended — `import` is a Go reserved word so the package must be named something else; `imports` is the natural plural, parallel to `internal/catalog/` and `internal/state/`). The original CONTEXT.md floated `import_pkg/` — keeping `imports/` is cleaner.
- Goroutine sweep tick interval (recommend 1h).
- `import_jobs` migration filename: planner picks **`000003_create_import_jobs.up.sql`** (NEW migration, NOT amending 000002). Rationale in §Open Questions Q4.
- Whether per-entity `Import*Request` lives inline vs in a separate `openapi/components/imports.yaml` (inline recommended — current spec is 3213 lines; 6 new schemas adds ~250 lines, crossing the 3000 threshold flagged by Phase 2. Inline is still fine for v0.1; spec-split is a v0.2 refactor).
- Test data file convention: `services/api/internal/imports/testdata/<entity>-<scenario>.{json,csv}` recommended (mirrors Go convention).

### Deferred Ideas (OUT OF SCOPE)

- Async import pathway (IMP-09) — 413 message points users to v0.2.
- Dry-run import mode (IMP-10).
- Error CSV download (IMP-11).
- `succeeded_ids JSONB` on `import_jobs` for full idempotent replay.
- Skill REMOVAL via import (negative syntax).
- Multi-entity import in one POST.
- Distributed lock for cleanup goroutine.
- Reverse-proxy ingress body limit alignment.
- OTel per-row spans.
- Per-org rate limiting.
- Schema_version v0.2 migration compatibility layer.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| IMP-01 | POST /catalog/import?entity={type} accepts JSON or CSV body for all 6 entities | §Architecture Patterns §1 (Handler dispatch) + §Code Examples §1 |
| IMP-02 | CSV parser normalises UTF-8 BOM + CRLF + LF + embedded quotes/commas/newlines | §Code Examples §3 (BOM strip + csv.Reader defaults) + §Pitfall 3 |
| IMP-03 | **[POST-04.1]** Upsert keyed by `(org_id, code)` via `UpsertXByCode` from Phase 04.1 — same input twice → no duplicates | §Standard Stack §sqlc + §Code Examples §4 (Phase 04.1 query inventory verified) |
| IMP-04 | Failed rows return structured errors with row + field + message | D5-12 + §Code Examples §6 (`BulkImportFailedRow` shape — already in types.gen.go) |
| IMP-05 | HTTP 207 partial success / 200 all success | §Architecture Patterns §1 (strict-server response objects already generated) |
| IMP-06 | `import_jobs` persisted; GET /imports/{id} returns it | §Standard Stack §migration + §Code Examples §7 (000003 migration + new sqlc queries) |
| IMP-07 | 50 MB / 500 row cap → 413 | §Code Examples §2 (MaxBytesReader middleware + inline row counter) + §Pitfall 7 |
| IMP-08 | CSV requires ?schema_version=v0.1; mismatch → 400 with supported versions | §Code Examples §5 (schema version validation) + §Pitfall 5.4 |

## Project Constraints (from CLAUDE.md)

No `./CLAUDE.md` exists in the repo root. The user's PC-global `~/.claude/CLAUDE.md` (visible via system reminder) carries two hard rules that apply to this phase:

| Directive | Applicability to Phase 5 |
|-----------|---------------------------|
| **GitHub assignee** — every `gh issue create`/`gh pr create` from this PC MUST include `--assignee mpt-luongld` | Applies when shipping the Phase 5 PR |
| **Cross-AI peer review** — invoke Codex + Gemini on the diff before declaring work done, for any non-trivial work unit (≥3 commits, ≥5 files) | Phase 5 touches ~25+ files; cross-AI review is REQUIRED at phase ship |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HTTP request body intake + size enforcement | API / chi middleware | — | Stop large bodies BEFORE they reach the handler; chi middleware is the canonical seam (mirror Phase 1 OrgContext + UUIDv7PathParams). |
| Streaming JSON / CSV parsing | API / handler | — | Per-row error recovery requires handler-level control. |
| Typed coercion pipeline (D5-01) | API / handler (pure functions) | — | Pure transform; testable in isolation; reusable across formats. |
| Per-row code-format validation (Layer 1 inherited) | API / handler | — | **[POST-04.1]** Reuses Phase 04.1's `validateCodeFormat` helper (`services/api/internal/catalog/codecheck.go:33`) — single source of truth for the regex; no duplication. Phase 5 calls it on every cell that holds a `code` value (entity's own code + nested `skill_code` in agent imports). |
| Per-entity row validation (Layer 2) | API / handler | — | Mirrors Phase 3's `validateProficiencyRange` pattern in `catalog/agent_skills.go:37`. |
| Per-entity SQL upsert | DB / pgx + sqlc | — | **[POST-04.1]** All 6 `UpsertXByCode` queries already exist in Phase 04.1; Phase 5 invokes them via `generated.New(savepointTx).UpsertXByCode(ctx, ...)`. SQLChecker validates `org_id = $N` on every query. |
| Skill `code` → UUID resolution | DB / pgx + sqlc | API / handler caches per-request | **[POST-04.1]** Phase 5 authors a new `ResolveSkillCodes :many` query (`SELECT id, code FROM skills WHERE org_id = $1 AND code = ANY($2::text[])`) using Phase 04.1's `code` column. Handler caches `map[code]UUID` per chunk to avoid N+1. |
| `agent_skills` merge on import | DB / pgx + sqlc | — | **[POST-04.1 NEW]** Phase 5 authors `MergeAgentSkill :exec` (`INSERT ... ON CONFLICT (agent_id, skill_id) DO UPDATE SET proficiency = EXCLUDED.proficiency`). Phase 04.1 did NOT include this. |
| `import_jobs` lifecycle | DB / pgx + sqlc | — | Single short tx for create; single short tx for finalise. |
| Crash-recovery sweep | API / cmd/api goroutine | DB / pgx | Mirror Phase 4 `state.Server.safetySweep` (`state/ttl.go:160`): time.Ticker in cmd/api, bypass-marked ctx (`db.WithBypass(ctx, "import_crash_sweep")`) for SQLChecker. |
| Idempotency-Key replay | API / handler | DB / pgx | Lookup `(org_id, key)` BEFORE any work; rehydrate `BulkImportResult` from persisted job. |
| Cache invalidation | Phase 3's `cache.Cache` | API / handler | **[POST-04.1]** Phase 5's per-row processor calls `cache.Del(ctx, cache.Key(orgID, entity, id))` POST-COMMIT (after the chunk's outerTx.Commit succeeds), mirroring `catalog/agents.go:202`. The cache key uses the same `or:{orgID}:{entity}:{id}` shape — invalidates any cached detail GET response. Phase 5 does NOT need a separate "import cache" path; the existing CAT-11 contract is reused. |
| Error mapping (23505 constraint introspection) | API / handler | — | **[POST-04.1]** Phase 5 invokes the existing `mapPgError` in `services/api/internal/catalog/errors.go:48` — it already distinguishes `duplicate_code` (`_org_id_code_key` suffix) from `duplicate_external_id` (`ix_*_org_external_id` prefix) per D04_1-21. Phase 5's row processor wraps the mapPgError verdict into per-row `failed[]` entries. |
| OTel + slog attribution | App / telemetry | — | All inherited from Phase 1 (D-29 TracingHandler). Sweep goroutine emits `event=import_crash_sweep` per D5-11. |

## Standard Stack

### Core (already in go.mod, no install required)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/csv` (stdlib) | go 1.25 | CSV parsing | Only stdlib option; RFC 4180-compliant; well-tested. [VERIFIED: pkg.go.dev/encoding/csv] |
| `encoding/json` (stdlib) | go 1.25 | JSON parsing (streaming via `json.Decoder` token-mode) | Stdlib; same Decoder.Token + Decoder.More + Decoder.Decode pattern. [VERIFIED: pkg.go.dev/encoding/json] |
| `net/http` (stdlib) | go 1.25 | `http.MaxBytesReader` + `*http.MaxBytesError` | Stdlib since Go 1.0; `*MaxBytesError` typed error since Go 1.19. [VERIFIED: pkg.go.dev/net/http#MaxBytesReader] |
| `bufio` (stdlib) | go 1.25 | Wrap r.Body for `Peek(3)` BOM detection | Stdlib; canonical pattern. [VERIFIED: pkg.go.dev/bufio#Reader.Peek] |
| `unicode/utf8` (stdlib) | go 1.25 | `utf8.Valid([]byte)` for D5-08 strict UTF-8 | Stdlib. [VERIFIED: pkg.go.dev/unicode/utf8] |
| `strconv` (stdlib) | go 1.25 | `ParseFloat` + `Atoi` + `ParseBool` for D5-02 / D5-07 coercion | Stdlib. [VERIFIED: pkg.go.dev/strconv] |
| `math` (stdlib) | go 1.25 | `math.Trunc` for D5-07 integer-from-float truncation | Stdlib. [VERIFIED: pkg.go.dev/math#Trunc] |
| `regexp` (stdlib) | go 1.25 | **[POST-04.1]** Code-format regex via Phase 04.1's `codeFormat` helper (`services/api/internal/catalog/codecheck.go:28`) | Stdlib RE2; bounded `{0,63}` quantifier → no ReDoS. Compiled once at package init. |
| `github.com/google/uuid` | v1.6.0 | `uuid.NewV7()` for `import_jobs.id` | Carry-forward from Phase 1 D-19. [VERIFIED: services/api/go.mod:11] |
| `github.com/jackc/pgx/v5` | v5.9.2 | DB driver; `tx.Begin()` returns child Tx backed by SAVEPOINT; typed error mapping (23505 with `pgErr.ConstraintName`) | Carry-forward from Phase 1. [VERIFIED: pkg.go.dev/github.com/jackc/pgx/v5] |
| `github.com/jonboulle/clockwork` | v0.4.0 | Injectable Clock for sweep goroutine tests | Carry-forward from Phase 4 STATE-07 (already promoted to direct in Phase 4). [VERIFIED: services/api/go.mod] |

### Supporting (already in go.mod)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sqlc` CLI (build tool) | v1.31.1 | Generate type-safe Go from SQL | New `import_jobs.sql` + `MergeAgentSkill` + `ResolveSkillCodes` |
| `oapi-codegen` (build tool) | v2.7.0 | Regen `Import*Request` types + `BulkImportCatalogParams` (with Idempotency-Key header) after spec edits | Wave 0 spec edit + `task gen` re-runs |
| `golang-migrate` (build tool) | v4.19.1 | Apply `000003_create_import_jobs.up.sql` | Wave 0 |
| `github.com/redis/go-redis/v9` | v9.x | Cache layer (Phase 3 `cache.Cache` reused; Phase 5 calls `cache.Del` post-commit per CAT-11) | Cache invalidation per-row post-commit |
| `github.com/stretchr/testify` | v1.11.1 (test-only) | Integration test assertions | Inherited |
| `github.com/testcontainers/testcontainers-go` + `modules/postgres` | v0.42.0 (test-only) | Real Postgres in tests | `testsupport.StartPostgres(t)` |
| `github.com/alicebob/miniredis/v2` | v2.38.0 (test-only) | In-process Redis for unit tests | Cache assertions; mirror Phase 3 cache_test.go |

### Supporting (NEW — to install in Phase 5)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| *(none)* | — | All required libraries are already in `services/api/go.mod`. | Phase 5 introduces NO new direct go.mod dependencies. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `encoding/csv` | `github.com/go-csv/go-csv` or `github.com/jszwec/csvutil` | Third-party adds BOM-stripping convenience but introduces a new dep and obscures Pitfall 3 visibility. Stick with stdlib. |
| stdlib `encoding/json` streaming Decoder | `github.com/goccy/go-json` or `github.com/bytedance/sonic` | Performance-only. At 500-row / 50MB scale the stdlib is more than fast enough. Stick with stdlib. |
| Per-row SAVEPOINT inside one big tx | `COPY ... FROM STDIN` (Postgres-native bulk insert) | COPY is faster but doesn't support per-row partial-success or ON CONFLICT logic. D5-09 chunked-savepoint is correct. |
| `pgx.Tx.Begin()` for savepoint (pgx canonical) | Raw `tx.Exec(ctx, "SAVEPOINT row_N")` + `RELEASE` / `ROLLBACK TO` | pgx's `tx.Begin()` on a Tx returns a pseudo-nested Tx via SAVEPOINT. [VERIFIED: pkg.go.dev/github.com/jackc/pgx/v5 — "The Tx returned from Conn.Begin also implements the Tx.Begin method. ... internally implemented with savepoints."] Use the typed API. |
| `oneOf` for request body schema | `items: {}` + handler dispatch on `?entity=` | **§Architectural Findings F2:** oapi-codegen v2.7.0 does NOT generate `AsX/FromX` for `oneOf` request bodies. Keep `items: {}`. |
| Authoring `UpsertXByExternalId` queries | **[POST-04.1]** Invoke Phase 04.1's `UpsertXByCode` queries | Phase 04.1 already authored these (verified — 6 queries land in `services/api/internal/db/queries/*.sql`). Phase 5 only authors NEW queries that Phase 04.1 did not produce: `MergeAgentSkill`, `ResolveSkillCodes`, `import_jobs.sql` (5 queries). |
| Authoring `UpsertAgentSkill` to replace skills wholesale | `MergeAgentSkill :exec` with ON CONFLICT DO UPDATE | D5-18 mandates MERGE (PATCH-like), NOT replace (PUT-like). Phase 3's `replaceAgentSkills` (DELETE-ALL + INSERT-N) is the wrong semantic for import. Author MERGE separately; do NOT call `replaceAgentSkills`. |

**Installation:**
```bash
# No new go.mod direct deps.
```

**Version verification:**
```bash
go list -m github.com/jackc/pgx/v5         # v5.9.2 — current
go list -m github.com/jonboulle/clockwork  # v0.4.0 — Phase 4 carry-forward (direct)
go list -m github.com/google/uuid          # v1.6.0 — current
```

## Package Legitimacy Audit

Phase 5 installs NO new external Go modules. Every library is either already in `services/api/go.mod` (verified in tree) or a Go standard-library package. No slopcheck run required for this phase.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| (none) | — | — | — | — | — | NO new packages |

**Packages removed due to slopcheck [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────┐
│                          HTTP Request                                │
│      POST /v1/orgs/{org_id}/catalog/import?entity={type}             │
│        Headers: X-Org-Id, Content-Type, Idempotency-Key?             │
│        Body: JSON array OR CSV (max 50 MB / 500 rows)                │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│ chi router (server.NewMux)                                           │
│  1. Recoverer (chi stdlib)                                           │
│  2. RequestID (UUIDv7) — appmw.RequestID                             │
│  3. orgContextMiddleware  → /v1/* requires X-Org-Id                  │
│  4. uuidv7PathParamsMiddleware → no-op (no {id} on /import)          │
│  5. NEW: bodyLimitMiddleware (50<<20) → wraps r.Body for THIS path   │
│       (a) Content-Length pre-flight → 413 if > 50MB                  │
│       (b) http.MaxBytesReader wrap                                   │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│ Generated strict-server wrapper (server.gen.go BulkImportCatalog)    │
│  • Reads ?entity= and ?schema_version= → BulkImportCatalogParams     │
│  • Reads Idempotency-Key header → Params.IdempotencyKey *uuid        │
│  • If Content-Type starts with "application/json":                   │
│       eager json.NewDecoder(r.Body).Decode(&body) → JSONBody         │
│       (memory-bounded by MaxBytesReader; OOM-safe per §F1)           │
│  • If Content-Type starts with "text/csv":                           │
│       request.Body = r.Body (still MaxBytes-wrapped, not consumed)   │
│  • Else: no JSONBody, no Body → handler returns 415                  │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│ imports.Server.BulkImportCatalog(ctx, req) (strict handler)          │
│   STEP 1: Validate ?entity= ∈ {6 valid types}; else 400.             │
│   STEP 2: If CSV path, validate ?schema_version=v0.1; else 400.      │
│   STEP 3: If Idempotency-Key present, lookup (org_id, key) →         │
│           hit → rehydrate BulkImportResult, set                      │
│                  idempotent_replay=true, return early                │
│           miss → proceed                                              │
│   STEP 4: Parse + validate header (CSV only) OR coerce array         │
│           (JSON). Header validation is BATCH-LEVEL strict.            │
│   STEP 5: Mint import_jobs.id (uuid.NewV7); INSERT job row           │
│           status='pending' in its OWN short tx.                       │
│   STEP 6: Read rows in streaming fashion:                            │
│           • Per-row count ≤ 500 (D5-22; exit at 501 with 413)        │
│           • Per-row coercion pipeline (D5-01 trim→lower→split→parse) │
│           • Per-row Layer-2 validation                                │
│           • For agent imports: per-row validateCodeFormat on agent's │
│             `code` AND on every nested skill_code                     │
│           • Buffer rows into chunks of 50 (D5-09)                    │
│   STEP 7: For each chunk:                                            │
│           • orgDB.BeginTx (real pgx Tx)                              │
│           • Resolve skill codes ONCE for the whole chunk via          │
│             ResolveSkillCodes(ctx, orgID, allCodes) → map[code]UUID  │
│           • For each row i: tx.Begin(ctx) (child Tx = SAVEPOINT)     │
│             ├─ UpsertXByCode (Phase 04.1 query) → returns id+row     │
│             ├─ If agent (new id): also InsertAgentState              │
│             │   with ON CONFLICT (agent_id) DO NOTHING                │
│             │   (Phase 4 Hazard 7 / catalog/agents.go:164 pattern)    │
│             ├─ If agent: MergeAgentSkill per skill (D5-18 PATCH-like)│
│             ├─ Success → savepointTx.Commit() (RELEASE)              │
│             ├─ Failure → savepointTx.Rollback() (ROLLBACK TO)        │
│             │     mapPgError → {duplicate_code | duplicate_external_id│
│             │                    | invalid_reference | invalid_value} │
│             │     append {row,field,error,reason} to failed[]        │
│           • outerTx.Commit() (whole chunk)                            │
│           • Per-entity cache.Del for each succeeded row (POST-COMMIT)│
│   STEP 8: UPDATE import_jobs row → status=completed/failed,          │
│           succeeded_rows, failed_rows, errors JSONB.                  │
│   STEP 9: Marshal BulkImportResult → 200/207/422                     │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│ PostgreSQL: catalog tables + import_jobs                             │
│  • UNIQUE (org_id, code) on each catalog entity (Phase 04.1)         │
│  • Partial UNIQUE (org_id, external_id) WHERE external_id IS NOT NULL│
│  • UNIQUE (org_id, idempotency_key) WHERE idempotency_key IS NOT NULL│
│  • ix_import_jobs_pending_updated for crash sweep                    │
└──────────────────────────────────────────────────────────────────────┘

       ┌──────────────────────────────────────────────────────────────┐
       │ Background goroutine (cmd/api/main.go)                       │
       │  • clockwork.NewRealClock().NewTicker(1h)                    │
       │  • Bypass-marked ctx (db.WithBypass(ctx, "import_crash_sweep"))│
       │  • UPDATE import_jobs SET status='failed' WHERE ...          │
       │    status='pending' AND updated_at < now() - 24h             │
       │  • Mirror state.Server.Start/Stop lifecycle (WaitGroup, ctx) │
       │  • Synchronous startup sweep before ticker (mirror D-95)     │
       └──────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
services/api/
├── cmd/api/
│   └── main.go                          # MODIFIED: add *imports.Server to ApiHandlers composite + Start/Stop calls
├── internal/
│   ├── api/                              # MODIFIED: regen after openapi.yaml edits (task gen)
│   ├── catalog/
│   │   ├── codecheck.go                  # REUSED from Phase 04.1 — validateCodeFormat + validateImmutableCode
│   │   ├── errors.go                     # REUSED from Phase 04.1 — mapPgError with constraint-name introspection
│   │   └── notimpl.go                    # DELETED: real handlers ship in internal/imports/
│   ├── imports/                          # NEW PACKAGE (per D5 / Claude's Discretion)
│   │   ├── doc.go                        # Package overview + D5-* invariant index
│   │   ├── server.go                     # Server struct + Deps + New + Start/Stop lifecycle (mirror state.Server)
│   │   ├── handler_import.go             # BulkImportCatalog method (the big one)
│   │   ├── handler_get_job.go            # GetImportJob method
│   │   ├── coerce.go                     # D5-01 typed pipeline: TrimToLower / ParseBool / ParseInt / SplitMulti
│   │   ├── parser_json.go                # Re-marshal-from-interface{} per-row loop (post §F1 strategy)
│   │   ├── parser_csv.go                 # BOM strip + utf8.Valid + csv.Reader loop + header validation
│   │   ├── header.go                     # Per-entity column registry: required + optional + typed dispatch
│   │   ├── row_agent.go                  # Agent row processor: validateCodeFormat + UpsertAgentByCode + InsertAgentState + MergeAgentSkill
│   │   ├── row_skill.go                  # Skill row processor: validateCodeFormat + UpsertSkillByCode
│   │   ├── row_queue.go                  # Queue row processor: UpsertQueueByCode
│   │   ├── row_channel.go                # Channel row processor (default_queue_code FK by code lookup + UpsertChannelByCode)
│   │   ├── row_adapter.go                # Adapter row processor (JSONB config passthrough; UpsertAdapterByCode)
│   │   ├── row_break_reason.go           # BreakReason row processor: UpsertBreakReasonByCode
│   │   ├── chunk.go                      # Chunked-savepoint orchestration: outerTx.Begin + sp.Begin per row
│   │   ├── jobs.go                       # import_jobs INSERT/UPDATE/Get/Replay
│   │   ├── idempotency.go                # Idempotency-Key lookup + rehydration
│   │   ├── sweep.go                      # Crash-recovery goroutine (mirror state.Server safetySweep / startupSweep)
│   │   ├── errors.go                     # Sentinel errors + reason mapping + per-row mapPgError wrapper
│   │   ├── testdata/
│   │   │   ├── agents-basic.json
│   │   │   ├── agents-basic.csv
│   │   │   ├── agents-with-skills.csv
│   │   │   ├── agents-with-skills.json
│   │   │   ├── windows-excel-agents.csv      # UTF-8 BOM + CRLF (Pitfall 3)
│   │   │   ├── break_reasons-basic.csv
│   │   │   ├── full-failure.csv              # 422 all-fail case
│   │   │   ├── partial-success.csv           # 207 case
│   │   │   ├── oversized-501-rows.csv        # 501 rows
│   │   │   ├── invalid-utf8.csv              # 400 csv_not_utf8 case
│   │   │   └── invalid-code-format.csv       # 400 invalid_code_format (lowercase regex violation)
│   │   ├── coerce_test.go                # Per-pipeline unit tests
│   │   ├── parser_csv_test.go            # BOM/CRLF/embedded-comma/quoted-newline coverage
│   │   ├── parser_json_test.go           # Streaming + recovery + malformed test cases
│   │   ├── header_test.go                # missing_columns / unknown_columns
│   │   ├── row_agent_test.go             # Per entity (including validateCodeFormat negative cases)
│   │   ├── chunk_test.go                 # SAVEPOINT roll-forward / roll-back behaviour
│   │   ├── jobs_test.go                  # import_jobs lifecycle
│   │   ├── idempotency_test.go           # Replay hit/miss
│   │   ├── sweep_test.go                 # Clockwork-backed crash sweep
│   │   ├── handlers_test.go              # End-to-end handler tests (one per entity × per format)
│   │   └── testutil_test.go              # Shared httptest server + seed helpers
│   ├── middleware/
│   │   ├── bodylimit.go                  # NEW: BodyLimit(maxBytes) chi MiddlewareFunc
│   │   └── bodylimit_test.go             # NEW: Content-Length pre-flight + mid-stream MaxBytesError test
│   ├── db/queries/
│   │   ├── agents.sql                    # UNCHANGED (Phase 04.1 already authored UpsertAgentByCode)
│   │   ├── skills.sql                    # MODIFIED: append ResolveSkillCodes :many (NEW; Phase 04.1 did not author)
│   │   ├── queues.sql                    # UNCHANGED
│   │   ├── channels.sql                  # MODIFIED: append ResolveChannelDefaultQueueCode (optional — see Open Q3)
│   │   ├── adapters.sql                  # UNCHANGED
│   │   ├── break_reasons.sql             # UNCHANGED
│   │   ├── agent_skills.sql              # MODIFIED: append MergeAgentSkill :exec (NEW; Phase 04.1 did not author)
│   │   └── import_jobs.sql               # NEW FILE: InsertImportJob, FinaliseImportJob, GetImportJob, LookupByIdempotencyKey, SweepCrashedImportJobs
│   ├── db/sqlcheck.go                    # MODIFIED: add "import_jobs": {} to tenantTables (mirror Phase 4 agent_states:42)
│   └── test/isolation/
│       └── imports_test.go               # NEW: cross-org probes (orgA POST → orgB GET → 404; same-code-across-orgs both succeed)
├── openapi/openapi.yaml                  # MODIFIED:
│                                         #   • Add 6 Import*Request schemas (Block 1 from PHASE5-AMEND-CHECKLIST)
│                                         #   • Update line 3067 description: external_id → code upsert key
│                                         #   • Update line 3212 Import tag description
│                                         #   • Update ImportJob.status enum (D5-26)
│                                         #   • Add Idempotency-Key header parameter (D5-27)
│                                         #   • Add idempotent_replay field on BulkImportResult (D5-27)
└── migrations/
    └── 000003_create_import_jobs.up.sql  # NEW
    └── 000003_create_import_jobs.down.sql # NEW
```

### Pattern 1: Strict-server handler dispatch on `?entity=` + Content-Type (D5-14, D5-24)

**What:** The handler receives a `BulkImportCatalogRequestObject` carrying `Params.Entity`, optional `Params.SchemaVersion`, optional `Params.IdempotencyKey`, `JSONBody` (eager-decoded `[]interface{}` for `application/json`), and `Body` (`io.Reader` for `text/csv`). Handler picks parser, picks per-entity row processor, returns one of 6 `BulkImportCatalog{200,207,400,413,422,500}JSONResponse` types — all already generated in `server.gen.go`.

**When to use:** Single entry point for IMP-01.

**Example:**
```go
// Source: services/api/internal/catalog/agents.go:71 (Phase 3 + 04.1 pattern,
//         adapted for streaming + multiple entity types)
// Source: services/api/internal/api/server.gen.go (BulkImportCatalogRequestObject)
func (s *Server) BulkImportCatalog(ctx context.Context, req api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
        }}, nil
    }

    // STEP 1: Validate ?entity= (already a typed enum from oapi-codegen)
    if !req.Params.Entity.Valid() {
        return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
            Error: api.ErrorCodeInvalidBody, Reason: "unsupported_entity",
        }), nil
    }

    // STEP 2: Idempotency-Key replay lookup BEFORE any work.
    if req.Params.IdempotencyKey != nil {
        if replay, found, err := s.lookupIdempotentReplay(ctx, orgID, *req.Params.IdempotencyKey); err != nil {
            return api.BulkImportCatalog500JSONResponse{...}, nil
        } else if found {
            return replay, nil
        }
    }

    // STEP 3: Dispatch parser by which body field is populated.
    switch {
    case req.JSONBody != nil:
        return s.importJSON(ctx, orgID, req.Params.Entity, *req.JSONBody, req)
    case req.Body != nil:
        if req.Params.SchemaVersion == nil || *req.Params.SchemaVersion != "v0.1" {
            return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
                Error: api.ErrorCodeInvalidBody,
                Reason: "unsupported_schema_version_supported_versions=v0.1",
            }), nil
        }
        return s.importCSV(ctx, orgID, req.Params.Entity, req.Body, req)
    default:
        return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
            Error: api.ErrorCodeInvalidBody, Reason: "unsupported_content_type",
        }), nil
    }
}
```

### Pattern 2: Body-size enforcement middleware (D5-21)

**What:** Chi MiddlewareFunc that (a) pre-flights `Content-Length` → 413 if > 50 MB; (b) wraps `r.Body` in `http.MaxBytesReader` so mid-stream over-limit also fails. Path-scoped to the import endpoint.

**Example:**
```go
// services/api/internal/middleware/bodylimit.go (NEW)
// Source: pkg.go.dev/net/http#MaxBytesReader [CITED]
package middleware

import (
    "errors"
    "net/http"
    "strconv"
    "strings"
)

// BodyLimit returns a chi MiddlewareFunc that rejects requests whose
// declared Content-Length exceeds maxBytes and wraps r.Body in
// http.MaxBytesReader so mid-stream over-limit also surfaces as
// *http.MaxBytesError (D5-21). pathPrefix scopes the middleware to a
// single URL so other endpoints retain unlimited bodies.
func BodyLimit(maxBytes int64, pathPrefix string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !strings.HasPrefix(r.URL.Path, pathPrefix) {
                next.ServeHTTP(w, r)
                return
            }
            // (a) Content-Length pre-flight (when present and trusted)
            if cl := r.Header.Get("Content-Length"); cl != "" {
                if n, err := strconv.ParseInt(cl, 10, 64); err == nil && n > maxBytes {
                    WriteError(r.Context(), w, http.StatusRequestEntityTooLarge,
                        "invalid_body", "request_too_large_use_async_pathway")
                    return
                }
            }
            // (b) Wrap r.Body so mid-stream over-limit (chunked transfer,
            //     lying Content-Length) also fails. Handler downstream
            //     gets *http.MaxBytesError on Read and must map to 413.
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            next.ServeHTTP(w, r)
        })
    }
}

// In handler code, when parsing fails with *http.MaxBytesError → 413.
//   var maxBytesErr *http.MaxBytesError
//   if errors.As(decodeErr, &maxBytesErr) { return 413 }
```

### Pattern 3: BOM strip + UTF-8 validation + csv.Reader streaming (IMP-02, D5-08)

**What:** Read first 3 bytes via `bufio.Reader.Peek`. If they are `{0xEF, 0xBB, 0xBF}` → consume them. Validate remainder is UTF-8 by buffering (already bounded by MaxBytesReader) and calling `utf8.Valid(buf)`. Then hand the BOM-stripped reader to `csv.Reader`. csv.Reader auto-converts CRLF → LF [VERIFIED: pkg.go.dev/encoding/csv].

**Example:**
```go
// services/api/internal/imports/parser_csv.go (NEW)
package imports

import (
    "bufio"
    "encoding/csv"
    "errors"
    "io"
    "unicode/utf8"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripBOM peeks 3 bytes and consumes them iff they match the BOM.
// Peek is non-destructive when no BOM is present [CITED: pkg.go.dev/bufio#Reader.Peek].
func stripBOM(r io.Reader) (*bufio.Reader, error) {
    br := bufio.NewReader(r)
    head, err := br.Peek(3)
    if err != nil && !errors.Is(err, io.EOF) {
        return nil, err
    }
    if len(head) == 3 && head[0] == utf8BOM[0] && head[1] == utf8BOM[1] && head[2] == utf8BOM[2] {
        _, _ = br.Discard(3)
    }
    return br, nil
}

// newCSVReader returns a csv.Reader configured for D5-* compliance:
//   • LazyQuotes=false  (strict RFC 4180)
//   • FieldsPerRecord=0 (latches to first row's column count)
//   • TrimLeadingSpace=false  (coercion pipeline owns trimming)
// csv.Reader auto-converts CRLF → LF on output [CITED: pkg.go.dev/encoding/csv].
func newCSVReader(r io.Reader) *csv.Reader {
    cr := csv.NewReader(r)
    cr.LazyQuotes = false
    cr.FieldsPerRecord = 0
    cr.TrimLeadingSpace = false
    return cr
}
```

### Pattern 4: Re-marshal JSON to typed `Import*Request` (D5-23, §F1 accommodation)

**What:** Per §Architectural Findings F1, the strict-server has already eagerly decoded the JSON body into `[]interface{}` by the time the handler runs. The handler iterates the slice and re-marshals each element into the right typed `Import*Request` struct.

**Example:**
```go
// services/api/internal/imports/parser_json.go (NEW)
package imports

import (
    "bytes"
    "encoding/json"
    "fmt"
)

// reMarshalAs takes one element from req.JSONBody (interface{}) and
// turns it into a typed struct via the marshal→unmarshal round-trip.
// Per-row decode errors are isolated.
//
// Used as: reMarshalAs[api.ImportAgentRequest](rawItem) — once the new
// Import*Request schemas land in types.gen.go via Wave 0.
func reMarshalAs[T any](item interface{}) (T, error) {
    var zero T
    raw, err := json.Marshal(item)
    if err != nil {
        return zero, fmt.Errorf("re-marshal: %w", err)
    }
    var out T
    if err := json.Unmarshal(raw, &out); err != nil {
        return zero, fmt.Errorf("re-unmarshal: %w", err)
    }
    return out, nil
}
```

```go
// In handler_import.go:
func (s *Server) importJSON(ctx context.Context, orgID uuid.UUID, entity api.ImportEntityType, body api.BulkImportCatalogJSONRequestBody, req api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
    if len(body) > 500 {  // D5-22 hard cap
        return api.BulkImportCatalog413JSONResponse{...}, nil
    }
    return s.processRows(ctx, orgID, entity, len(body), func(idx int) (any, error) {
        return body[idx], nil
    })
}
```

### Pattern 5: Chunked-savepoint orchestration (D5-09)

**What:** For each chunk of ≤50 rows: open one outer `OrgTx`. Resolve all skill codes for the chunk in one batch call. For each row: call `outerTx.Begin(ctx)` which pgx v5 implements as SAVEPOINT [VERIFIED: pkg.go.dev/github.com/jackc/pgx/v5 — "The Tx returned from Conn.Begin also implements the Tx.Begin method. ... internally implemented with savepoints."]. Try the per-row `UpsertXByCode`; on success `savepointTx.Commit()` (= RELEASE); on failure `savepointTx.Rollback()` (= ROLLBACK TO).

**Critical pgx gotcha:** `OrgTx` (Phase 1 wrapper at `services/api/internal/db/orgdb.go:172`) does NOT expose a `Begin()` method that returns a child `OrgTx`. Phase 5 must extend it:

```go
// services/api/internal/db/orgdb.go — APPEND
// BeginSavepoint starts a savepoint on the current tx and returns a new
// *OrgTx wrapping the child. SQLChecker preflight is preserved.
func (t *OrgTx) BeginSavepoint(ctx context.Context) (*OrgTx, error) {
    child, err := t.tx.Begin(ctx)
    if err != nil {
        return nil, fmt.Errorf("orgtx: begin savepoint: %w", err)
    }
    return &OrgTx{tx: child, checker: t.checker, mode: t.mode}, nil
}
```

**Example chunk loop:**
```go
// services/api/internal/imports/chunk.go (NEW)
func (s *Server) processChunk(ctx context.Context, orgID uuid.UUID, rowProc rowProcessor, rows []parsedRow) (succeeded []succeededRow, failed []api.BulkImportFailedRow) {
    outerTx, err := s.deps.OrgDB.BeginTx(ctx)
    if err != nil {
        for _, r := range rows {
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "tx_begin_failed",
            })
        }
        return
    }
    defer func() { _ = outerTx.Rollback(ctx) }()  // safe after commit; pgx ignores

    // Resolve skill codes ONCE for the chunk (D5-16, D5-17 batch optimization).
    // Mirrors the SkillsPresentInOrg pattern in catalog/agent_skills.go:88.
    skillResolver, err := s.resolveChunkSkillCodes(ctx, outerTx, orgID, rows)
    if err != nil {
        for _, r := range rows {
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "skill_resolve_failed",
            })
        }
        return
    }

    for _, r := range rows {
        sp, spErr := outerTx.BeginSavepoint(ctx)
        if spErr != nil {
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "savepoint_begin_failed",
            })
            continue
        }
        out, procErr := rowProc.process(ctx, sp, orgID, r, skillResolver)
        if procErr != nil {
            _ = sp.Rollback(ctx)  // ROLLBACK TO savepoint
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Field: procErr.Field, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: procErr.Reason,
            })
            continue
        }
        if err := sp.Commit(ctx); err != nil {  // RELEASE savepoint
            failed = append(failed, ...)
            continue
        }
        succeeded = append(succeeded, out)
    }

    if err := outerTx.Commit(ctx); err != nil {
        // Chunk commit failure — all "succeeded" rows in this chunk are lost.
        for _, sr := range succeeded {
            failed = append(failed, api.BulkImportFailedRow{
                Row: sr.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "chunk_commit_failed",
            })
        }
        return nil, failed
    }

    // POST-COMMIT cache invalidation. ALWAYS after outerTx.Commit succeeds.
    // Mirrors catalog/agents.go:202 pattern.
    for _, sr := range succeeded {
        cacheKey := cache.Key(orgID, string(sr.entity), sr.id)
        if delErr := s.deps.Cache.Del(ctx, cacheKey); delErr != nil {
            s.deps.Logger.WarnContext(ctx, "import.cache_del_failed",
                "key", cacheKey, "err", delErr)
        }
    }
    return succeeded, failed
}
```

### Pattern 6: Per-entity upsert via Phase 04.1's `UpsertXByCode` (IMP-03) — POST-04.1

**What:** Phase 04.1 already shipped all 6 `UpsertXByCode` queries. Phase 5 invokes them inside the savepoint Tx.

**Example agent row processor:**
```go
// services/api/internal/imports/row_agent.go (NEW)
//
// IMP-03 (post-04.1): per-row upsert keyed by (org_id, code) via
// Phase 04.1's UpsertAgentByCode query (services/api/internal/db/queries/agents.sql:127).
// New row → INSERT with defaults; existing row → UPDATE all import-mentioned
// columns + bump version. The ON CONFLICT path leaves `code` untouched
// (it IS the conflict target). `external_id` can be (re)bound on conflict.
type agentRowProc struct {
    handlers *Server
}

func (p *agentRowProc) process(ctx context.Context, sp *db.OrgTx, orgID uuid.UUID, r parsedRow, skillResolver *chunkSkillResolver) (succeededRow, *rowError) {
    typed, err := reMarshalAs[api.ImportAgentRequest](r.raw)
    if err != nil {
        return succeededRow{}, &rowError{Field: "", Reason: "invalid_json_row"}
    }

    // Layer 1: code-format validation (reuse Phase 04.1 helper).
    if !validateCodeFormat(typed.Code) {
        return succeededRow{}, &rowError{Field: "code", Reason: "invalid_code_format"}
    }

    // Layer 1 for nested skill_code: validate each.
    if typed.Skills != nil {
        for i, sk := range *typed.Skills {
            if !validateCodeFormat(sk.SkillCode) {
                return succeededRow{}, &rowError{
                    Field:  fmt.Sprintf("skills[%d].skill_code", i),
                    Reason: "invalid_code_format",
                }
            }
        }
    }

    // Resolve skill UUIDs (from pre-computed chunk-level map).
    skillUUIDs, err := skillResolver.resolveAll(getSkillCodes(typed))
    if err != nil {
        // err.Codes is the set of unknown codes.
        return succeededRow{}, &rowError{
            Field:  fmt.Sprintf("skills[%d].skill_code", err.FirstIndex),
            Reason: "unknown_skill",
        }
    }

    qtx := generated.New(sp)
    id := uuid.Must(uuid.NewV7())

    row, upErr := qtx.UpsertAgentByCode(ctx, generated.UpsertAgentByCodeParams{
        ID:         pgUUID(id),
        OrgID:      pgUUID(orgID),
        Code:       typed.Code,
        ExternalID: ptrToPgText(typed.ExternalId),  // optional, mutable
        Name:       typed.Name,
        Email:      string(typed.Email),
        Enabled:    derefOr(typed.Enabled, true),
    })
    if upErr != nil {
        status, code, reason := mapPgError(upErr, "agent")
        _ = status  // Phase 5 wraps in row-level failed[], not HTTP status
        return succeededRow{}, &rowError{
            Field:  "",
            Reason: fmt.Sprintf("%s:%s", code, reason),
        }
    }

    // Seed agent_states for NEW agents only. ON CONFLICT (agent_id) DO NOTHING
    // so re-imports don't reset the state machine to Offline.
    // Phase 4 Hazard 7 / catalog/agents.go:164 pattern, adapted with ON CONFLICT.
    // Phase 5 must also author this `InsertAgentStateOnConflictNothing` query
    // — Phase 4's InsertAgentState has no ON CONFLICT clause.
    if _, stErr := qtx.InsertAgentStateOnConflictNothing(ctx, generated.InsertAgentStateOnConflictNothingParams{
        AgentID: row.ID,
        OrgID:   pgUUID(orgID),
        Status:  string(api.AgentStatusOffline),
    }); stErr != nil {
        return succeededRow{}, &rowError{Field: "", Reason: "agent_state_seed_failed"}
    }

    // Merge agent_skills (D5-18 PATCH-like).
    if typed.Skills != nil {
        for _, sk := range *typed.Skills {
            uuid_ := skillUUIDs[sk.SkillCode]
            if _, msErr := qtx.MergeAgentSkill(ctx, generated.MergeAgentSkillParams{
                AgentID:     row.ID,
                SkillID:     pgUUID(uuid_),
                OrgID:       pgUUID(orgID),
                Proficiency: int32(sk.Proficiency),
            }); msErr != nil {
                return succeededRow{}, &rowError{
                    Field:  fmt.Sprintf("skills[%d]", -1),
                    Reason: "merge_skill_failed",
                }
            }
        }
    }
    return succeededRow{
        id:     uuid.UUID(row.ID.Bytes),
        entity: api.Agents,
        lineNo: r.lineNo,
    }, nil
}
```

### Pattern 7: New sqlc queries Phase 5 must author (POST-04.1 deltas)

**What:** Phase 04.1 authored all 6 `UpsertXByCode` queries; Phase 5 authors only the queries Phase 04.1 did NOT produce.

```sql
-- services/api/internal/db/queries/skills.sql — APPEND
--
-- name: ResolveSkillCodes :many
-- D5-16 + D5-17: batch lookup of skill UUIDs by `code` (post-04.1).
-- Used by the agent row processor to resolve nested skill references
-- once per chunk. Returns id + code pairs. Missing codes are absent
-- from the result; handler distinguishes "unknown skill" by set difference.
SELECT id, code
FROM skills
WHERE org_id = $1 AND code = ANY($2::text[]);
```

```sql
-- services/api/internal/db/queries/agent_skills.sql — APPEND
--
-- name: MergeAgentSkill :exec
-- D5-18 + D5-19: skill assignment MERGE on import (post-04.1).
-- ON CONFLICT (agent_id, skill_id) DO UPDATE so existing assignments get
-- the new proficiency from the import (D5-19 import wins). Existing skills
-- NOT in this payload are LEFT INTACT (D5-18 PATCH-like) — achieved by
-- simply NOT issuing DELETE statements.
INSERT INTO agent_skills (agent_id, skill_id, org_id, proficiency)
VALUES ($1, $2, $3, $4)
ON CONFLICT (agent_id, skill_id) DO UPDATE
SET proficiency = EXCLUDED.proficiency;
```

```sql
-- services/api/internal/db/queries/agent_states.sql — APPEND
--
-- name: InsertAgentStateOnConflictNothing :exec
-- Phase 5 import: seed state for NEW agents only; re-imports MUST NOT
-- regress the state machine to Offline. Differs from InsertAgentState
-- (Phase 4, line 1) which is unconditional.
INSERT INTO agent_states (agent_id, org_id, status, state_version)
VALUES ($1, $2, $3, 1)
ON CONFLICT (agent_id) DO NOTHING;
```

```sql
-- services/api/internal/db/queries/import_jobs.sql — NEW FILE
--
-- name: InsertImportJob :one
INSERT INTO import_jobs (
    id, org_id, entity_type, status, total_rows,
    succeeded_rows, failed_rows, errors, idempotency_key
)
VALUES ($1, $2, $3, 'pending', $4, 0, 0, NULL, sqlc.narg('idempotency_key'))
RETURNING id, org_id, entity_type, status, total_rows, succeeded_rows,
          failed_rows, errors, idempotency_key, created_at, updated_at;

-- name: FinaliseImportJob :one
UPDATE import_jobs
SET status = $1,
    succeeded_rows = $2,
    failed_rows = $3,
    errors = $4,
    updated_at = NOW()
WHERE id = $5 AND org_id = $6
RETURNING id, org_id, entity_type, status, total_rows, succeeded_rows,
          failed_rows, errors, idempotency_key, created_at, updated_at;

-- name: GetImportJob :one
SELECT id, org_id, entity_type, status, total_rows, succeeded_rows,
       failed_rows, errors, idempotency_key, created_at, updated_at
FROM import_jobs
WHERE id = $1 AND org_id = $2;

-- name: LookupImportJobByIdempotencyKey :one
SELECT id, org_id, entity_type, status, total_rows, succeeded_rows,
       failed_rows, errors, idempotency_key, created_at, updated_at
FROM import_jobs
WHERE org_id = $1 AND idempotency_key = $2;

-- name: SweepCrashedImportJobs :many
-- D5-11 crash-recovery sweep: flip pending → failed for jobs > 24h old.
-- Phase 5 sweep goroutine wraps ctx with db.WithBypass(ctx, "import_crash_sweep")
-- because this UPDATE is org-agnostic (mirrors state/ttl.go:179
-- "wrapup_sweeper.safety" pattern).
UPDATE import_jobs
SET status = 'failed',
    errors = '[{"row":0,"error":"import_failed","reason":"server_crash"}]'::jsonb,
    updated_at = NOW()
WHERE status = 'pending'
  AND updated_at < NOW() - INTERVAL '24 hours'
RETURNING id, org_id;
```

### Pattern 8: `import_jobs` migration (IMP-06)

**What:** New `000003_create_import_jobs.up.sql` (and `.down.sql`). Table has org_id (for SQLChecker), entity_type CHECK, status CHECK, counters, errors JSONB, idempotency_key NULL with partial unique index, timestamps.

```sql
-- migrations/000003_create_import_jobs.up.sql (NEW)
BEGIN;

CREATE TABLE import_jobs (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL,
    entity_type     TEXT NOT NULL
                      CHECK (entity_type IN ('agents','skills','queues','channels','adapters','break_reasons')),
    status          TEXT NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','completed','failed')),
    total_rows      INTEGER NOT NULL DEFAULT 0,
    succeeded_rows  INTEGER NOT NULL DEFAULT 0,
    failed_rows     INTEGER NOT NULL DEFAULT 0,
    errors          JSONB,
    idempotency_key TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_import_jobs_org_created ON import_jobs (org_id, created_at DESC, id DESC);

-- D5-11 crash-recovery: partial index on status='pending' so the
-- sweep query scans a tiny subset.
CREATE INDEX ix_import_jobs_pending_updated
    ON import_jobs (updated_at)
    WHERE status = 'pending';

-- D5-13 idempotency: partial UNIQUE index so the key only enforces
-- uniqueness when present.
CREATE UNIQUE INDEX uq_import_jobs_org_idempotency
    ON import_jobs (org_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

COMMIT;
```

### Pattern 9: Crash-recovery goroutine (D5-11, mirroring Phase 4 STATE-07)

**What:** A `time.Ticker`-based goroutine that periodically flips `status='pending'` rows older than 24h to `status='failed'`. Direct port of Phase 4's `state.Server.safetySweep` in `services/api/internal/state/ttl.go:160`. Phase 5's `imports.Server` has the same Start/Stop lifecycle and runs alongside `state.Server` in `cmd/api/main.go`.

**Critical reuse points from Phase 4:**

- `clockwork.Clock` injection via `WithClock` option (`state/handlers.go` pattern)
- `context.WithCancel` for graceful shutdown
- `sync.WaitGroup` for in-flight ticks
- `db.WithBypass(ctx, "import_crash_sweep")` so SQLChecker accepts the org-agnostic query (mirror `state/ttl.go:179` — `"wrapup_sweeper.safety"`)
- `slog.Warn` per transition with `event=import_crash_sweep` (D5-11)
- Single-replica only; STATE.md blocker flagged
- **Synchronous startup sweep BEFORE spawning the ticker** (mirror D-95 from `state/ttl.go:132 startupSweep`). Reason: a crash during import leaves a pending row that should flip to failed IMMEDIATELY on next process boot, not 1 hour later.

```go
// services/api/internal/imports/sweep.go (NEW — mirror state/ttl.go:160)
package imports

import (
    "context"
    "time"
    // ...
)

func (s *Server) safetySweep() {
    defer s.wg.Done()
    ticker := s.clock.NewTicker(s.sweepInterval)
    defer ticker.Stop()
    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.Chan():
            s.runSweepPastDue()
        }
    }
}

func (s *Server) runSweepPastDue() {
    ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
    defer cancel()
    ctx = db.WithBypass(ctx, "import_crash_sweep")

    q := generated.New(s.deps.OrgDB)
    rows, err := q.SweepCrashedImportJobs(ctx)
    if err != nil {
        s.deps.Logger.WarnContext(ctx, "import.crash_sweep.failed", "err", err)
        return
    }
    for _, r := range rows {
        s.deps.Logger.WarnContext(ctx, "import.crash_sweep",
            "event", "import_crash_sweep",
            "import_job_id", uuid.UUID(r.ID.Bytes),
            "org_id", uuid.UUID(r.OrgID.Bytes),
        )
    }
}
```

### Pattern 10: Idempotency-Key extraction + replay (D5-13, D5-27)

**What:** Add `Idempotency-Key` as a header parameter in `openapi.yaml` (D5-27) → oapi-codegen generates a typed field in `BulkImportCatalogParams`. Replay returns the prior `BulkImportResult` rehydrated from persisted `errors` JSONB + counters.

```yaml
# openapi.yaml additions (Wave 0)
components:
  parameters:
    IdempotencyKeyHeader:
      name: Idempotency-Key
      in: header
      required: false
      schema:
        type: string
        format: uuid
        description: Client-generated UUIDv7 for retry-safe POST.
```

```go
// In handler:
if req.Params.IdempotencyKey != nil {
    q := generated.New(s.deps.OrgDB)
    prior, err := q.LookupImportJobByIdempotencyKey(ctx, generated.LookupImportJobByIdempotencyKeyParams{
        OrgID:          pgUUID(orgID),
        IdempotencyKey: req.Params.IdempotencyKey.String(),
    })
    if err == nil {
        return rehydrateBulkImportResult(prior, true), nil
    }
    if !errors.Is(err, pgx.ErrNoRows) {
        return api.BulkImportCatalog500JSONResponse{...}, nil
    }
}

// D5-13 KNOWN LIMITATION: succeeded[] cannot be reconstructed from counters.
func rehydrateBulkImportResult(row generated.ImportJob, replay bool) api.BulkImportCatalog200JSONResponse {
    var failed []api.BulkImportFailedRow
    if row.Errors.Valid {
        _ = json.Unmarshal(row.Errors.Bytes, &failed)
    }
    return api.BulkImportCatalog200JSONResponse(api.BulkImportResult{
        Succeeded:        []api.UUIDv7{},
        Failed:           failed,
        IdempotentReplay: &replay,
    })
}
```

### Anti-Patterns to Avoid

- **DO NOT call `generated.New(s.deps.OrgDB)` inside the chunk loop** — every per-row query MUST go through the savepoint Tx (`generated.New(savepointTx)`). Otherwise the UPSERT commits outside the chunk tx and rollback-to-savepoint is broken. Mirror `catalog/agents.go:124` (`qtx := generated.New(tx)`).
- **DO NOT issue per-row skill code lookups** — that's N+1. Issue ONE `ResolveSkillCodes(ctx, orgID, allSkillCodes)` query per chunk at the start, build a `map[string]uuid.UUID`, and re-use across all 50 rows.
- **DO NOT use COPY ... FROM STDIN** for bulk insert. COPY doesn't support `ON CONFLICT` or per-row partial-success.
- **DO NOT auto-detect charset** (e.g., chardet or similar). D5-08 locks UTF-8 only; Pitfall 5.1 explicitly flags charset auto-detect as the #1 silent-corruption source.
- **DO NOT silently drop unknown CSV columns.** D5-05 strict policy: unknown column → 400 batch-level, NO rows processed. Pitfall 5.4.
- **DO NOT echo raw row content in `BulkImportFailedRow.reason`** — the Phase 2 OQ-2A resolution forbids this (deferred to v0.2 IMP-11).
- **DO NOT call `cache.Del` per-row inside the chunk loop.** Cache Del MUST run AFTER the chunk's outer `tx.Commit()` succeeds. Mirror Phase 3's post-commit pattern (`catalog/agents.go:202`).
- **DO NOT mint UUIDv7 for entity rows ahead of `UpsertXByCode`** — wait, actually `import_jobs.id` is minted via `uuid.NewV7()`. Entity row UUIDs are passed as `$1=id` to UpsertXByCode; on conflict the supplied id is ignored and the existing row's id is returned in RETURNING. Pattern matches `catalog/agents.go:111` (`id := uuid.Must(uuid.NewV7())`).
- **DO NOT skip `agent_states` INSERT for new agents.** Phase 4 `04-PATTERNS.md` Hazard 7 / Phase 5 Inheritance Reminder — every new agent must seed an `agent_states` row in the same SAVEPOINT tx, with `ON CONFLICT (agent_id) DO NOTHING` so re-imports don't regress state. Pattern matches `catalog/agents.go:164` adapted with ON CONFLICT (new query — see Pattern 7).
- **DO NOT call `replaceAgentSkills` (Phase 3 helper) for import row processing.** That helper uses DELETE-ALL + INSERT-N (PUT semantics). D5-18 mandates MERGE. Use `MergeAgentSkill` per skill in the import payload.
- **DO NOT amend migration 000002 to add `import_jobs`.** Phase 04.1 already amended 000002 (per D04_1-09); adding Phase 5's table into the same file mixes phase concerns. Use a new `000003`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CSV parsing (RFC 4180, quoted fields) | Custom split-by-comma | `encoding/csv` stdlib | Quoted fields, CRLF inside quotes, doubled quotes — RFC 4180 edge cases the stdlib handles. |
| BOM stripping | Read first N bytes manually | `bufio.Reader.Peek(3)` | Peek is non-destructive [CITED: pkg.go.dev/bufio#Reader.Peek]. |
| Streaming JSON array decode | `json.Unmarshal` to `[]interface{}` after `io.ReadAll` | `json.Decoder` Token+More loop (in principle) | Eager decode → memory blowup. **But v0.1 accepts strict-server eager decode; see §F1.** |
| Body-size limits | Custom `io.Reader` wrapper | `http.MaxBytesReader` + `*http.MaxBytesError` | Stdlib since 1.0; typed error since 1.19. |
| SAVEPOINT management | Raw `tx.Exec(ctx, "SAVEPOINT ...")` strings | `pgx.Tx.Begin()` returns a child Tx backed by SAVEPOINT | pgx idiom; avoids manual savepoint name uniqueness. |
| Per-entity upsert SQL | **[POST-04.1] Author UpsertXByExternalId** | **[POST-04.1] Phase 04.1's UpsertXByCode** | Phase 04.1 already shipped all 6 queries; Phase 5 invokes them. |
| Per-row cache invalidation orchestration | Hand-roll a transaction-with-cache module | Phase 3's `cache.Cache.Del` post-commit pattern (`catalog/agents.go:202`) | Phase 3 owns CAT-11 cache lifecycle; Phase 5 wraps it. |
| Idempotency-Key UUID parsing | Hand-roll a UUID validator | `openapi_types.UUID` (oapi-codegen runtime) | Already used everywhere in the codebase. |
| Transition logic for sweep goroutine | New ticker pattern | Mirror `state.Server.safetySweep` in `state/ttl.go:160` verbatim | Phase 4 paid the cost; Phase 5 inherits. |
| Per-entity row column registry | One-off `if entity == "agents" {...}` switches scattered throughout the handler | Single registry map: `entityRegistry: map[ImportEntityType]entityColumnRegistry` with required/optional column lists + typed dispatch func | Centralises the per-entity contract. |
| Code-format regex check | New regex compile per file | **[POST-04.1] Reuse `validateCodeFormat`** from `services/api/internal/catalog/codecheck.go:33` | Single source of truth for D04_1-03 regex; compiled once at package init. |
| Entity-specific 23505 error mapping | New mapping per entity | **[POST-04.1] Reuse `mapPgError`** from `services/api/internal/catalog/errors.go:48` | Already distinguishes `duplicate_code` from `duplicate_external_id` via constraint-name introspection (D04_1-21). |

**Key insight:** Bulk import touches every reusable layer of the stack (parsing, validation, persistence, cache, observability). The temptation is to write a "monolithic import service" — resist it. Each layer already has a Phase 1-4+04.1 pattern; Phase 5 is integration glue.

## Runtime State Inventory

> Phase 5 is greenfield code (a new package, new migration, new endpoint implementations). No renames, no refactors, no string replacements are involved. The Phase 04.1 amend checklist for CONTEXT.md is documentation-only and tracked separately.

**This section is intentionally OMITTED — no rename scope applies to Phase 5 code.**

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — new code | — |
| Live service config | None | — |
| OS-registered state | None | — |
| Secrets/env vars | None | — |
| Build artifacts | `services/api/internal/api/*.gen.go` will be regenerated by Wave 0 `task gen`; `web/packages/ui/src/api/generated.ts` regenerated by `pnpm gen:api`. All three committed per CONTRACT-04. | `task gen` regenerates; CI codegen-drift gate enforces. |

## Architectural Findings (PRESERVED from prior research per D04_1-26)

> These three findings are independent of the identity-model rename and were verified in the working tree during the prior research session. They survive intact and remain MANDATORY for the planner to address.

### F1. Strict-server eagerly decodes JSON (D5-23 architectural tension)

**What goes wrong:** The strict-server wrapper at `services/api/internal/api/server.gen.go` for `BulkImportCatalog` (verified verbatim in this session — quoted below) calls `json.NewDecoder(r.Body).Decode(&body)` to eagerly load the entire JSON request body into `req.JSONBody` BEFORE the handler runs whenever `Content-Type` starts with `application/json`. 50 MB raw JSON → ≈ 200-300 MB live `map[string]interface{}` representation. D5-23 says "JSON parsing is streaming, not eager" — but the strict-server defeats this for application/json.

**Verified verbatim from `server.gen.go`:**
```go
func (sh *strictHandler) BulkImportCatalog(w http.ResponseWriter, r *http.Request, orgId OrgIdPath, params BulkImportCatalogParams) {
    var request BulkImportCatalogRequestObject

    request.OrgId = orgId
    request.Params = params
    if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
        var body BulkImportCatalogJSONRequestBody
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            sh.options.RequestErrorHandlerFunc(w, r, fmt.Errorf("can't decode JSON body: %w", err))
            return
        }
        request.JSONBody = &body
    }
    if strings.HasPrefix(r.Header.Get("Content-Type"), "text/csv") {
        request.Body = r.Body
    }
    // ... dispatch to ssi.BulkImportCatalog
}
```

**Why it happens:** oapi-codegen's strict-server flavor is opinionated about request decoding for the common case (POST with typed body). It doesn't expose a per-operation knob to skip auto-decode.

**How to avoid:** Three options, in order of preference:

1. **Accept eager decode in v0.1.** Memory is bounded by D5-21 MaxBytesReader to 50 MB raw → ≈ 250 MB live max. Acceptable for v0.1 catalog scale. Document this in the architectural decisions.
2. Customize codegen with `client-server-templates` or a per-operation skip-decoding directive. Increases build complexity; not worth the risk for v0.1.
3. Split BulkImportCatalog into per-entity endpoints. Each endpoint has a typed body shape, eager decode becomes acceptable. But this contradicts the locked D5-14 single-endpoint contract.

**Recommendation:** Option 1. Pattern 4 implements the row-level recovery via re-marshal-from-interface.

**Warning signs:** Importing a 50 MB JSON file with 500 large rows causes the binary to allocate ≈ 250 MB heap before the handler runs. Profile early with a representative payload.

### F2. oapi-codegen v2 does not generate `AsX/FromX` for `oneOf` request bodies

**What goes wrong:** Following D5-25 literally and using `oneOf: [6 Import*Request]` for the request body schema generates a Go type with `union json.RawMessage` and ZERO accessor methods (oapi-codegen v2 generates `AsX/FromX` ONLY for response bodies as of v2.7.0). The handler has no way to decode it.

**Why it happens:** oapi-codegen issue #1620 (open since 2024) [VERIFIED: github.com/oapi-codegen/oapi-codegen/issues/1620] documents this as a known gap. v2 generates `AsX/FromX` for response oneOf but not request oneOf.

**How to avoid:** Keep the spec request body as `application/json: { type: array, items: {} }` (current Phase 2 state at `openapi.yaml:3092-3097`). Author the 6 new `Import*Request` schemas as `components.schemas` (so they appear in `types.gen.go` and clients can use them), but the request body schema points to `items: {}` (no oneOf). Handler dispatches by `?entity=` and re-marshals each `interface{}` element into the right typed struct via the round-trip helper in Pattern 4.

**Verified current shape (Phase 04.1 didn't change this):**
```go
// services/api/internal/api/types.gen.go:1337
type BulkImportCatalogJSONBody = []interface{}
```

**Warning signs:** `task gen` produces a generated type with a `union json.RawMessage` field and no accessor methods — that's the failure mode.

### F3. CONTEXT.md path drift: stubs live in `catalog/notimpl.go`, not `server/stubs.go`

**What goes wrong:** CONTEXT.md says `services/api/internal/server/stubs.go:230` / `:254`. The actual stubs live in `services/api/internal/catalog/notimpl.go:28,34` (verified during this re-research session — `services/api/internal/server/` contains only `health.go`, `health_test.go`, `request_id_exhaustiveness_test.go`, `server.go`, `server_test.go`).

**Why it happens:** Phase 4 replaced the Phase 2 stubs.go pattern with a composite `ApiHandlers` struct in `cmd/api/main.go:178-194` that embeds `*catalog.Handlers` and `*state.Server`. The stub methods were moved into `catalog/notimpl.go` so the composite still satisfies `StrictServerInterface`.

**How to avoid:** Phase 5 MUST:
1. Add a third embedded type to the `ApiHandlers` composite — e.g., `*imports.Server`.
2. Delete `services/api/internal/catalog/notimpl.go` (the file body is the two Phase 5 stubs Phase 04.1 already kept; once Phase 5 ships real implementations, the file is empty and should be removed).
3. Update the compile-time interface assertion at `cmd/api/main.go:189`: `var _ api.StrictServerInterface = (*ApiHandlers)(nil)` still holds because the `*imports.Server` provides the two Phase 5 methods.

**Verified current shape:**
```go
// services/api/cmd/api/main.go:178-194
type ApiHandlers struct {
    *catalog.Handlers
    *state.Server
}
var _ api.StrictServerInterface = (*ApiHandlers)(nil)
apiHandlers := &ApiHandlers{
    Handlers: catalogHandlers,
    Server:   stateServer,
}
```

**Phase 5 target shape:**
```go
type ApiHandlers struct {
    *catalog.Handlers
    *state.Server
    *imports.Server  // NEW
}
apiHandlers := &ApiHandlers{
    Handlers: catalogHandlers,
    Server:   stateServer,  // ambiguous selector? — see Open Q5
    // imports.Server fields...
}
```

**Open question — selector ambiguity:** Both `*state.Server` and `*imports.Server` would have type name `Server`. The composite literal `&ApiHandlers{Server: stateServer}` becomes ambiguous. Resolutions:
- Rename `imports.Server` to `imports.Handlers` for parallel naming with `*catalog.Handlers` (recommended).
- Or use field-tagging on the struct literal (Go allows the embedded field name to be the type name and Go's selector resolution handles it — but readability suffers).

**Recommendation:** Name the new type `imports.Handlers` to parallel `catalog.Handlers`. Composite literal becomes `&ApiHandlers{Handlers: ?, Server: stateServer, ?: importsHandlers}` — but `catalog.Handlers` already claims `Handlers`. The cleanest path is to name the new type something distinct (`imports.Server` like Phase 4 chose), accept that `Server` is ambiguous, and resolve by promoting one of them. This is captured in §Open Questions Q5.

## Common Pitfalls

### Pitfall 1: BOM consumed by `csv.Reader` as part of first column name (Pitfall 5.1)

**What goes wrong:** Excel-on-Windows saves CSV with UTF-8 BOM (3 bytes: 0xEF 0xBB 0xBF) at the start. Go's `encoding/csv` does NOT strip BOM. The first cell of the header row reads as `\xEF\xBB\xBFcode`, header validation fails with "missing required column code", admin gets a confusing 400.

**Why it happens:** Phase 2 RESEARCH and PITFALLS 5.1 documented this; the stdlib decision is to leave BOM handling to callers.

**How to avoid:** Pattern 3's `stripBOM` helper. Required to write a `testdata/windows-excel-agents.csv` fixture (UTF-8 BOM + CRLF + at least one quoted-with-embedded-comma field) and a regression test asserting the BOM file imports successfully.

**Warning signs:** CSV imports work in dev (macOS/Linux LF, no BOM) but fail on customer files. Always test with a real Excel-exported file.

### Pitfall 2: SQLChecker rejects sweep query without org_id

**What goes wrong:** The crash-recovery sweep SQL is `UPDATE import_jobs SET status='failed' WHERE status='pending' AND updated_at < NOW() - INTERVAL '24 hours' RETURNING id, org_id`. No org_id in the WHERE clause. SQLChecker's `MustContainOrgFilter` rejects this.

**Why it happens:** SQLChecker (`services/api/internal/db/sqlcheck.go:32-42`) requires org_id to appear in WHERE/INSERT/UPDATE for tables listed in `tenantTables`.

**How to avoid:** Two parts:
1. Add `"import_jobs": {}` to `tenantTables` (Phase 4 added `"agent_states": {}` at line 41 — same pattern).
2. The crash-sweep goroutine wraps its ctx with `db.WithBypass(ctx, "import_crash_sweep")` (mirror Phase 4 STATE-07 `state/ttl.go:179` `db.WithBypass(ctx, "wrapup_sweeper.safety")`). The bypass emits a structured slog event (`db/orgdb.go` audit) and SKIPS the org_id-presence check.

**Warning signs:** Sweep goroutine panics in dev (ValidationPanic mode) with `orgdb: SQL string missing org_id filter`.

### Pitfall 3: Missing `agent_states` INSERT on new agent import (Phase 4 Hazard 7)

**What goes wrong:** Phase 5's agent row processor upserts the `agents` table via `UpsertAgentByCode` but forgets to INSERT into `agent_states`. The agent appears in GET /agents but GET /agents/{id}/status returns 404, breaking IsRoutable downstream.

**Why it happens:** Phase 4 introduced the invariant that EVERY agent has a sibling `agent_states` row (`catalog/agents.go:164`). Phase 5's wider import path could miss this.

**How to avoid:** Phase 4 `04-PATTERNS.md` Hazard 7 + Phase 5 Inheritance Reminder. After every `UpsertAgentByCode` call inside the chunk's savepoint Tx, immediately call `qtx.InsertAgentStateOnConflictNothing(...)` with `ON CONFLICT (agent_id) DO NOTHING` so re-imports don't reset the state machine to Offline. Phase 5 authors this new query (Phase 4's existing `InsertAgentState` has no ON CONFLICT clause — adding ON CONFLICT to the existing query would break Phase 4 callers). A regression test (`TestBulkImport_SeedsAgentStatesForNewAgents`) is mandatory.

**Warning signs:** Importing 3 agents via CSV; GET /agents/{each}/status returns 404 for any of them.

### Pitfall 4: Per-row cache invalidation before chunk commit

**What goes wrong:** Handler calls `cache.Del(key)` for each successful row immediately. If the chunk commit fails, the cache is now empty for rows that never landed in the DB. Next GET re-loads stale data — appears correct (returns 404) but the cache state diverged.

**How to avoid:** Buffer successful row UUIDs per chunk in a local `succeededInChunk []succeededRow` slice. ONLY after `outerTx.Commit()` succeeds, walk that slice and call `cache.Del`. Mirror Phase 3's post-commit pattern (`catalog/agents.go:202`).

**Warning signs:** Tests fail intermittently with "cache hit returned stale row" after chunk-commit-failure injection.

### Pitfall 5: `Content-Length` header trust without MaxBytesReader fallback (D5-21)

**What goes wrong:** Handler trusts `Content-Length` only. Client sends `Content-Length: 1000` but the actual body is 100 MB. Pre-flight passes (`1000 < 50<<20`); handler reads body, OOMs.

**How to avoid:** D5-21 explicitly mandates BOTH the pre-flight AND `http.MaxBytesReader` wrap. The Pattern 2 middleware does both. Tests MUST include a "lying Content-Length" case.

### Pitfall 6: Skill `code` batch lookup N+1 (D5-16 / D5-17)

**What goes wrong:** Agent row processor calls `q.GetSkillByCode(ctx, {orgID, code})` per skill per row. 50 rows × 3 skills = 150 round trips.

**How to avoid:** Pattern 6's `ResolveSkillCodes(ctx, orgID, []codes)` query takes a single text array (`text[]`) and returns one batch. Handler collects all unique skill codes across the 50-row chunk, issues ONE lookup, builds `map[string]uuid.UUID`, re-uses for every row.

**Warning signs:** Performance tests show 10x slower per-row latency than expected. EXPLAIN ANALYZE shows 150 separate index scans where 1 should be.

### Pitfall 7: Idempotency-Key replay with `succeeded[]` reconstruction temptation

**What goes wrong:** Engineer sees that `import_jobs` stores counters + errors but not the original `succeeded[]` UUIDs. To avoid the "return empty succeeded" limitation, they decide to add a `succeeded_ids JSONB` column AT THE LAST MINUTE and ship a broken replay.

**How to avoid:** D5-13 EXPLICITLY locks v0.1 to return empty `succeeded[]` + `idempotent_replay: true`. The planner MUST NOT relitigate. v0.2 deferred to `succeeded_ids JSONB` enhancement.

### Pitfall 8: Tx.Begin on already-committed/rolled-back outer Tx (pgx savepoint footgun)

**What goes wrong:** Engineer puts the per-row savepoint Begin AFTER some failure handling that already rolled back the outer Tx; pgx returns `tx closed` error on `outerTx.Begin(ctx)`.

**How to avoid:** Structure the chunk loop as: outer Tx open → per-row savepoint inside loop (Begin → process → Commit/Rollback the savepoint) → after loop: outer Commit → defer outer Rollback handles only the no-op-after-commit case. The deferred Rollback after a Commit is a pgx no-op (`catalog/agents.go:122` documents this: "safe after commit — pgx ignores").

### Pitfall 9 [POST-04.1]: Forgetting to call `validateCodeFormat` for nested `skill_code`

**What goes wrong:** Phase 5's agent row processor validates the agent's own `code` via `validateCodeFormat` (per Phase 04.1's helper) but forgets to validate each `skill_code` in the nested `skills[]` array. A row with `skill_code: "INVALID UPPER"` reaches `ResolveSkillCodes` and returns "unknown_skill" — a confusing error because the real issue is the code format, not a missing skill.

**How to avoid:** Iterate `typed.Skills` BEFORE the skill resolver call; for each `sk.SkillCode`, call `validateCodeFormat(sk.SkillCode)` and emit `reason: invalid_code_format` on failure. Captured in D5-16/D5-17 amendments per the PHASE5-AMEND-CHECKLIST (blocks 3 + 4).

**Warning signs:** An admin sees "unknown_skill" for `SKILL_VOICE` (uppercase) instead of "invalid_code_format" — they create a new skill row with that exact uppercase value, hit `duplicate_external_id` on the next attempt, and conclude the system is broken.

### Pitfall 10 [POST-04.1]: Mis-using `replaceAgentSkills` for import (PUT vs PATCH semantics)

**What goes wrong:** Engineer wires Phase 5's agent row processor to call `catalog.replaceAgentSkills` (Phase 3's existing helper at `catalog/agent_skills.go:74`). That helper does DELETE-ALL + INSERT-N (PUT semantics — replaces the whole set). D5-18 mandates MERGE (PATCH semantics — preserves existing skills not in payload).

**How to avoid:** Do NOT call `replaceAgentSkills`. Author and call `MergeAgentSkill :exec` from Pattern 7 — ON CONFLICT (agent_id, skill_id) DO UPDATE preserves rows not in the import payload.

**Warning signs:** Admin imports a partial CSV that includes only "skill_voice"; an existing assignment to "skill_chat" disappears. Customer complaint about silent data loss.

## Code Examples

(See §Architecture Patterns §1-10 above. All code examples are verified against the working tree or stdlib docs; sources are cited inline.)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Per-row autocommit | Batched-savepoint with chunk size 50 | D5-09 | ~10× faster than per-row; chunk-atomic semantics. |
| Eager JSON load via `io.ReadAll` + `json.Unmarshal` | Streaming `json.Decoder` token-mode | D5-23 (aspirational) | Memory O(1 row). ⚠️ Defeated by strict-server eager decode in v0.1 — §F1. |
| Charset auto-detect (chardet) | UTF-8 strict + BOM strip | D5-08 + Pitfall 5.1 | Eliminates silent corruption. |
| Per-row N+1 lookup of skill code | Batch `ANY($1::text[])` resolve via `ResolveSkillCodes` | D5-16, D5-17 | 150 round trips → 1 per chunk. |
| `oneOf` for polymorphic request body | `items: {}` + `?entity=` dispatch | This research [VERIFIED: oapi-codegen issue #1620] | Hard library constraint. |
| `setInterval`-style WrapUp TTL | `time.Ticker` + `clockwork.Clock` + WaitGroup | Phase 4 ship | Phase 5 inherits identical pattern. |
| **[POST-04.1]** Per-entity `Upsert*ByExternalId` SQL (Phase 5 authors) | **[POST-04.1]** Phase 04.1's `Upsert*ByCode` SQL (Phase 5 invokes) | Phase 04.1 ship (D04_1-01, D04_1-18) | Phase 5 drops 6 query authoring tasks; only authors `ResolveSkillCodes`, `MergeAgentSkill`, `InsertAgentStateOnConflictNothing`, and the 5 `import_jobs` queries. |
| **[POST-04.1]** Per-handler 23505 mapping | **[POST-04.1]** Shared `mapPgError` with constraint-name introspection | Phase 04.1 ship (D04_1-21) | Phase 5 reuses; no new error code constants needed (3 added in 04.1). |

**Deprecated/outdated:**
- The pgx v4 SAVEPOINT pattern is superseded by pgx v5's `tx.Begin()` returning a child Tx.
- Authoring `UpsertXByExternalId` queries is OBSOLETE post-04.1 — use `UpsertXByCode`.
- `external_id` as the upsert key is OBSOLETE post-04.1 — `code` is canonical; `external_id` is optional integration-mapping (mutable).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | oapi-codegen v2.7.0's strict-server generates `json.NewDecoder(r.Body).Decode(&body)` for `application/json` bodies | §F1 | Low — verified verbatim in this session at `services/api/internal/api/server.gen.go` BulkImportCatalog wrapper. |
| A2 | Adding `"import_jobs": {}` to `tenantTables` is sufficient for SQLChecker to accept the new table's queries | Pitfall 2 | Low — exactly mirrors Phase 4's `"agent_states": {}` at sqlcheck.go:41. |
| A3 | Phase 3's per-entity CACHE invalidation pattern (`cache.Del(cache.Key(orgID, "agents", id))`) extends to bulk import | Don't Hand-Roll | Medium — the post-commit Del per succeeded row is correct because future GETs will repopulate. Verified pattern in `catalog/agents.go:202`. |
| A4 | `bufio.Reader.Peek(3)` is non-destructive on `*http.MaxBytesReader`-wrapped readers | Pattern 3 | Low — pkg.go.dev/bufio#Reader.Peek docs explicit. |
| A5 | `pgx.Tx.Begin()` returns a child Tx via SAVEPOINT, safe to call inside the hot row loop | Pattern 5 | Low — pgx v5 documented idiom. |
| A6 | 200-300 MB live memory for 50 MB raw JSON is acceptable at v0.1 catalog scale | §F1 | Medium — depends on deployment RAM. Most cloud Go binaries have ≥ 1 GB. Flag at deploy time. |
| A7 | The CONTEXT.md path `services/api/internal/server/stubs.go` is stale; actual stubs live in `services/api/internal/catalog/notimpl.go:28,34` | §F3 | High — VERIFIED via Read + ls. Planner must update file pointers. |
| A8 | Phase 4's `state.Server.Start/Stop` lifecycle pattern ports 1:1 to Phase 5's crash-sweep | Pattern 9 | Low — explicit RESEARCH+CONTEXT statement (D5-11) + verified Phase 4 ship behavior. |
| A9 | Chi middleware path-prefix scoping for `BodyLimit` works because the import URL has a unique prefix | Pattern 2 | Low — verified via openapi.yaml `paths:` survey. |
| A10 | Per-org rate limiting deferral is acceptable for v0.1 — hostile authenticated client could flood the endpoint | §Security Domain | Medium — explicitly accepted in v0.1; OrgContext is stub auth so all clients are "trusted" by current contract. |
| A11 [POST-04.1] | All 6 `UpsertXByCode` queries shipped in Phase 04.1 and have signatures covering every field on the corresponding `Create*Request` | §Pattern 6 | Low — verified by reading every query file in `services/api/internal/db/queries/` during this session. The 6 queries are in `agents.sql:127`, `skills.sql:101`, `queues.sql`, `channels.sql`, `adapters.sql:92`, `break_reasons.sql:113`. |
| A12 [POST-04.1] | Phase 04.1 did NOT author `UpsertAgentSkill` or `MergeAgentSkill`; Phase 5 authors it | §Pattern 7 | Low — verified via `grep -n "name: " services/api/internal/db/queries/agent_skills.sql` returning only InsertAgentSkill, DeleteAgentSkills, ListSkillsForAgent, SkillsPresentInOrg. |
| A13 [POST-04.1] | `InsertAgentState` in Phase 4 has NO `ON CONFLICT` clause; Phase 5 authors `InsertAgentStateOnConflictNothing` as a separate sqlc query | §Pattern 7 + Pitfall 3 | Low — verified by reading `agent_states.sql:1-7`. Adding ON CONFLICT to the existing query would change Phase 4's CreateAgent semantics. |
| A14 [POST-04.1] | OpenAPI spec still describes upsert keyed by `(org_id, external_id)` at 3 locations (lines 3067, 3212, items: {} at 3096) and must be amended in Phase 5 Wave 0 BEFORE codegen | §Summary New Finding 6 | Low — verified by grep in this session. Phase 04.1 explicitly scoped these out per D04_1-25 (Phase 5 amends on its own branch). |
| A15 [POST-04.1] | The 3 new error code constants (`ErrorCodeDuplicateCode`, `ErrorCodeDuplicateExternalId`, `ErrorCodeImmutableField`) already exist in `types.gen.go:87-89` | §Pattern 6 | Low — verified via grep. Phase 5 references them as typed constants; no codegen change required for these. |
| A16 [POST-04.1] | Phase 5 must add `*imports.Server` (or `*imports.Handlers`) as a THIRD embedded type in the `ApiHandlers` composite at `cmd/api/main.go:178` | §F3 | Low — verified composite shape in this session. Selector ambiguity if both `state.Server` and `imports.Server` are embedded — see Open Q5. |

**If this table is empty:** It is not. 16 assumptions captured. A3, A6, A7, A10 are MEDIUM-or-higher risk; planner must surface them in the plan-checker pass.

## Open Questions

1. **Idempotency-Key replay response shape — should `BulkImportResult` gain `idempotent_replay: boolean`?**
   - What we know: D5-27 says yes, add the field. CONTEXT.md explicitly calls this out as "Implementation Decision 5-27."
   - What's unclear: Whether the planner can add this field to `BulkImportResult` without breaking existing 200/207/422 consumers (the field is `nullable: true` with default false, so existing consumers ignore it).
   - Recommendation: Add as `nullable: true, default: false, readOnly: true`. Codegen-drift CI will catch any consumer break.

2. **Should the crash-sweep goroutine fall back to per-org iteration if `db.WithBypass` audit-log volume becomes a concern?**
   - What we know: `db.WithBypass` emits one structured slog event per bypass call. 1-hour ticker = 1 event per sweep (org-agnostic UPDATE → one bypass call).
   - What's unclear: Production v1 with 100s of orgs makes this nothing — the bypass is per-call, not per-row.
   - Recommendation: Accept bypass for v0.1; tag as v0.2 hardening item only if multi-replica land introduces concerns.

3. **Should `Import*Request` schemas be inline or in `openapi/components/imports.yaml`?**
   - What we know: Current spec is 3213 lines; Phase 2 deferred the spec-splitting until ~3000 lines. 6 new schemas adds ~250 lines, putting spec at ~3450.
   - Recommendation: Inline is acceptable for v0.1; flag as v0.2 refactor. Splitting now would require Phase 2 codegen pipeline changes that exceed the v0.1 risk budget.

4. **Migration filename: amend 000002 or new 000003?**
   - What we know: Phase 04.1 amended 000002 in place (D04_1-09). D-61 keeps 000002 editable until v0.1 SHIPS to a real environment. Phase 04.1 has not yet merged to main per STATE.md but is in this worktree.
   - What's unclear: Whether "ships" includes the Phase 04.1 PR landing on main or only customer-facing v0.1 release.
   - Recommendation: **Use 000003**. Rationale: (a) mixing Phase 04.1 catalog identity work with Phase 5 import_jobs in the same migration entangles PR review surface; (b) `import_jobs` is a Phase 5-owned table not a 04.1-owned schema change; (c) the migration is independent — applying 000003 against a 04.1-only DB still works. The "editable migration" policy was for catalog identity refactors where the row data is intertwined; `import_jobs` has no row dependency on catalog tables.

5. **[POST-04.1] How should the `ApiHandlers` composite handle the `Server` selector ambiguity?**
   - What we know: `*state.Server` is already embedded. Phase 5 needs to embed `*imports.Server` too. Both have the literal type name `Server`, making composite literal `&ApiHandlers{Server: stateServer}` ambiguous.
   - Recommendation: Name Phase 5's struct `imports.Handlers` (parallel to `catalog.Handlers`). The composite literal becomes:
     ```go
     type ApiHandlers struct {
         *catalog.Handlers
         *state.Server
         *imports.Handlers
     }
     // Field-name conflict on `Handlers` — Phase 5 must name its type differently.
     ```
     The cleanest path is to name Phase 5's struct `imports.Server` (mirror Phase 4) AND rename Phase 5's field to disambiguate from `state.Server`. Actually the simplest: keep `imports.Server` and rename the field at the composite struct (Go allows the embedded type's name OR an explicit field name):
     ```go
     type ApiHandlers struct {
         *catalog.Handlers
         StateServer   *state.Server    // explicit field name
         ImportsServer *imports.Server  // explicit field name
     }
     ```
     But this breaks the strict-server method resolution (method promotion requires the type to be embedded, not as a named field). The actual cleanest path: name Phase 5's package `imports` and its struct `Server`, then accept the ambiguity by qualifying at the composite literal call site:
     ```go
     apiHandlers := &ApiHandlers{
         Handlers: catalogHandlers,
         Server:   stateServer,    // disambiguate by source — first Server in declaration order
         // ...for imports: need a different field name
     }
     ```
     Go does NOT allow two anonymous fields of the same name within a struct. So **the planner MUST give the two structs different type names.** Recommendation: name Phase 5's struct `imports.Importer` (verb-noun, matches `state.Server` semantic of "owns a long-running responsibility"). Composite becomes:
     ```go
     type ApiHandlers struct {
         *catalog.Handlers
         *state.Server
         *imports.Importer
     }
     ```
     Method promotion works because all three type names are distinct.
   - This is a Wave 0 decision the planner must lock.

6. **Where does the 422 partial-failure case fire vs the 207 case?**
   - What we know: 200 = all succeed; 207 = some failed AND some succeeded; 422 = ALL failed.
   - What's unclear: An import with zero rows after header validation (admin uploads an empty CSV after the header) — is that 200 or 422?
   - Recommendation: 200 with empty arrays. 422 is "no rows succeeded"; an empty input vacuously has no failures either.

7. **Does `bodyLimit` middleware need to run BEFORE `orgContextMiddleware`?**
   - What we know: Mid-stream MaxBytesError happens during body read. orgContextMiddleware only reads the X-Org-Id header (cheap).
   - Recommendation: 400 first (cheap rejection before body read). Run `orgContextMiddleware` BEFORE `bodyLimit` so unauth requests short-circuit without ever wrapping the body. Matches Phase 2 hardening pattern.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Source build | ✓ (assumed) | 1.25.10 | — |
| PostgreSQL 17 | Migration + integration tests | ✓ (via docker-compose + testcontainers) | 17 | — |
| Redis 7+ | Phase 3 cache (Phase 5 reuses) | ✓ (via docker-compose + miniredis test fallback) | 9.x client | — |
| sqlc CLI | Codegen for new queries | ✓ (Phase 1+2 CI install) | v1.31.1 | — |
| oapi-codegen | Regen after spec edits | ✓ (Phase 2 install) | v2.7.0 | — |
| golang-migrate | Apply 000003 migration | ✓ (Phase 1 CI install) | v4.19.1 | — |
| Excel / LibreOffice (for golden testdata) | Generate windows-excel-agents.csv real Excel file | ⚠️ Development machine only | macOS/Linux Excel or LibreOffice | Author bytes manually via Python `b'\xef\xbb\xbf...'` if Excel unavailable. |

**Missing dependencies with no fallback:** none.

**Missing dependencies with fallback:** Excel binary access — author golden CSV via byte-level scripting if needed. Recommendation: have the user generate one Excel export during Wave 5 testing.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `go test` + `testify/require` + `testcontainers-go` (Phase 1+ pattern) |
| Config file | none — `services/api/go.mod` only |
| Quick run command | `task test:quick` → `cd services/api && go test -count=1 -short ./internal/imports/...` |
| Full suite command | `task test` → `cd services/api && go test -race -count=1 ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| IMP-01 | POST /catalog/import accepts JSON for each of 6 entities | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_JSON_<entity>` | ❌ Wave 5 |
| IMP-01 | POST /catalog/import accepts CSV for each of 6 entities | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_CSV_<entity>` | ❌ Wave 5 |
| IMP-02 | UTF-8 BOM stripped; CRLF handled; embedded quotes/commas/newlines decoded | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_WindowsExcel_BOM_CRLF` (`testdata/windows-excel-agents.csv`) | ❌ Wave 5 |
| IMP-02 | Invalid UTF-8 → 400 csv_not_utf8 | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_InvalidUTF8_400` | ❌ Wave 5 |
| IMP-03 | **[POST-04.1]** Upsert keyed by `(org_id, code)` via Phase 04.1's `UpsertXByCode`; same payload twice → no duplicates | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_Idempotent_DoubleRun_NoDuplicate` | ❌ Wave 5 |
| IMP-03 | **[POST-04.1]** Cross-org same-code-different-rows isolation (mirror `TestCatalog_CrossOrgSameCode_BothSucceed` at `test/isolation/catalog_test.go:375`) | integration | `go test -count=1 ./test/isolation/ -run TestImport_CrossOrgSameCode_BothSucceed` | ❌ Wave 5 |
| IMP-04 | Failed rows return structured errors with row + field + message | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_FailedRow_Structure` | ❌ Wave 5 |
| IMP-05 | 200 all succeed; 207 partial; 422 all fail | integration | `go test -count=1 ./internal/imports/ -run "TestBulkImport_HTTPStatus_(200\|207\|422)"` | ❌ Wave 5 |
| IMP-06 | import_jobs persisted; GET /imports/{id} returns it | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_GetImportJob_RoundTrip` | ❌ Wave 5 |
| IMP-07 | 50 MB → 413; 500 rows → 413 | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_Oversize_413` | ❌ Wave 5 |
| IMP-08 | CSV requires ?schema_version=v0.1; mismatch → 400 | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_SchemaVersion_Mismatch_400` | ❌ Wave 5 |
| D5-01 | Typed coercion pipeline | unit | `go test -count=1 ./internal/imports/ -run "TestCoerce_(Bool\|Int\|Multi)"` | ❌ Wave 2 |
| D5-09 | Chunked-savepoint: per-row failure rollback; chunk-level commit | integration (testcontainers Postgres) | `go test -count=1 ./internal/imports/ -run TestChunk_SavepointRollback` | ❌ Wave 3 |
| D5-11 | Crash sweep flips pending → failed after 24h | unit (clockwork) | `go test -count=1 ./internal/imports/ -run TestSweep_PendingOver24h_FlipsToFailed` | ❌ Wave 5 |
| D5-13 | Idempotency-Key replay returns prior result; new key proceeds; replay has `idempotent_replay=true` | integration | `go test -count=1 ./internal/imports/ -run "TestIdempotencyKey_(Replay\|MissProceed)"` | ❌ Wave 4 |
| D5-15 | **[POST-04.1]** Per-entity Import*Request schemas; FK by `code`; unknown skill_code → unknown_skill | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_UnknownSkillCode` | ❌ Wave 3 |
| D5-15 | **[POST-04.1]** Invalid code format on entity's own `code` → per-row `invalid_code_format` (handler delegates to validateCodeFormat) | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_InvalidCodeFormat_400OrPerRow` | ❌ Wave 3 |
| D5-16/17 | **[POST-04.1]** Invalid nested `skill_code` format → per-row `invalid_code_format` BEFORE skill resolution | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_NestedSkillCode_InvalidFormat_PerRow` | ❌ Wave 3 |
| D5-18 | Skill MERGE on update; existing skill not in import retained | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_SkillMerge_PreservesExisting` | ❌ Wave 3 |
| Hazard 7 | New agent import seeds agent_states row (ON CONFLICT DO NOTHING) | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_SeedsAgentStatesForNewAgents` | ❌ Wave 3 |
| Hazard 7 (re-import) | Re-importing an agent does NOT reset agent_states to Offline | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_Reimport_PreservesAgentStateMachine` | ❌ Wave 3 |
| FOUND-08 | Cross-org isolation: orgA imports, orgB GET /imports/{id} → 404 | integration | `go test -count=1 ./test/isolation/ -run TestImport_CrossOrg_404` | ❌ Wave 5 |
| FOUND-08 | **[POST-04.1]** Migration idempotency smoke (mirror `migration_idempotent_test.go`) for the new 000003 | integration | `go test -count=1 ./test/isolation/ -run TestMigration_ImportJobs_Idempotent` | ❌ Wave 5 |
| Pitfall 4 (cache) | Per-entity cache.Del fires only after chunk commit | unit (miniredis) | `go test -count=1 ./internal/imports/ -run TestChunk_CacheInvalidation_PostCommitOnly` | ❌ Wave 3 |
| Pitfall 5 (Content-Length lie) | 100MB body with Content-Length:1000 → 413 via MaxBytesError | integration | `go test -count=1 ./internal/middleware/ -run TestBodyLimit_LyingContentLength_413` | ❌ Wave 1 |

### Sampling Rate

- **Per task commit:** `task test:quick` (covers internal/imports/ unit tests, sub-30-second target).
- **Per wave merge:** `task test` (full suite including testcontainers Postgres; ≈ 1-2 minutes given Phase 4's full suite ran sub-2min on similar scope).
- **Phase gate:** Full suite green + `task gen` produces no diff + isolation suite passes + `task lint` clean before `/gsd:verify-work`.

### Wave 0 Gaps

- [ ] `services/api/internal/imports/coerce_test.go` — covers D5-01 / D5-02 / D5-04 / D5-06 / D5-07 / D5-08
- [ ] `services/api/internal/imports/parser_csv_test.go` — covers IMP-02 / D5-08 / Pitfall 1
- [ ] `services/api/internal/imports/parser_json_test.go` — covers D5-23 (eager-decode-is-acceptable-with-MaxBytes)
- [ ] `services/api/internal/imports/header_test.go` — covers D5-05 strict header policy
- [ ] `services/api/internal/imports/chunk_test.go` — covers D5-09 / D5-19 / Hazard 7 / Pitfall 10
- [ ] `services/api/internal/imports/jobs_test.go` — covers IMP-06 / D5-10
- [ ] `services/api/internal/imports/idempotency_test.go` — covers D5-13
- [ ] `services/api/internal/imports/sweep_test.go` — covers D5-11 (clockwork-backed)
- [ ] `services/api/internal/imports/handlers_test.go` — entity × format matrix (≥ 12 cases: 6 entities × 2 formats)
- [ ] `services/api/internal/imports/testutil_test.go` — shared httptest harness
- [ ] `services/api/internal/imports/testdata/windows-excel-agents.csv` — UTF-8 BOM + CRLF golden file (Pitfall 1 NON-NEGOTIABLE)
- [ ] `services/api/internal/imports/testdata/<entity>-<scenario>.{json,csv}` — at minimum 4 scenarios per entity
- [ ] `services/api/internal/imports/testdata/invalid-code-format.csv` — **[POST-04.1]** drives `invalid_code_format` per-row error
- [ ] `services/api/internal/middleware/bodylimit_test.go` — covers D5-21 / Pitfall 5
- [ ] `services/api/test/isolation/imports_test.go` — covers FOUND-08 cross-org probes
- [ ] `services/api/test/isolation/imports_cross_org_same_code_test.go` — **[POST-04.1]** mirror `TestCatalog_CrossOrgSameCode_BothSucceed` for import path

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (stub) | X-Org-Id header via OrgContext (Phase 1 carry-forward). v0.1 stub auth; full auth in AUTH milestone. |
| V3 Session Management | no | Stateless REST; no sessions in v0.1. |
| V4 Access Control | yes | OrgContext + SQLChecker enforce org-scope on every query. Phase 5 inherits. |
| V5 Input Validation | yes | D5-01..D5-08 typed coercion pipeline; D5-05 strict header policy; **[POST-04.1]** `validateCodeFormat` regex at handler boundary (Layer 1). |
| V6 Cryptography | no | No crypto authored. UUIDv7 is not a credential. |
| V8 Data Protection | yes | `BulkImportFailedRow.reason` deliberately does NOT echo raw row content (Phase 2 OQ-2A) → eliminates PII leakage. |
| V9 Communication | inherited | TLS/HTTPS terminated upstream. |
| V10 Malicious Code | yes | No new third-party deps (slopcheck N/A). |
| V12 Files & Resources | yes | Body size (D5-21) + row count (D5-22) limits. |
| V13 API & Web Service | yes | OpenAPI 3.0 spec contract; strict-server enforces. |
| V14 Configuration | inherited | docker-compose env. |

### Known Threat Patterns for {chi + pgx + stdlib CSV/JSON + Redis}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Body-size DoS (multi-GB POST) | Denial of Service | `http.MaxBytesReader` middleware + Content-Length pre-flight (D5-21, Pattern 2) |
| Row-count DoS (millions of tiny rows) | Denial of Service | Inline counter; exit at row 501 with 413 (D5-22) |
| Slow-loris on body stream | Denial of Service | `srv.ReadHeaderTimeout: 10s` (cmd/api/main.go:218) + chi default. ⚠️ no per-request body-read timeout in v0.1; flag for hardening. |
| JSON bomb (deeply nested) | Denial of Service | `json.Decoder` has no depth limits; relies on body size cap. 50 MB cap bounds worst-case stack depth. Acceptable for v0.1. |
| Billion-laughs-analog CSV (very wide row) | Denial of Service | `csv.Reader` has no field count limit; relies on body size cap. Per-entity column count caps real-world width. |
| SQL injection via CSV cell | Tampering | All writes go through sqlc-generated parameterized queries via OrgDB. Cells are passed as positional args. |
| **[POST-04.1]** Cross-org write via crafted `code` | Information Disclosure | `(org_id, code)` upsert key is org-scoped by composite UNIQUE (Phase 04.1). Phase 1 D-04 SQLChecker rejects any query missing org_id. Integration test required: orgA imports `code=foo`; orgB GET shows nothing. **The Phase 04.1 test `TestCatalog_CrossOrgSameCode_BothSucceed` already proves the schema invariant; Phase 5 mirrors it for the import path.** |
| **[POST-04.1]** Cross-org write via crafted `external_id` | Information Disclosure | Partial unique index `(org_id, external_id) WHERE external_id IS NOT NULL` is org-scoped. 23505 on `ix_*_org_external_id` → 409 `duplicate_external_id` via mapPgError introspection. |
| **[POST-04.1]** Code-injection via malformed `code` | Tampering | `validateCodeFormat` regex `^[a-z][a-z0-9_]{0,63}$` rejects shell metacharacters, SQL specials, path traversal. Handler returns 400 invalid_code_format BEFORE DB call. |
| Idempotency-Key replay against stale data | Tampering | Replay returns PERSISTED prior result; does NOT re-execute. |
| Idempotency-Key exhaustion | Denial of Service | Client-supplied UUIDv7 → 2^122 keyspace. Per-org spam still possible (rate limiting deferred). |
| Org_id propagation lost in goroutine | Information Disclosure | Crash-sweep goroutine uses `db.WithBypass(ctx, "import_crash_sweep")` per D-04 carry-forward; structured slog audit on every bypass. Fresh ctx with `context.Background()` per Phase 4 ttl.go:71 pattern. |
| Eager-decode memory blowup → OOM | Denial of Service | MaxBytesReader bounds raw to 50 MB → live ≤ ~300 MB. Below 1 GB Go binary default. |
| `slog.Debug("import: int_truncation", ..., "raw", "7.5")` log inflation | Denial of Service | D5-07 emits debug-level only. Per-row in a 500-row import = 500 log lines worst case. Acceptable. |
| BOM bytes consumed as field data | Tampering | Pattern 3 stripBOM + utf8.Valid → 400 csv_not_utf8 on invalid. |
| Schema-version-skew silent corruption | Tampering | D5-05 strict header policy: unknown column → 400 at batch level. |

**v0.1-accepted risks (explicit):**
- Per-org rate limiting is deferred.
- Distributed lock for crash-sweep is deferred (multi-replica race is benign — idempotent UPDATE).
- Reverse-proxy body-size alignment is a deploy concern (Pitfall 5.3).

## Sources

### Primary (HIGH confidence — verified in this session)

- **Working tree code (verified by Read + grep):**
  - `services/api/internal/api/server.gen.go` BulkImportCatalog strict-handler wrapper (eager JSON decode — quoted in §F1)
  - `services/api/internal/api/types.gen.go:87-89` (`ErrorCodeDuplicateCode`, `ErrorCodeDuplicateExternalId`, `ErrorCodeImmutableField` — Phase 04.1 shipped)
  - `services/api/internal/api/types.gen.go:1337` (`BulkImportCatalogJSONBody = []interface{}`)
  - `services/api/internal/api/types.gen.go:156-167` (`ImportEntityType` enum + `Valid()` method)
  - `services/api/internal/api/types.gen.go:589-622` (`BulkImportFailedRow`)
  - `services/api/internal/catalog/codecheck.go:28-46` (Phase 04.1 helpers: `codeFormat`, `validateCodeFormat`, `validateImmutableCode`)
  - `services/api/internal/catalog/errors.go:48-92` (Phase 04.1 `mapPgError` with constraint-name introspection)
  - `services/api/internal/catalog/notimpl.go:28,34` (current Phase 5 stubs — CORRECTED path)
  - `services/api/internal/state/ttl.go:30-196` (Phase 4 STATE-07 goroutine pattern to mirror)
  - `services/api/internal/state/ttl.go:179` (`db.WithBypass(ctx, "wrapup_sweeper.safety")` — sweep bypass pattern)
  - `services/api/internal/db/orgdb.go:162-228` (`OrgTx` struct + `BeginTx` method — Phase 5 extends with `BeginSavepoint`)
  - `services/api/internal/db/sqlcheck.go:32-42` (`tenantTables` — Phase 5 adds `"import_jobs": {}`)
  - `services/api/internal/db/bypass.go:28-46` (`WithBypass` + `BypassReason` mechanism)
  - `services/api/internal/db/queries/agents.sql:119,127` (`GetAgentByCode`, `UpsertAgentByCode` — Phase 04.1)
  - `services/api/internal/db/queries/skills.sql:93,101` (`GetSkillByCode`, `UpsertSkillByCode` — Phase 04.1)
  - `services/api/internal/db/queries/queues.sql` (per equivalent pattern — Phase 04.1)
  - `services/api/internal/db/queries/channels.sql` (per equivalent pattern — Phase 04.1)
  - `services/api/internal/db/queries/adapters.sql:84,92` (`GetAdapterByCode`, `UpsertAdapterByCode` — Phase 04.1)
  - `services/api/internal/db/queries/break_reasons.sql:105,113` (`GetBreakReasonByCode`, `UpsertBreakReasonByCode` — Phase 04.1)
  - `services/api/internal/db/queries/agent_skills.sql:29-79` (`InsertAgentSkill`, `DeleteAgentSkills`, `ListSkillsForAgent`, `SkillsPresentInOrg` — Phase 3 only; NO `UpsertAgentSkill` or `MergeAgentSkill` — Phase 5 must author)
  - `services/api/internal/db/queries/agent_states.sql:1-7` (`InsertAgentState` without ON CONFLICT — Phase 5 must author `InsertAgentStateOnConflictNothing`)
  - `services/api/internal/catalog/agents.go:111,124,164,202` (UUIDv7 mint + qtx + InsertAgentState + cache.Del pattern — Phase 5 mirrors)
  - `services/api/internal/catalog/agent_skills.go:74` (Phase 3 `replaceAgentSkills` — PUT semantics; Phase 5 must NOT call this — see Pitfall 10)
  - `services/api/internal/cache/cache.go:104,124,254` (`Key`, `GetOrSet`, `Del` signatures)
  - `services/api/internal/middleware/httputil.go` (`WriteError` contract)
  - `services/api/cmd/api/main.go:178-194` (`ApiHandlers` composite — Phase 5 adds third embedded type)
  - `services/api/internal/server/server.go` (middleware chain order: Recoverer → RequestID → orgContextMiddleware → uuidv7PathParams → strict-server)
  - `migrations/000002_catalog_v0_1.up.sql:40,143,166` (Phase 04.1 schema verified: `code TEXT NOT NULL` + `external_id TEXT NULL` + partial unique indexes on all 6 entities; `break_reasons.UNIQUE(org_id, name)` confirmed absent)
  - `openapi/openapi.yaml:1454-1562` (`ImportEntityType`, `BulkImportFailedRow`, `BulkImportResult`, `ImportJob` — current spec; lacks Phase 5 additions)
  - `openapi/openapi.yaml:3050-3169` (BulkImportCatalog operation — still says upsert keyed by `(org_id, external_id)` at line 3067 + line 3212; items: {} at line 3096)
  - `services/api/test/isolation/catalog_test.go:375` (`TestCatalog_CrossOrgSameCode_BothSucceed` — Phase 04.1 reference test for Phase 5 to mirror)
  - `services/api/test/isolation/migration_idempotent_test.go:1-50` (Phase 04.1 idempotency test pattern — Phase 5 mirrors for `000003_create_import_jobs`)
  - `.planning/phases/04.1-catalog-identity-normalization/04.1-PHASE5-AMEND-CHECKLIST.md` (10 rename blocks for Phase 5 CONTEXT.md)
  - `.planning/phases/04.1-catalog-identity-normalization/04.1-06-SUMMARY.md` (Phase 04.1 ship state confirmation)
  - `services/api/go.mod` (current versions: pgx v5.9.2, oapi-codegen v2.7.0, clockwork v0.4.0, uuid v1.6.0)

- **Official Go stdlib docs:**
  - [pkg.go.dev/encoding/csv](https://pkg.go.dev/encoding/csv) — CRLF auto-conversion, LazyQuotes default false, FieldsPerRecord semantics
  - [pkg.go.dev/net/http#MaxBytesReader](https://pkg.go.dev/net/http#MaxBytesReader) — typed `*MaxBytesError`
  - [pkg.go.dev/encoding/json#Decoder](https://pkg.go.dev/encoding/json#Decoder) — Token+More streaming
  - [pkg.go.dev/bufio#Reader.Peek](https://pkg.go.dev/bufio#Reader.Peek) — non-destructive Peek
  - [pkg.go.dev/unicode/utf8#Valid](https://pkg.go.dev/unicode/utf8#Valid)
  - [pkg.go.dev/github.com/jackc/pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5) — pgx v5 SAVEPOINT idiom via `Tx.Begin()`

- **Phase 1-4 + 04.1 RESEARCH + CONTEXT + PATTERNS + SUMMARY docs (CARRY-FORWARD LOCKED):**
  - `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` — D-01..D-31
  - `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` — D-32..D-48
  - `.planning/phases/03-catalog-crud-go/03-VERIFICATION.md` — D-49..D-77
  - `.planning/phases/04-agent-state-machine-go/04-PATTERNS.md` — Hazard 7 + Phase 5 Inheritance Reminder
  - `.planning/phases/04.1-catalog-identity-normalization/04.1-CONTEXT.md` — D04_1-01..D04_1-26
  - `.planning/phases/04.1-catalog-identity-normalization/04.1-RESEARCH.md` — Pitfalls 1, 5, 7 carry-forward
  - `.planning/phases/04.1-catalog-identity-normalization/04.1-{01..06}-SUMMARY.md` — Phase 04.1 ship state
  - `.planning/research/PITFALLS.md` §5 — Bulk import pitfalls 5.1-5.4

### Secondary (MEDIUM confidence — WebSearch verified with stdlib/repo docs)

- [github.com/oapi-codegen/oapi-codegen issue #1620 (oneOf request body)](https://github.com/oapi-codegen/oapi-codegen/issues/1620) — confirms AsX/FromX NOT generated for request-body oneOf in v2 (carry-forward from prior research)
- [Go issue #9588 (CSV BOM not stripped)](https://github.com/golang/go/issues/9588) — confirms stdlib decision
- [RFC 4180](https://datatracker.ietf.org/doc/html/rfc4180) — CSV base format
- [Idempotency-Key draft RFC](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/) — header semantics

## Metadata

**Confidence breakdown:**

- **Standard stack:** HIGH — every package is already in `services/api/go.mod`; versions verified via working tree.
- **Architecture patterns:** HIGH — patterns 1-10 are either direct ports of shipped Phase 1-4+04.1 code (verified by Read) or stdlib idioms (verified by pkg.go.dev). Pattern 4 (re-marshal under strict-server eager decode) is the only MEDIUM-confidence design compromise; surface for user confirmation.
- **Pitfalls:** HIGH — pitfalls 1-10 each cite either a working-tree code line, a Phase 4 PATTERNS.md hazard, or a Pitfall 5.x research entry.
- **Validation architecture:** HIGH — Phase 1-4 test framework shipped and proven; Phase 5 inherits 1:1.
- **Security domain:** MEDIUM — ASVS categories mapped; two MEDIUM-risk items explicit (eager-decode memory, rate-limiting absence).
- **Open questions:** HIGH — 7 specific open questions with recommendations.
- **Architectural risk surfacing:** HIGH — three preserved findings (oapi-codegen oneOf, eager-decode, stubs path drift) + four new POST-04.1 findings (Phase 04.1 query inventory, missing agent_skills MERGE, OpenAPI spec amendments, migration sequence) are explicitly called out in the §Summary.

**Research date:** 2026-05-17 (re-research)

**Valid until:** 2026-06-16 (30 days for stable; recheck oapi-codegen v2.7+ release notes if any new minor lands)
