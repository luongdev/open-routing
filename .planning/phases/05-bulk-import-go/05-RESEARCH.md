# Phase 5: Bulk Import (Go) - Research

**Researched:** 2026-05-17
**Domain:** HTTP request streaming + CSV/JSON parsing + per-entity SQL upsert orchestration in Go (chi + pgx v5 + sqlc + oapi-codegen v2 strict-server)
**Confidence:** HIGH (everything that the planner needs to lock can be verified against shipped Phase 1-4 code + official Go stdlib docs + pgx v5 docs + oapi-codegen behaviour observed in the working tree)

## Summary

Phase 5 turns the two existing `not_implemented` strict-server stubs in
`services/api/internal/catalog/notimpl.go` (lines 28, 34) into a real
synchronous bulk-import endpoint that accepts JSON or CSV bodies for any
of the 6 catalog entities, performs `(org_id, external_id)` upserts with
per-row partial-success accounting, persists results in a new
`import_jobs` table, and supports optional `Idempotency-Key` replay.

The phase has a small surface of **truly new code**: a new
`internal/imports/` package (the planner picks the exact name —
`internal/import_pkg/` is a Go keyword conflict and must be avoided),
one new migration, a handful of new sqlc queries, six new request
schemas in `openapi.yaml`, a new chi middleware for body-size
enforcement, and a crash-recovery goroutine that mirrors Phase 4
STATE-07's `clockwork`-backed pattern. Everything else (orgDB validator,
strict-server pipeline, request-id injection, cache invalidation,
two-org isolation harness, UUIDv7 minting, slog/OTel attribution) is
inherited from Phases 1-4 without modification.

**Three architecture-shaping discoveries surfaced during research that
the planner MUST plan around:**

1. **Phase 3 did NOT author upsert-by-external-id SQL.** Only
   `Insert*` (raw insert; fails on duplicate `(org_id, external_id)`)
   and `Update*` (atomic version-checked update keyed by `id` + `version`)
   exist. CONTEXT.md flagged this as a possibility (line 553-557); the
   working tree confirms it. Phase 5 MUST author per-entity
   `Upsert{Entity}ByExternalId` sqlc queries — this is approximately 7
   new SQL queries (one per entity + one for the agent_skills MERGE).
2. **The strict-server eagerly decodes JSON.** `services/api/internal/api/server.gen.go:5629-5641`
   shows the generated wrapper calls `json.NewDecoder(r.Body).Decode(&body)`
   on the FULL request body before the handler runs whenever
   `Content-Type` starts with `application/json`. This conflicts with
   D5-23 (streaming JSON decoder). True streaming requires either (a)
   `http.MaxBytesReader` middleware that bounds memory to 50 MB
   regardless of eager decode (Phase 5's only viable v0.1 path), or (b)
   custom codegen options to disable auto-decode (out of scope for v0.1).
   `oapi-codegen` does NOT generate `AsX/FromX` helpers for `oneOf`
   request bodies in v2.7.0 [VERIFIED: oapi-codegen issue #1620, Sept 2024]
   — `BulkImportCatalogJSONBody` is currently `[]interface{}` and any
   per-row typed validation MUST happen in the handler after the
   eager decode lands (or by re-decoding `json.RawMessage` slices).
3. **CONTEXT.md path-of-stubs drift.** CONTEXT.md says
   `services/api/internal/server/stubs.go:230` / `:254`. The actual
   stubs live in `services/api/internal/catalog/notimpl.go:28,34`.
   Phase 4 replaced the Phase 2 stubs.go pattern with a composite
   `ApiHandlers` struct in `cmd/api/main.go:178-194` that embeds
   `*catalog.Handlers` and `*state.Server`. **Phase 5 MUST add a
   third embedded type** (e.g., `*imports.Handlers`) to the
   `ApiHandlers` composite, and replace `catalog/notimpl.go` (deleted
   by Phase 5 once the real implementations land).

**Primary recommendation:** Sequence Phase 5 in 6 waves: (W0) Spec
edits + codegen + migration + middleware skeleton; (W1) sqlc upsert
queries for all 6 entities + import_jobs queries + body-limit
middleware unit tests; (W2) Coercion + typed-parser pipeline
(`coerce.go` + `parser_json.go` + `parser_csv.go`); (W3) Per-entity
row processors (one file per entity) + `import_jobs` lifecycle;
(W4) Handler + ApiHandlers composite re-wiring + idempotency replay;
(W5) Crash-recovery goroutine + integration tests (entity × format
matrix + isolation tests + golden Excel CSV). Total ≈ 9-11 plans,
mirroring Phase 3's catalog plan count.

## User Constraints (from CONTEXT.md)

### Locked Decisions

**D5-01 (CSV cell type coercion pipeline).** Every CSV cell goes through
a typed pipeline `trim → lower → split → parse` BEFORE validation. The
pipeline is dispatched by the DB field type (resolved from
sqlc-generated Go types — `bool`, `int`, `string`, `pgtype.Text`, etc.).
One central dispatcher; per-type coercer; per-entity validator runs only
on post-coercion typed values.

**D5-02 (Bool coercion is loose).** Accept
`true|false|TRUE|FALSE|1|0|yes|no` (case-insensitive after
`trim → lower`). Any other value → per-row failure with
`reason: invalid_bool`.

**D5-03 (Empty cells = null/skip).** On create: spec default applies. On
update: field left unchanged. On a required column: per-row failure with
`reason: missing_required`.

**D5-04 (Multi-value cells delimited by `|`, fallback `;` then `,`).**
Priority `|` > `;` > `,`. CSV field-level `,` unescaping happens first
(RFC 4180).

**D5-05 (Strict batch-level header policy).** Missing required column →
HTTP 400 `{error: invalid_body, reason: missing_columns, missing: [...]}`.
Unknown column → HTTP 400 `{error: invalid_body, reason: unknown_columns,
unknown: [...]}`. NO rows processed.

**D5-06 (Enums case-insensitive auto-lower).** `voice`/`Voice`/`VOICE` →
`voice`. No-match → per-row `reason: invalid_enum`.

**D5-07 (Integers tolerate leading float).** Pipeline:
`trim → strconv.ParseFloat → math.Trunc → int`. Excel's `7.0` truncates
to `7`. `"seven"` → per-row `reason: invalid_int`. Truncation discarding
a non-zero fractional part emits `slog.Debug("import: int_truncation",
"row", N, "field", F, "raw", "7.5", "value", 7)`.

**D5-08 (UTF-8 only after BOM strip).** Validate with
`utf8.Valid([]byte)`. Invalid → HTTP 400
`{error: invalid_body, reason: csv_not_utf8}`. NO Latin-1 fallback.

**D5-09 (Batched-savepoint transactions; chunk size = 50 rows).** 500-row
max → ≤ 10 chunks. Each chunk = 1 tx + per-row savepoint. Failed rows
do NOT abort chunk; chunk abort only on infrastructure error.

**D5-10 (`import_jobs` row at start, finalised at end).** INSERT
`status='pending'` BEFORE chunk 1. UPDATE counters + `errors` JSONB +
`status='completed'` after chunk 10. Job INSERT is its own short tx.

**D5-11 (Crash-recovery sweep goroutine; 24h TTL).** Reuse Phase 4
STATE-07 pattern. Tick interval: 1 hour. UPDATE pending rows older than
24h to `status='failed'` with synthetic error entry. Single-replica;
v1 multi-replica needs distributed lock (STATE.md blocker).

**D5-12 (`errors` JSONB shape mirrors `BulkImportResult.failed[]`).**
`[{"row": 3, "field": "email", "error": "import_failed", "reason":
"invalid email format"}, ...]`. Zero transformation.

**D5-13 (`Idempotency-Key` header optional + persisted).** Column
`import_jobs.idempotency_key TEXT NULL` with `UNIQUE (org_id,
idempotency_key) WHERE idempotency_key IS NOT NULL`. Hit → return prior
job's `BulkImportResult` from `errors` JSONB + counters. Known v0.1
limitation: `succeeded[]` UUID list NOT reconstructable from counters →
replay returns `succeeded: []` + `idempotent_replay: true`.

**D5-14 (`?entity=` query is canonical; Content-Type selects parser).**
Generated `BulkImportCatalogRequestObject` exposes `JSONBody
*BulkImportCatalogJSONRequestBody` AND `Body io.Reader`. Handler
dispatches: `if req.JSONBody != nil` → JSON path; else → CSV path on
`req.Body`. Unsupported Content-Type → strict-server 415.

**D5-15 (Per-entity `Import*Request` schemas — NOT `Create*Request`).**
6 new schemas. FKs by `external_id` (human-readable), NOT by `id`
(server-minted UUIDv7).

**D5-16 (JSON agent imports support nested `skills[]`).**
`ImportAgentRequest.skills: [{skill_external_id, proficiency}, ...]`.
Lookup `(org_id, skill_external_id)` → skill UUID. Unknown skill →
per-row failure (whole agent row fails).

**D5-17 (CSV agent imports support skills via `skills` column).** Single
cell `"SKILL_VOICE:7|SKILL_CHAT:9|SKILL_EMAIL:5"`. Pipeline: trim →
split by D5-04 priority → split each token on `:` → validate.

**D5-18 (Skill MERGE semantics on agent update — PATCH-like, NOT PUT).**
Import `skills` array MERGES into existing `agent_skills`. Existing
skills NOT in the payload are LEFT INTACT. Diverges from
`UpdateAgentRequest` PUT semantics. **Trade-off:** no skill REMOVAL via
import (UI or PATCH only).

**D5-19 (Proficiency conflict → import value wins).** `ON CONFLICT
(agent_id, skill_id) DO UPDATE SET proficiency = EXCLUDED.proficiency`.

**D5-20 (Other entity types have no N:M in v0.1 import scope).**
Skills/queues/channels/adapters/break_reasons are flat in import.

**D5-21 (50 MB cap via Content-Length pre-flight + `http.MaxBytesReader`).**
Order: (a) Content-Length > 50<<20 → 413 immediately; (b) wrap r.Body
in MaxBytesReader BEFORE any parser sees it. Reusable middleware
factory in `services/api/internal/middleware/bodylimit.go`.

**D5-22 (500-row cap enforced inline during streaming parse).** Exit at
row 501 with 413. Streaming for both JSON token-mode + CSV reader-loop.

**D5-23 (JSON parsing is streaming, not eager).** `json.Decoder`
token-mode. Per-row decode failure → record in `failed[]`, continue.
Syntactic error → abort with 400 `malformed_json`. ⚠️ **See Architecture
Patterns § "JSON streaming under oapi-codegen v2 eager decode" — this
decision is structurally in tension with the generated strict-server
wrapper that eagerly decodes the JSON body BEFORE the handler runs.
Research recommends accepting eager decode in v0.1 (memory-bounded by
D5-21 MaxBytesReader) and revisiting for v0.2 async path. The planner
must surface this for user confirmation.**

**D5-24 (Content-Type dispatch is header-only — strict).** No magic-byte
sniffing, no `?format=` query. Missing/unrecognised → strict-server 415.

**D5-25 (6 new request schemas in openapi.yaml).** `ImportAgentRequest`,
`ImportSkillRequest`, `ImportQueueRequest`, `ImportChannelRequest`,
`ImportAdapterRequest`, `ImportBreakReasonRequest`. Each mirrors
`Create*Request` with FK fields swapped to `*_external_id`.
`ImportAgentRequest` carries `skills: [{skill_external_id,
proficiency}]`. Request body schema becomes `oneOf: [6 schemas]` or
per-entity discriminator. ⚠️ **See Architecture Patterns § "oneOf
request body unsupported by oapi-codegen v2" — the planner MUST pick
`items: {}` (current) over `oneOf` and let the handler dispatch by
`?entity=` because oapi-codegen v2.7.0 does NOT generate AsX/FromX
helpers for oneOf request bodies (only for response bodies). This is a
hard library constraint; D5-25's "oneOf or per-entity discriminator"
both fail.**

**D5-26 (`ImportJob.status` enum: pending|completed|failed).** Required.
v0.1 returns `completed` or `failed`; `pending` only via crash window /
sweep transition.

**D5-27 (Optional `Idempotency-Key` header parameter + `idempotent_replay`
boolean on `BulkImportResult`).**

### Claude's Discretion

- Exact internal package layout: `internal/imports/` (recommended —
  `import_pkg` was floated but `import` is a Go reserved word so package
  must be named something else; `internal/imports/` is the natural
  English plural, parallel to `internal/catalog/`).
- Goroutine sweep tick interval (recommend 1h).
- `import_jobs` migration filename: planner chooses sequence (likely
  `000003_create_import_jobs.up.sql` since Phase 3+4 both appended to
  `000002_catalog_v0_1.up.sql`).
- Whether per-entity `Import*Request` lives inline vs in a separate
  `openapi/components/imports.yaml` (inline recommended — current spec
  is 2829 lines, well under the ~3000 line split threshold).
- Test data file convention: `services/api/internal/imports/testdata/<entity>-<scenario>.{json,csv}`
  recommended (mirrors Go convention; same-package test files can use
  `testdata/` automatically via `os.ReadFile`).

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
| IMP-01 | POST /catalog/import?entity={type} accepts JSON or CSV body for all 6 entities | Architecture Patterns §1 (Handler dispatch) + Code Examples §1 |
| IMP-02 | CSV parser normalises UTF-8 BOM + CRLF + LF + embedded quotes/commas/newlines | Code Examples §3 (BOM strip + csv.Reader defaults) + Pitfall 5.1 |
| IMP-03 | Upsert keyed by (org_id, external_id) via INSERT ... ON CONFLICT ... DO UPDATE | Standard Stack §sqlc + Code Examples §4 (per-entity upsert SQL); critical gap — Phase 3 did NOT author these queries |
| IMP-04 | Failed rows return structured errors with row + field + message | D5-12 + Code Examples §6 (`BulkImportFailedRow` shape) |
| IMP-05 | HTTP 207 partial success / 200 all success | Architecture Patterns §1 (strict-server response objects) |
| IMP-06 | import_jobs persistence + GET /imports/{id} | Standard Stack §migration + Code Examples §7 (import_jobs migration + sqlc queries) |
| IMP-07 | 50 MB / 500 row cap → 413 | Code Examples §2 (MaxBytesReader middleware + inline row counter) + Pitfall 5.3 |
| IMP-08 | CSV requires ?schema_version=v0.1; mismatch → 400 with supported versions | Code Examples §5 (schema version validation) + Pitfall 5.4 |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| HTTP request body intake + size enforcement | API / chi middleware | — | Stop large bodies BEFORE they reach the handler; chi middleware is the canonical seam (mirror Phase 1 OrgContext + UUIDv7PathParams). |
| Streaming JSON / CSV parsing | API / handler | — | Per-row error recovery requires handler-level control (cannot live in middleware which is single-pass). |
| Typed coercion pipeline (D5-01) | API / handler (pure functions) | — | Pure transform; testable in isolation; reusable across formats. |
| Per-entity row validation (Layer 2) | API / handler | — | Mirrors Phase 3's `validateProficiencyRange` pattern in `catalog/agent_skills.go:11`. |
| Per-entity SQL upsert | DB / pgx + sqlc | — | All writes flow through OrgDB; SQLChecker validates `org_id = $N` on every query. |
| Skill external_id → UUID resolution | DB / pgx + sqlc | API / handler caches per-request | Single batch lookup query reused across all rows in chunk; handler caches `map[external_id]UUID` for the request lifetime to avoid N+1. |
| `import_jobs` lifecycle | DB / pgx + sqlc | — | Single short tx for create; single short tx for finalise. |
| Crash-recovery sweep | API / cmd/api goroutine | DB / pgx | Mirror Phase 4 STATE-07 pattern: time.Ticker in cmd/api, bypass-marked ctx for SQLChecker. |
| Idempotency-Key replay | API / handler | DB / pgx | Lookup `(org_id, key)` BEFORE any work; rehydrate `BulkImportResult` from persisted job. |
| Cache invalidation | Phase 3's `cache.Cache` | API / handler | Phase 3's per-entity upsert helpers (which Phase 5 authors) MUST call `cache.Del` post-commit, mirroring `catalog/agents.go:190` (CAT-11). Phase 5 does NOT duplicate this — the helper owns the cache key. |
| OTel + slog attribution | App / telemetry | — | All inherited from Phase 1 (D-29 TracingHandler). Sweep goroutine emits `event=import_crash_sweep` per D5-11. |

## Standard Stack

### Core (already in go.mod, no install required)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/csv` (stdlib) | go 1.25 | CSV parsing | Only stdlib option; RFC 4180-compliant; well-tested. No third-party alternative offers a meaningful advantage at v0.1 scale. [VERIFIED: pkg.go.dev/encoding/csv] |
| `encoding/json` (stdlib) | go 1.25 | JSON parsing (streaming via `json.Decoder` token-mode) | Stdlib; same Decoder.Token + Decoder.More + Decoder.Decode pattern used in `cmd/migrate` for OpenAPI spec load. [VERIFIED: pkg.go.dev/encoding/json] |
| `net/http` (stdlib) | go 1.25 | `http.MaxBytesReader` + `*http.MaxBytesError` for body-size enforcement | Stdlib since Go 1.0; `*MaxBytesError` typed error since Go 1.19 (released 2022). [VERIFIED: pkg.go.dev/net/http#MaxBytesReader] |
| `bufio` (stdlib) | go 1.25 | Wrap r.Body for `Peek(3)` BOM detection | Stdlib; canonical pattern for "look at first 3 bytes without consuming" required by D5-08. [VERIFIED: pkg.go.dev/bufio#Reader.Peek] |
| `unicode/utf8` (stdlib) | go 1.25 | `utf8.Valid([]byte)` for D5-08 strict UTF-8 enforcement | Stdlib. [VERIFIED: pkg.go.dev/unicode/utf8] |
| `strconv` (stdlib) | go 1.25 | `ParseFloat` + `Atoi` + `ParseBool` for D5-02 / D5-07 coercion | Stdlib. [VERIFIED: pkg.go.dev/strconv] |
| `math` (stdlib) | go 1.25 | `math.Trunc` for D5-07 integer-from-float truncation | Stdlib. [VERIFIED: pkg.go.dev/math#Trunc] |
| `github.com/google/uuid` | v1.6.0 (already direct) | `uuid.NewV7()` for `import_jobs.id` | Carry-forward from Phase 1 D-19. [VERIFIED: services/api/go.mod:11] |
| `github.com/jackc/pgx/v5` | v5.9.2 (already direct) | DB driver; `tx.Begin()` returns a child Tx backed by SAVEPOINT (D5-09); typed error mapping (23505, 23503, 23514) | Carry-forward from Phase 1. tx.Begin-on-Tx returns a pseudo-nested tx that is internally a SAVEPOINT [VERIFIED: pkg.go.dev/github.com/jackc/pgx/v5]. |
| `github.com/jonboulle/clockwork` | v0.4.0 (already indirect via Phase 4 — promote to direct) | Injectable Clock for sweep goroutine tests | Carry-forward from Phase 4 STATE-07. [VERIFIED: services/api/go.mod:61 — currently `// indirect`; Phase 5 elevates] |

### Supporting (already in go.mod)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sqlc` CLI (build tool) | v1.31.1 | Generate type-safe Go from SQL | Per-entity upsert queries + import_jobs queries [VERIFIED: 03-VERIFICATION.md line 30] |
| `oapi-codegen` (build tool) | v2.7.0 | Generate `Import*Request` types + updated `BulkImportCatalogRequestObject` after spec edits | Wave 0 spec edit + `task gen` re-runs [VERIFIED: services/api/go.mod:14] |
| `golang-migrate` (build tool) | v4.19.1 | Apply `000003_create_import_jobs.up.sql` | Wave 0 [VERIFIED: services/api/go.mod:11] |
| `github.com/redis/go-redis/v9` | v9.19.0 (already direct via Phase 3 cache) | Cache layer (Phase 3 `cache.Cache` reused — Phase 5 calls `cache.Del` post-commit per CAT-11) | Cache invalidation in per-entity upsert helpers [VERIFIED: services/api/go.mod:17] |
| `github.com/stretchr/testify` | v1.11.1 (test-only) | Integration test assertions | Inherited from Phase 1-4 |
| `github.com/testcontainers/testcontainers-go` + `modules/postgres` | v0.42.0 (test-only) | Real Postgres in tests | `testsupport.StartPostgres(t)` (`services/api/internal/testsupport/postgres.go:44`) |
| `github.com/alicebob/miniredis/v2` | v2.38.0 (test-only) | In-process Redis for non-isolation tests | Phase 3's cache_test.go pattern; Phase 5 can reuse for unit tests that need cache.Del-was-called assertions |

### Supporting (NEW — to install in Phase 5 Wave 0)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| *(none)* | — | All required libraries are already in go.mod. | Phase 5 introduces NO new direct go.mod dependencies. The only `// indirect` → direct promotion is `clockwork` (already used by Phase 4 transitively). |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `encoding/csv` | `github.com/go-csv/go-csv` or `github.com/jszwec/csvutil` | Third-party adds BOM-stripping convenience but introduces a new dependency and obscures the Pitfall 5.1 visibility. The Phase 4 STATE-07 hardening pattern is to be explicit about quirks at the call site, not bury them in a library. Stick with stdlib. |
| stdlib `encoding/json` streaming Decoder | `github.com/goccy/go-json` or `github.com/bytedance/sonic` | Performance-only. At 500-row / 50MB scale the stdlib decoder is more than fast enough. Adds dependency surface without benefit. Stick with stdlib. |
| Per-row SAVEPOINT inside one big tx | `COPY ... FROM STDIN` (Postgres-native bulk insert) | COPY is faster but doesn't support per-row partial-success or ON CONFLICT logic. D5-09 chunked-savepoint is the correct trade-off for the v0.1 contract. |
| `pgx.Tx.Begin()` for savepoint (pgx canonical) | Raw `tx.Exec(ctx, "SAVEPOINT row_N")` + `RELEASE` / `ROLLBACK TO` | pgx's `tx.Begin()` on a Tx returns a pseudo-nested Tx that internally uses SAVEPOINT [VERIFIED: pkg.go.dev/github.com/jackc/pgx/v5]. Use the typed API (cleaner, less footgun). However: the raw SAVEPOINT route gives slightly more control over the savepoint name (Postgres requires unique names within a tx). Recommendation: use `tx.Begin()` per pgx idiom; the auto-generated savepoint name is fine. |
| `oneOf` for request body schema | `items: {}` (current spec state) + handler dispatch on `?entity=` | oapi-codegen v2.7.0 does NOT generate `AsX/FromX` helpers for `oneOf` request bodies [VERIFIED: oapi-codegen issue #1620, 2024]. The `oneOf` route generates a `union json.RawMessage` field with NO accessor methods — completely unusable. Keep `items: {}` and have the handler dispatch by `?entity=`. The new `ImportAgentRequest`, `ImportSkillRequest`, ... etc schemas are still authored in `openapi.yaml` and consumed BY the handler (the handler `json.Unmarshal`s each row into the right typed struct after eager-decode places `BulkImportCatalogJSONRequestBody` = `[]interface{}` in JSONBody). |

**Installation:**
```bash
# No new go.mod direct deps.
# Promote clockwork from indirect → direct:
cd services/api && go get github.com/jonboulle/clockwork@v0.4.0
```

**Version verification:**
```bash
go list -m github.com/jackc/pgx/v5         # v5.9.2 — current, no upgrade needed
go list -m github.com/jonboulle/clockwork  # v0.4.0 (indirect) — Phase 4 carry-forward
go list -m github.com/google/uuid          # v1.6.0 — current
```

## Package Legitimacy Audit

Phase 5 installs NO new external Go modules. Every library used is
either already in `services/api/go.mod` (verified in tree) or a Go
standard-library package (compiled into the toolchain). The only
"new" introduction is promoting `clockwork` from `// indirect` to
direct require — the package is already in `go.sum` from Phase 4 and
was audited by Phase 4 RESEARCH (lines 119-124: "github.com/jonboulle/clockwork
— active since 2014, MIT-licensed, used by Kubernetes, Docker,
Terraform, Prometheus").

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| (none) | — | — | — | — | — | NO new packages |

**Packages removed due to slopcheck [SLOP] verdict:** none (none requested).
**Packages flagged as suspicious [SUS]:** none.

*slopcheck was not run in this research session because Phase 5 does
not install any new packages. The `clockwork` promotion was already
audited in Phase 4 RESEARCH (line 124 explicitly recorded a
`checkpoint:human-verify` task before the original Phase 4 install).*

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
│  1. Recoverer                                                        │
│  2. RequestID (UUIDv7)                                               │
│  3. orgContextMiddleware  → /v1/* requires X-Org-Id                  │
│  4. uuidv7PathParamsMiddleware → no-op (no {id} on /import)          │
│  5. NEW: bodyLimitMiddleware (50<<20) → wraps r.Body for THIS path   │
│     (path-prefix matches /v1/orgs/{org_id}/catalog/import)           │
│       (a) Content-Length pre-flight → 413 if > 50MB                  │
│       (b) http.MaxBytesReader wrap                                   │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│ Generated strict-server wrapper (server.gen.go:5624)                 │
│  • Reads ?entity= and ?schema_version= → BulkImportCatalogParams     │
│  • If Content-Type starts with "application/json":                   │
│       eager json.NewDecoder(r.Body).Decode(&body) → JSONBody         │
│       (memory-bounded by MaxBytesReader; OOM-safe)                   │
│  • If Content-Type starts with "text/csv":                           │
│       request.Body = r.Body (still MaxBytes-wrapped, not consumed)   │
│  • Else: no JSONBody, no Body → handler returns 415                  │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│ imports.Handlers.BulkImportCatalog(ctx, req) (strict handler)        │
│   STEP 1: Validate ?entity= ∈ {6 valid types}; else 400.             │
│   STEP 2: If CSV path, validate ?schema_version=v0.1; else 400.      │
│   STEP 3: If Idempotency-Key present, lookup (org_id, key) →         │
│           hit → rehydrate BulkImportResult, set                      │
│                  idempotent_replay=true, return early                │
│           miss → proceed                                             │
│   STEP 4: Parse + validate header (CSV only) OR coerce array         │
│           (JSON). Header validation is BATCH-LEVEL strict — missing  │
│           required column or unknown column → 400.                   │
│   STEP 5: Mint import_jobs.id (uuid.NewV7); INSERT job row           │
│           status='pending' in its OWN short tx.                      │
│   STEP 6: Read rows in streaming fashion:                            │
│           • Per-row count ≤ 500 (D5-22; exit at 501 with 413)        │
│           • Per-row coercion pipeline (D5-01 trim→lower→split→parse) │
│           • Per-row Layer-2 validation                               │
│           • Buffer rows into chunks of 50 (D5-09)                    │
│   STEP 7: For each chunk:                                            │
│           • orgDB.BeginTx (real pgx Tx)                              │
│           • For each row i: tx.Begin(ctx) (child Tx = SAVEPOINT)     │
│             ├─ resolve FK externals via batch query                  │
│             ├─ Upsert{Entity}ByExternalId(qtx, ...)                  │
│             ├─ If agent: also INSERT agent_states                    │
│             │   (Pitfall 6 / Hazard 7 ON CONFLICT DO NOTHING)        │
│             ├─ If agent: MERGE agent_skills (D5-18 PATCH-like)       │
│             ├─ Success → savepointTx.Commit() (RELEASE)              │
│             ├─ Failure → savepointTx.Rollback() (ROLLBACK TO)        │
│             │     record {row,field,error,reason} in failed[]        │
│           • tx.Commit() (whole chunk)                                │
│           • Per-entity cache.Del for each succeeded row              │
│   STEP 8: UPDATE import_jobs row → status=completed/failed,          │
│           succeeded_rows, failed_rows, errors JSONB.                 │
│   STEP 9: Marshal BulkImportResult{succeeded[], failed[]} → 200/207/422│
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────────┐
│ PostgreSQL: catalog tables + import_jobs                             │
│  • UNIQUE (org_id, external_id) on each catalog entity               │
│  • UNIQUE (org_id, idempotency_key) WHERE idempotency_key IS NOT NULL│
│  • ix_import_jobs_pending_updated for crash sweep                    │
└──────────────────────────────────────────────────────────────────────┘

       ┌──────────────────────────────────────────────────────────────┐
       │ Background goroutine (cmd/api/main.go)                       │
       │  • clockwork.NewRealClock().NewTicker(1h)                    │
       │  • Bypass-marked ctx (db.WithBypass("import_crash_sweep"))   │
       │  • UPDATE import_jobs SET status='failed' WHERE ...          │
       │    status='pending' AND updated_at < now() - 24h             │
       │  • Mirror state.Server.Start/Stop lifecycle (WaitGroup, ctx) │
       └──────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
services/api/
├── cmd/api/
│   └── main.go                          # MODIFIED: add *imports.Handlers to ApiHandlers composite + Start/Stop calls
├── internal/
│   ├── api/                              # MODIFIED: regen after openapi.yaml edits (task gen)
│   ├── catalog/
│   │   └── notimpl.go                    # DELETED: real handlers ship in internal/imports/
│   ├── imports/                          # NEW PACKAGE
│   │   ├── doc.go                        # Package overview + D5-* invariant index
│   │   ├── handlers.go                   # Handlers struct + Deps + New + Start/Stop lifecycle
│   │   ├── handler_import.go             # BulkImportCatalog method (the big one)
│   │   ├── handler_get_job.go            # GetImportJob method
│   │   ├── coerce.go                     # D5-01 typed pipeline: TrimToLower / ParseBool / ParseInt / SplitMulti
│   │   ├── parser_json.go                # streaming JSON token-mode loop
│   │   ├── parser_csv.go                 # BOM strip + utf8.Valid + csv.Reader loop + header validation
│   │   ├── header.go                     # Per-entity column registry: required + optional + typed dispatch
│   │   ├── row_agent.go                  # Agent row processor: coerce → validate → resolveSkills → upsert
│   │   ├── row_skill.go                  # Skill row processor
│   │   ├── row_queue.go                  # Queue row processor
│   │   ├── row_channel.go                # Channel row processor (default_queue_id FK by external_id)
│   │   ├── row_adapter.go                # Adapter row processor (JSONB config passthrough)
│   │   ├── row_break_reason.go           # BreakReason row processor
│   │   ├── chunk.go                      # Chunked-savepoint orchestration: BeginTx + tx.Begin per row
│   │   ├── jobs.go                       # import_jobs INSERT/UPDATE/Get/Replay
│   │   ├── idempotency.go                # Idempotency-Key lookup + rehydration
│   │   ├── sweep.go                      # Crash-recovery goroutine (mirror state.Server pattern)
│   │   ├── errors.go                     # Sentinel errors + reason mapping
│   │   ├── testdata/
│   │   │   ├── agents-basic.json
│   │   │   ├── agents-basic.csv
│   │   │   ├── agents-with-skills.csv
│   │   │   ├── agents-with-skills.json
│   │   │   ├── windows-excel-agents.csv      # UTF-8 BOM + CRLF (Pitfall 5.1)
│   │   │   ├── break_reasons-basic.csv
│   │   │   ├── full-failure.csv              # 422 all-fail case
│   │   │   ├── partial-success.csv           # 207 case
│   │   │   ├── oversized.csv                 # 501 rows
│   │   │   └── invalid-utf8.csv              # 400 csv_not_utf8 case
│   │   ├── coerce_test.go                # Per-pipeline unit tests
│   │   ├── parser_csv_test.go            # BOM/CRLF/embedded-comma/quoted-newline coverage
│   │   ├── parser_json_test.go           # Streaming + recovery + malformed test cases
│   │   ├── header_test.go                # missing_columns / unknown_columns
│   │   ├── row_agent_test.go             # ... per entity
│   │   ├── chunk_test.go                 # SAVEPOINT roll-forward / roll-back behaviour
│   │   ├── jobs_test.go                  # import_jobs lifecycle
│   │   ├── idempotency_test.go           # replay hit/miss
│   │   ├── sweep_test.go                 # clockwork-backed crash sweep
│   │   ├── handlers_test.go              # End-to-end handler tests (one per entity × per format)
│   │   └── testutil_test.go              # Shared httptest server + seed helpers
│   ├── middleware/
│   │   ├── bodylimit.go                  # NEW: BodyLimit(maxBytes) chi MiddlewareFunc
│   │   └── bodylimit_test.go             # NEW: Content-Length pre-flight + mid-stream MaxBytesError test
│   ├── db/queries/
│   │   ├── agents.sql                    # MODIFIED: add UpsertAgentByExternalId :one
│   │   ├── skills.sql                    # MODIFIED: add UpsertSkillByExternalId :one + ResolveSkillExternalIds :many
│   │   ├── queues.sql                    # MODIFIED: add UpsertQueueByExternalId :one
│   │   ├── channels.sql                  # MODIFIED: add UpsertChannelByExternalId :one
│   │   ├── adapters.sql                  # MODIFIED: add UpsertAdapterByName :one  (adapters have no external_id)
│   │   ├── break_reasons.sql             # MODIFIED: add UpsertBreakReasonByName :one  (break_reasons have no external_id)
│   │   ├── agent_skills.sql              # MODIFIED: add MergeAgentSkill :exec (D5-18 ON CONFLICT DO UPDATE)
│   │   └── import_jobs.sql               # NEW: InsertImportJob, FinaliseImportJob, GetImportJob, LookupByIdempotencyKey, ListCrashSweepCandidates
│   ├── db/sqlcheck.go                    # MODIFIED: add "import_jobs": {} to tenantTables
│   └── test/isolation/
│       └── imports_test.go               # NEW: cross-org probes (orgA POST → orgB GET → 404)
├── openapi/openapi.yaml                  # MODIFIED: 6 new Import*Request + ImportJob.status + Idempotency-Key header + idempotent_replay
└── migrations/
    └── 000003_create_import_jobs.up.sql  # NEW
    └── 000003_create_import_jobs.down.sql # NEW
```

### Pattern 1: Strict-server handler dispatch on `?entity=` + Content-Type (D5-14, D5-24)

**What:** The handler receives a `BulkImportCatalogRequestObject` carrying
`Params.Entity`, optional `Params.SchemaVersion`, `JSONBody`
(eager-decoded `[]interface{}` for `application/json`), and `Body`
(`io.Reader` for `text/csv`). Handler picks parser, picks per-entity row
processor, returns one of 6 `BulkImportCatalog{200,207,400,413,422,500}JSONResponse`
types.

**When to use:** Single entry point for IMP-01.

**Example:**
```go
// Source: services/api/internal/catalog/agents.go:71 (Phase 3 pattern,
//         adapted for streaming + multiple entity types)
// Source: services/api/internal/api/server.gen.go:3640 (request shape)
func (h *Handlers) BulkImportCatalog(ctx context.Context, req api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
            Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
        }}, nil
    }

    // STEP 1: Validate ?entity= (already a typed enum from oapi-codegen)
    entity := req.Params.Entity
    if !isSupportedEntity(entity) {
        return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
            Error: api.ErrorCodeInvalidBody, Reason: "unsupported_entity",
        }), nil
    }

    // STEP 2: Dispatch parser by which body field is populated.
    // The strict-server wrapper (server.gen.go:5629-5641) sets exactly
    // one of req.JSONBody (application/json) or req.Body (text/csv).
    // Missing/unrecognised Content-Type → both nil → 415 from strict layer.
    switch {
    case req.JSONBody != nil:
        return h.importJSON(ctx, orgID, entity, *req.JSONBody, req)
    case req.Body != nil:
        // STEP 2a: CSV requires ?schema_version=v0.1 per IMP-08
        if req.Params.SchemaVersion == nil || *req.Params.SchemaVersion != "v0.1" {
            return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
                Error: api.ErrorCodeInvalidBody,
                Reason: "unsupported_schema_version_supported_versions=v0.1",
            }), nil
        }
        return h.importCSV(ctx, orgID, entity, req.Body, req)
    default:
        return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
            Error: api.ErrorCodeInvalidBody, Reason: "unsupported_content_type",
        }), nil
    }
}
```

### Pattern 2: Body-size enforcement middleware (D5-21)

**What:** Chi MiddlewareFunc that (a) pre-flights `Content-Length`
header → 413 if > 50 MB; (b) wraps `r.Body` in `http.MaxBytesReader`
so mid-stream over-limit also fails. Path-scoped to the import
endpoint so other write endpoints aren't affected.

**When to use:** Wave 1; reusable for any future endpoint that needs body-size limits.

**Example:**
```go
// Source: pkg.go.dev/net/http#MaxBytesReader [CITED: pkg.go.dev/net/http]
// services/api/internal/middleware/bodylimit.go (NEW)
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
//
// Reusable: future write endpoints (e.g., adapter config upload) can
// mount this at their own path prefix.
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

// In handler code (handler_import.go), when parsing fails with
// *http.MaxBytesError → translate to 413.
//   var maxBytesErr *http.MaxBytesError
//   if errors.As(decodeErr, &maxBytesErr) { return 413 }
```

### Pattern 3: BOM strip + UTF-8 validation + csv.Reader streaming (IMP-02, D5-08)

**What:** Read first 3 bytes via `bufio.Reader.Peek`. If they are
`{0xEF, 0xBB, 0xBF}` → consume them. Validate remainder is UTF-8 by
streaming validation per-line OR — simpler — buffer ≤ 50 MB (already
bounded by MaxBytesReader) and validate with `utf8.Valid(buf)`. Then
hand the BOM-stripped reader to `csv.Reader`. csv.Reader auto-converts
CRLF → LF in its output [VERIFIED: pkg.go.dev/encoding/csv].

**When to use:** CSV parsing entry point only.

**Example:**
```go
// Source: pkg.go.dev/encoding/csv (CRLF behaviour) +
//         golang/go issue #9588 (BOM not stripped) [CITED]
// services/api/internal/imports/parser_csv.go (NEW)
package imports

import (
    "bufio"
    "encoding/csv"
    "errors"
    "io"
    "unicode/utf8"
)

// utf8BOM is the 3-byte UTF-8 byte-order mark Excel prepends to "CSV UTF-8".
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripBOM wraps r in a bufio.Reader, peeks the first 3 bytes, and
// consumes them iff they match the UTF-8 BOM. Returns the bufio.Reader
// so the caller can chain into csv.NewReader. The peek is non-destructive
// when no BOM is present (Peek does not advance the read pointer per
// pkg.go.dev/bufio#Reader.Peek). [CITED]
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
//   • LazyQuotes=false  (strict RFC 4180 per D5-04 documentation)
//   • FieldsPerRecord=0 (csv.Reader latches to first record's column
//     count; mismatched data rows error → handler turns into per-row
//     failure with reason: column_count_mismatch)
//   • TrimLeadingSpace=false  (D5-01 trim happens in coercion pipeline,
//     NOT in CSV reader, so quoted-empty-cell "" remains distinguishable
//     from "   ")
// csv.Reader auto-converts CRLF → LF on output per
// pkg.go.dev/encoding/csv: "The Reader converts all \r\n sequences in
// its input to plain \n". [CITED]
func newCSVReader(r io.Reader) *csv.Reader {
    cr := csv.NewReader(r)
    cr.LazyQuotes = false
    cr.FieldsPerRecord = 0
    cr.TrimLeadingSpace = false
    return cr
}

// readCSVHeader reads the first record (the header row) and validates
// it against the per-entity column registry. D5-05 strict batch-level:
// missing required → 400; unknown → 400; no rows processed.
func readCSVHeader(cr *csv.Reader, registry entityColumnRegistry) (mapping headerMapping, err error) {
    raw, rErr := cr.Read()
    if errors.Is(rErr, io.EOF) {
        return nil, errEmptyCSV
    }
    if rErr != nil {
        return nil, rErr
    }
    return registry.validateHeader(raw)
}
```

```go
// services/api/internal/imports/handler_import.go — CSV body path
func (h *Handlers) importCSV(ctx context.Context, orgID uuid.UUID, entity api.ImportEntityType, body io.Reader, req api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
    // STEP 1: For UTF-8 validation we either (a) buffer body fully and call
    //         utf8.Valid, or (b) validate per-line. Given body is already
    //         50MB-capped by middleware, (a) is the simpler+correct path.
    // [VERIFIED: pkg.go.dev/unicode/utf8#Valid]
    raw, err := io.ReadAll(body)
    if err != nil {
        var maxBytesErr *http.MaxBytesError
        if errors.As(err, &maxBytesErr) {
            return api.BulkImportCatalog413JSONResponse{
                RequestEntityTooLargeJSONResponse: api.RequestEntityTooLargeJSONResponse{
                    Error: api.ErrorCodeInvalidBody,
                    Reason: "request_too_large_use_async_pathway",
                },
            }, nil
        }
        h.deps.Logger.ErrorContext(ctx, "import csv read body", "err", err)
        return api.BulkImportCatalog500JSONResponse{InternalServerErrorJSONResponse: ...}, nil
    }
    stripped := raw
    if len(raw) >= 3 && bytes.HasPrefix(raw, utf8BOM) {
        stripped = raw[3:]
    }
    if !utf8.Valid(stripped) {
        return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
            Error: api.ErrorCodeInvalidBody, Reason: "csv_not_utf8",
        }), nil
    }

    cr := newCSVReader(bytes.NewReader(stripped))
    // ... header validation, then streaming row loop with 501 check
}
```

### Pattern 4: Streaming JSON token-mode decoder (D5-23) with v0.1 caveat

**What:** Standard Go stdlib pattern for streaming-decode of a top-level
JSON array. Per-row decode errors that are `*json.UnmarshalTypeError`
tear the stream per pkg.go.dev — the doc is explicit: "Token guarantees
that the delimiters [ ] { } it returns are properly nested and matched:
if Token encounters an unexpected delimiter in the input, it will
return an error." [CITED: pkg.go.dev/encoding/json#Decoder.Token].

**⚠️ ARCHITECTURE TENSION:** The generated strict-server wrapper at
`server.gen.go:5629-5641` EAGERLY decodes the JSON body upfront into
`BulkImportCatalogJSONRequestBody = []interface{}` BEFORE the handler
runs. This is incompatible with D5-23's streaming spirit. The
recommended v0.1 path:

1. Accept the eager decode. Memory is bounded by D5-21 MaxBytesReader.
   50 MB raw JSON → ≈ 200-300 MB live `[]interface{}` representation.
   Acceptable for v0.1 catalog scale (the 500-row cap and per-row
   shape mean the per-row interface{} fan-out is small).
2. After the eager decode lands, iterate `*req.JSONBody` (a `[]interface{}`).
   For each item, call `json.Marshal(item)` then `json.Unmarshal(bytes,
   &typed)` into the right per-entity struct. Per-row errors are now
   isolated by design (each item is its own subdocument).
3. The 500-row cap (D5-22) is enforced inline by iterating and breaking
   at index 500 with 413.

**Why this works:** The streaming benefit was "per-row error recovery."
Once the body is in `[]interface{}`, per-row recovery is trivial via
the re-marshal/unmarshal trick. The memory benefit was "O(1 row)"; for
v0.1's 50MB cap this is sacrificed but bounded.

**Future v0.2 fix:** customize `oapi-codegen.server.yaml` with a custom
template that skips body decode for this operation, or split the
`BulkImportCatalog` operation into per-entity endpoints so each has a
typed body shape. Out of v0.1 scope.

**When to use:** JSON body path of `importJSON()`.

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
// Per-row decode errors are isolated to the row — the rest of the
// batch is unaffected.
//
// Generic over the target type so each row-entity processor calls
// reMarshalAs[api.ImportAgentRequest](rawItem).
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
func (h *Handlers) importJSON(ctx context.Context, orgID uuid.UUID, entity api.ImportEntityType, body api.BulkImportCatalogJSONRequestBody, req api.BulkImportCatalogRequestObject) (api.BulkImportCatalogResponseObject, error) {
    if len(body) > 500 {  // D5-22 hard cap
        return api.BulkImportCatalog413JSONResponse{...}, nil
    }
    succeeded, failed, err := h.processRows(ctx, orgID, entity, len(body), func(idx int) (any, error) {
        return body[idx], nil   // each iteration yields one raw row
    })
    // ... finalise job, build response
}
```

### Pattern 5: Chunked-savepoint orchestration (D5-09)

**What:** For each chunk of ≤50 rows: open one outer `OrgTx`. For each
row in chunk: call `outerTx.Begin(ctx)` which pgx v5 implements as
SAVEPOINT (pseudo-nested tx) [VERIFIED: pkg.go.dev/github.com/jackc/pgx/v5
— "Tx returned from Conn.Begin also implements the Tx.Begin method.
This can be used to implement pseudo nested transactions. These are
internally implemented with savepoints."]. Try the per-row upsert; on
success `savepointTx.Commit()` (= RELEASE); on failure
`savepointTx.Rollback()` (= ROLLBACK TO). Commit the outer Tx after
all 50 row attempts complete.

**Critical pgx gotcha:** `OrgTx` (Phase 1 wrapper) does NOT expose a
`Begin()` method that returns a child OrgTx. It wraps `pgx.Tx` but its
public surface is `Exec / Query / QueryRow / Commit / Rollback`. Phase 5
must either:

1. **Extend `OrgTx` with a `BeginSavepoint(ctx) (*OrgTx, error)` method**
   that calls the underlying `pgx.Tx.Begin(ctx)` and returns a wrapped
   child OrgTx (preserving the SQLChecker preflight on the savepoint's
   Exec calls). **Recommended** — preserves Phase 1 D-02 invariant.
2. Drop down to raw `tx.Exec(ctx, "SAVEPOINT row_N")` + `RELEASE` /
   `ROLLBACK TO` strings, bypassing the wrapper. **Rejected** — would
   bypass SQLChecker on the SAVEPOINT statement, although those
   statements don't reference any table (so SQLChecker accepts) — but
   it's fragile.

**When to use:** Per-chunk in the row processor.

**Example:**
```go
// Source: pgx v5 docs [CITED: pkg.go.dev/github.com/jackc/pgx/v5]
// Source: services/api/internal/db/orgdb.go:223 (BeginTx pattern)
// services/api/internal/db/orgdb.go — NEW METHOD on *OrgTx
//
// BeginSavepoint starts a savepoint on the current tx and returns a new
// *OrgTx wrapping the child. SQLChecker preflight is preserved.
//
// pgx idiom: "The Tx returned from Conn.Begin also implements the
// Tx.Begin method. ... internally implemented with savepoints."
func (t *OrgTx) BeginSavepoint(ctx context.Context) (*OrgTx, error) {
    child, err := t.tx.Begin(ctx)
    if err != nil {
        return nil, fmt.Errorf("orgtx: begin savepoint: %w", err)
    }
    return &OrgTx{tx: child, checker: t.checker, mode: t.mode}, nil
}
```

```go
// services/api/internal/imports/chunk.go (NEW)
func (h *Handlers) processChunk(ctx context.Context, orgID uuid.UUID, rowProc rowProcessor, rows []parsedRow) (succeeded []uuid.UUID, failed []api.BulkImportFailedRow) {
    outerTx, err := h.deps.OrgDB.BeginTx(ctx)
    if err != nil {
        // Infrastructure failure — all rows in chunk fail with the same reason.
        for _, r := range rows {
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "tx_begin_failed",
            })
        }
        return
    }
    defer func() { _ = outerTx.Rollback(ctx) }()

    for _, r := range rows {
        sp, spErr := outerTx.BeginSavepoint(ctx)
        if spErr != nil {
            failed = append(failed, api.BulkImportFailedRow{
                Row: r.lineNo, Error: api.BulkImportFailedRowErrorImportFailed,
                Reason: "savepoint_begin_failed",
            })
            continue
        }
        id, procErr := rowProc.process(ctx, sp, orgID, r)
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
        succeeded = append(succeeded, id)
    }

    if err := outerTx.Commit(ctx); err != nil {
        // Chunk commit failure — all "succeeded" rows in this chunk are
        // actually lost. Move them to failed with reason: chunk_commit_failed.
        for _, id := range succeeded {
            failed = append(failed, ...)
        }
        return nil, failed
    }
    return succeeded, failed
}
```

### Pattern 6: Per-entity upsert SQL — Phase 3 GAP that Phase 5 MUST fill (IMP-03)

**What:** Per-entity `Upsert{Entity}ByExternalId` sqlc query that
INSERTs new rows OR UPDATEs by `(org_id, external_id)`. Returns the
final row (id + version). The version increments on update via
explicit assignment (NOT COALESCE — import is authoritative for
included columns per D5-15).

**When to use:** Wave 1 — Phase 5 authors these queries before any handler code.

**Example:**
```sql
-- services/api/internal/db/queries/agents.sql — APPEND
--
-- name: UpsertAgentByExternalId :one
-- IMP-03: per-row upsert keyed by (org_id, external_id).
-- New row: defaults apply (enabled=TRUE, version=1).
-- Existing row: UPDATE all import-mentioned columns, bump version.
-- Returns the final row regardless of insert-vs-update so the handler
-- can record id in `succeeded[]`.
--
-- IMPORTANT: import semantics differ from PATCH:
-- import REPLACES every named column (D5-15) — there is no sparse-
-- PATCH COALESCE pattern. If the import row says enabled=false, the
-- existing enabled becomes false. The "empty cell = leave unchanged"
-- semantics (D5-03) is enforced at the HANDLER level by passing nil
-- for unchanged columns and having COALESCE preserve them for UPDATEs
-- of EXISTING rows. (Insert path uses the row defaults from the spec
-- for nil columns — see Phase 5's `mapAgentImportToParams` helper.)
INSERT INTO agents (id, org_id, external_id, name, email, enabled)
VALUES (sqlc.arg('id'), sqlc.arg('org_id'),
        sqlc.arg('external_id'), sqlc.arg('name'),
        sqlc.arg('email'),
        COALESCE(sqlc.narg('enabled')::bool, TRUE))
ON CONFLICT (org_id, external_id) DO UPDATE
SET name       = COALESCE(sqlc.narg('name')::text,    agents.name),
    email      = COALESCE(sqlc.narg('email')::text,   agents.email),
    enabled    = COALESCE(sqlc.narg('enabled')::bool, agents.enabled),
    version    = agents.version + 1,
    updated_at = NOW()
RETURNING id, org_id, external_id, name, email, enabled, version, created_at, updated_at;
```

```sql
-- services/api/internal/db/queries/skills.sql — APPEND
--
-- name: UpsertSkillByExternalId :one
INSERT INTO skills (id, org_id, external_id, name, description, skill_type, enabled)
VALUES (sqlc.arg('id'), sqlc.arg('org_id'),
        sqlc.arg('external_id'), sqlc.arg('name'),
        sqlc.narg('description'), sqlc.arg('skill_type'),
        COALESCE(sqlc.narg('enabled')::bool, TRUE))
ON CONFLICT (org_id, external_id) DO UPDATE
SET name        = COALESCE(sqlc.narg('name')::text,        skills.name),
    description = COALESCE(sqlc.narg('description')::text, skills.description),
    skill_type  = COALESCE(sqlc.narg('skill_type')::text,  skills.skill_type),
    enabled     = COALESCE(sqlc.narg('enabled')::bool,     skills.enabled),
    version     = skills.version + 1,
    updated_at  = NOW()
RETURNING id, org_id, external_id, name, description, skill_type, enabled, version, created_at, updated_at;

-- name: ResolveSkillExternalIds :many
-- D5-16 + D5-17: batch lookup of skill UUIDs by external_id. Used by the
-- agent row processor to resolve nested skill references once per chunk.
-- Returns id + external_id pairs. Missing external_ids are absent from
-- the result; handler distinguishes "unknown skill" by set difference.
SELECT id, external_id
FROM skills
WHERE org_id = $1 AND external_id = ANY($2::text[]);
```

```sql
-- services/api/internal/db/queries/agent_skills.sql — APPEND
--
-- name: MergeAgentSkill :exec
-- D5-18 + D5-19: skill assignment MERGE on import update. ON CONFLICT
-- DO UPDATE so existing assignments get the new proficiency from the
-- import (D5-19 import wins). Existing skills NOT in this payload are
-- LEFT INTACT (D5-18 PATCH-like) — this is achieved by simply NOT
-- issuing DELETE statements for omitted skills.
INSERT INTO agent_skills (agent_id, skill_id, org_id, proficiency)
VALUES ($1, $2, $3, $4)
ON CONFLICT (agent_id, skill_id) DO UPDATE
SET proficiency = EXCLUDED.proficiency;
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
-- D5-13 idempotent replay: lookup by (org_id, idempotency_key). Used
-- BEFORE any import work to short-circuit duplicate POSTs.
SELECT id, org_id, entity_type, status, total_rows, succeeded_rows,
       failed_rows, errors, idempotency_key, created_at, updated_at
FROM import_jobs
WHERE org_id = $1 AND idempotency_key = $2;

-- name: SweepCrashedImportJobs :many
-- D5-11 crash-recovery sweep: flip pending → failed for jobs > 24h old.
-- RETURNING so the goroutine can emit slog.Warn per row.
UPDATE import_jobs
SET status = 'failed',
    errors = '[{"row":0,"error":"import_failed","reason":"server_crash"}]'::jsonb,
    updated_at = NOW()
WHERE status = 'pending'
  AND updated_at < NOW() - INTERVAL '24 hours'
  AND org_id = $1
RETURNING id, org_id;
-- NOTE: org_id = $1 is REQUIRED for SQLChecker (D-02). The sweep
-- goroutine loops over all known orgs (queried separately) OR uses
-- db.WithBypass(ctx, "import_crash_sweep") + a separate non-tenant
-- query — planner picks. Recommend bypass to keep the sweep
-- org-agnostic (mirror state.Server.expireWrapUp pattern in ttl.go:92).
-- In that case the SQL drops the org_id filter; the bypass marker
-- exempts it from SQLChecker.
```

### Pattern 7: import_jobs migration (IMP-06)

**What:** New `000003_create_import_jobs.up.sql` (and `.down.sql`).
Table has org_id (for SQLChecker), entity_type CHECK, status CHECK,
counters, errors JSONB, idempotency_key NULL with partial unique
index, timestamps.

**Example:**
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

### Pattern 8: Crash-recovery goroutine (D5-11, mirroring Phase 4 STATE-07)

**What:** A `time.Ticker`-based goroutine that periodically flips
`status='pending'` rows older than 24h to `status='failed'`. The
exact pattern is Phase 4's `state.Server.safetySweep` in
`services/api/internal/state/ttl.go:160`. Phase 5's
`imports.Handlers` has the same Start/Stop lifecycle (handler.go:97-118)
and runs alongside `state.Server` in `cmd/api/main.go`.

**Critical reuse points from Phase 4 (must mirror exactly):**

- `clockwork.Clock` injection via `WithClock` option (`state/handlers.go:40`)
- `context.WithCancel` for graceful shutdown (`state/handlers.go:104`)
- `sync.WaitGroup` for in-flight ticks (`state/handlers.go:114`)
- `db.WithBypass(ctx, "import_crash_sweep")` so SQLChecker accepts
  the org-agnostic query (`state/ttl.go:179` — `db.WithBypass(ctx,
  "wrapup_sweeper.safety")`)
- `slog.Warn` per transition with `event=import_crash_sweep` (D5-11)
- Single-replica only; STATE.md blocker flagged
- Startup behaviour: Phase 4 does a SYNCHRONOUS startup sweep before
  spawning the ticker (D-95). **Phase 5 should do the same.** Reason:
  a crash during import leaves a pending row that should flip to
  failed IMMEDIATELY on next process boot, not 1 hour later.

**Example:**
```go
// services/api/internal/imports/sweep.go (NEW — mirror state/ttl.go)
package imports

import (
    "context"
    "errors"
    "fmt"
    "time"
    // ... imports
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

    // Either iterate per-org with a known list, OR use a bypass-marked
    // org-agnostic UPDATE. Phase 4 chose org-aware (`ExpireWrapUp` has
    // org_id in WHERE because state rows are denormalized). Phase 5
    // SHOULD do the same — the import_jobs table carries org_id
    // already and the UPDATE references it.
    //
    // The challenge: a single sweep iteration should cover ALL orgs.
    // Two approaches:
    //
    //  (a) Use db.WithBypass + drop org_id from WHERE. The bypass
    //      marker exempts SQLChecker from the org_id-present
    //      requirement (Phase 1 D-04). Mirror of ttl.go:179 pattern.
    //  (b) Iterate every org and run the per-org sweep. Cleaner DB
    //      semantics but requires an org-list query (no source for
    //      this in v0.1 — no "orgs" table; orgs are implicit from
    //      data presence).
    //
    // RECOMMENDED: (a). Add a new sqlc query:
    //   -- name: SweepAllCrashedImportJobs :many
    //   UPDATE import_jobs SET status='failed', errors='...', updated_at=NOW()
    //   WHERE status='pending' AND updated_at < NOW() - INTERVAL '24 hours'
    //   RETURNING id, org_id;
    //
    // SQLChecker will reject this without bypass. db.WithBypass(ctx,
    // "import_crash_sweep") sets the marker; orgdb.go:142 emits the
    // structured audit slog event for the bypass.
    q := generated.New(s.deps.OrgDB)
    rows, err := q.SweepAllCrashedImportJobs(ctx)
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

### Pattern 9: Idempotency-Key extraction + replay (D5-13, D5-27)

**What:** Strict-server doesn't auto-extract custom headers into the
`RequestObject`. Phase 5 must either:

1. **Add `Idempotency-Key` as a header parameter in `openapi.yaml`**
   per D5-27 → oapi-codegen generates a typed field in
   `BulkImportCatalogParams`. **Recommended.**
2. Read the header from `r.Header.Get("Idempotency-Key")` via the
   chi context. Possible but more fragile.

**When to use:** Step 3 of the handler (BEFORE any work).

**Example:**
```yaml
# openapi.yaml additions (Wave 0)
parameters:
  IdempotencyKeyHeader:
    name: Idempotency-Key
    in: header
    required: false
    schema:
      type: string
      format: uuid
      description: Client-generated UUIDv7 for retry-safe POST. See https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/
```

```go
// In handler:
var idempotencyKey *string
if req.Params.IdempotencyKey != nil {
    s := req.Params.IdempotencyKey.String()  // header is typed as openapi UUID
    idempotencyKey = &s
}

if idempotencyKey != nil {
    q := generated.New(h.deps.OrgDB)
    prior, err := q.LookupImportJobByIdempotencyKey(ctx, generated.LookupImportJobByIdempotencyKeyParams{
        OrgID:           pgUUID(orgID),
        IdempotencyKey:  *idempotencyKey,
    })
    if err == nil {
        // Replay hit: rehydrate response from persisted row.
        return rehydrateBulkImportResult(prior, true), nil
    }
    if !errors.Is(err, pgx.ErrNoRows) {
        h.deps.Logger.ErrorContext(ctx, "idempotency lookup", "err", err)
        return api.BulkImportCatalog500JSONResponse{...}, nil
    }
    // Miss → fall through.
}
```

**Replay reconstruction (D5-13 KNOWN LIMITATION):**
```go
// rehydrateBulkImportResult builds a BulkImportResult from a persisted
// import_jobs row. The succeeded[] UUID list CANNOT be reconstructed
// (we only stored counters); v0.1 returns succeeded=[] + sets
// idempotent_replay=true so callers detect the replay shape.
//
// v0.2 will add a succeeded_ids JSONB column to enable full replay.
func rehydrateBulkImportResult(row generated.ImportJob, replay bool) api.BulkImportCatalog200JSONResponse {
    var failed []api.BulkImportFailedRow
    if row.Errors.Valid {
        _ = json.Unmarshal(row.Errors.Bytes, &failed)
    }
    return api.BulkImportCatalog200JSONResponse(api.BulkImportResult{
        Succeeded:       []api.UUIDv7{}, // KNOWN LIMITATION
        Failed:          failed,
        // IdempotentReplay set via the new optional field on BulkImportResult (D5-27)
        IdempotentReplay: &replay,
    })
}
```

### Anti-Patterns to Avoid

- **DO NOT call `generated.New(h.deps.OrgDB)` inside a chunk loop** —
  every per-row query inside a chunk MUST go through the outer
  `OrgTx`'s child savepoint Tx (`generated.New(savepointTx)`). Otherwise
  the per-row UPSERT commits outside the chunk tx and the rollback-to-savepoint
  semantics are broken. Mirror `catalog/agents.go:113` (qtx := generated.New(tx)).
- **DO NOT issue per-row skill external_id lookups** — that's N+1. Issue
  ONE `ResolveSkillExternalIds(ctx, orgID, allSkillCodes)` query per
  chunk at the start, build a `map[string]uuid.UUID`, and re-use across
  all 50 rows.
- **DO NOT use COPY ... FROM STDIN** for the bulk insert. COPY doesn't
  support `ON CONFLICT` or per-row partial-success semantics.
- **DO NOT auto-detect charset** (e.g., chardet or similar). D5-08
  locks UTF-8 only; Pitfall 5.1 explicitly flags charset auto-detect
  as the #1 silent-corruption source.
- **DO NOT silently drop unknown CSV columns.** D5-05 strict policy:
  unknown column → 400 batch-level, NO rows processed. Pitfall 5.4.
- **DO NOT echo raw row content in `BulkImportFailedRow.reason`** — the
  Phase 2 OQ-2A resolution forbids this (deferred to v0.2 IMP-11). The
  `field` + `row` + canonical `reason` strings are enough.
- **DO NOT call `cache.Del` per-row inside the chunk loop.** Phase 3's
  cache invalidation contract is **post-commit**. Cache Del per-row
  inside an uncommitted tx is incorrect: if the chunk rolls back, the
  cache evictions for "successful" rows remain even though those rows
  never committed. Cache Del MUST run after the chunk's outer
  `tx.Commit()` succeeds, walking the chunk's `succeeded[]` list.
- **DO NOT mint UUIDv7 for entity rows in Phase 5** — wait, actually
  the import_jobs.id is minted in Phase 5 (`uuid.NewV7()`), but
  agent/skill/queue/channel/adapter/break_reason IDs are minted INSIDE
  the upsert SQL via the handler param (`InsertAgent ... VALUES
  ($1=id, ...)`). The handler passes a freshly-minted UUIDv7 in the
  insert path; the upsert's `ON CONFLICT DO UPDATE` branch ignores
  the supplied id and uses the existing row's id. Pattern matches
  `catalog/agents.go:100` (`id := uuid.Must(uuid.NewV7())`).
- **DO NOT skip `agent_states` INSERT for new agents.** Phase 4
  `04-PATTERNS.md:1249` (Hazard 7) and `:1272` (Phase 5 Inheritance
  Reminder) — every new agent must seed an `agent_states` row in the
  same SAVEPOINT tx, with `ON CONFLICT (agent_id) DO NOTHING` so
  re-imports don't regress state. Phase 5 Wave 3's agent row
  processor MUST include this insertion.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CSV parsing (RFC 4180, quoted fields, embedded commas/newlines) | Custom split-by-comma | `encoding/csv` stdlib | Quoted fields with embedded commas, CRLF inside quotes, doubled quotes (`""`) — all RFC 4180 edge cases the stdlib handles correctly. Pitfall 5.1 sources document the silent-failure modes when these are hand-rolled. |
| BOM stripping | Read first N bytes manually as raw byte slice | `bufio.Reader.Peek(3)` | Peek is non-destructive (does not advance the read pointer) per pkg.go.dev/bufio#Reader.Peek — manual byte juggling tends to consume bytes when no BOM is present and corrupt the first field. |
| Streaming JSON array decode | `json.Unmarshal` to `[]interface{}` after `io.ReadAll` | `json.Decoder` Token+More loop | Eager decode → memory blowup; streaming is the v0.2 path. **But for v0.1, the strict-server eager-decodes regardless — accept it; see Pattern 4.** |
| Body-size limits | Custom `io.Reader` wrapper | `http.MaxBytesReader` + `*http.MaxBytesError` | Stdlib since 1.0; typed error since 1.19. Custom wrappers tend to miss the chunked-transfer-encoding case. |
| SAVEPOINT management | Raw `tx.Exec(ctx, "SAVEPOINT ...")` strings | `pgx.Tx.Begin()` returns a child Tx backed by SAVEPOINT | pgx idiom; avoids manual savepoint name uniqueness management. |
| Per-row insert + cache Del orchestration | Hand-roll a transaction-with-cache module | Phase 3's `cache.Cache.Del` post-commit pattern (`catalog/agents.go:190`) | Phase 3 owns CAT-11 cache lifecycle; Phase 5 wraps it. |
| Idempotency-Key UUID parsing | Hand-roll a UUID validator | `openapi_types.UUID` (oapi-codegen runtime) | Already used everywhere in the codebase; consistent error shape. |
| Transition logic for sweep goroutine | New ticker pattern | Mirror `state.Server.safetySweep` in `state/ttl.go:160` verbatim | Phase 4 paid the cost; Phase 5 inherits. Pitfall avoidance was thorough (8 hazards documented in 04-PATTERNS.md). |
| Per-entity row column registry | One-off `if entity == "agents" {...}` switches scattered throughout the handler | Single registry map: `entityRegistry: map[ImportEntityType]entityColumnRegistry` with required/optional column lists + typed dispatch func | Centralises the per-entity contract; addition of a 7th entity in v0.2 is a one-line registry insert, not a 6-place audit. |

**Key insight:** Bulk import touches every reusable layer of the stack
(parsing, validation, persistence, cache, observability). The
temptation is to write a "monolithic import service" — resist it. Each
layer already has a Phase 1-4 pattern; Phase 5 is integration glue.

## Runtime State Inventory

> Phase 5 is greenfield code (a new package, new migration, new endpoint
> implementations). No renames, no refactors, no string replacements
> are involved. **This section is intentionally OMITTED — no rename
> scope applies.**

## Common Pitfalls

### Pitfall 1: oapi-codegen `oneOf` request body unusable (D5-25 architectural risk)

**What goes wrong:** Following D5-25 literally and using `oneOf: [6
Import*Request]` for the request body schema generates a Go type with
`union json.RawMessage` and ZERO accessor methods (oapi-codegen v2
generates `AsX/FromX` ONLY for response bodies as of v2.7.0). The handler
has no way to decode it.

**Why it happens:** oapi-codegen issue #1620 (open since 2024)
[VERIFIED: github.com/oapi-codegen/oapi-codegen/issues/1620] documents
this as a known gap. v2 generates `AsX/FromX` for response oneOf but
not request oneOf.

**How to avoid:** Keep the spec request body as `application/json:
{ type: array, items: {} }` (current Phase 2 state). Author the 6 new
`Import*Request` schemas as `components.schemas` (so they appear in
`types.gen.go` and clients can use them), but the request body schema
points to `items: {}` (no oneOf). Handler dispatches by `?entity=` and
re-marshals each `interface{}` element into the right typed struct via
the round-trip helper in Pattern 4.

**Warning signs:** `task gen` produces a generated type with a `union
json.RawMessage` field and no accessor methods — that's the failure mode.

### Pitfall 2: Strict-server eager JSON decode loads full body into memory (D5-23 architectural risk)

**What goes wrong:** The strict-server wrapper at `server.gen.go:5629-5641`
calls `json.NewDecoder(r.Body).Decode(&body)` to eagerly load the
entire JSON request body into `req.JSONBody` BEFORE the handler runs.
50 MB raw JSON → ≈ 200-300 MB live `map[string]interface{}` representation.
D5-23 says "JSON parsing is streaming, not eager" — but the strict-server
defeats this for application/json.

**Why it happens:** oapi-codegen's strict-server flavor is opinionated
about request decoding for the common case (POST with typed body). It
doesn't expose a per-operation knob to skip auto-decode.

**How to avoid:** Three options, in order of preference:

1. **Accept eager decode in v0.1.** Memory is bounded by D5-21
   MaxBytesReader to 50 MB raw → ≈ 250 MB live max. Acceptable for v0.1
   catalog scale. Document this in the architectural decisions.
2. **Customize codegen with `client-server-templates` or a
   per-operation skip-decoding directive.** Increases build complexity;
   not worth the risk for v0.1.
3. **Split BulkImportCatalog into per-entity endpoints** (e.g.,
   `POST /v1/orgs/{org_id}/catalog/import/agents`). Each endpoint has
   a typed body shape, eager decode becomes acceptable for that one
   shape, and the array-of-interfaces problem disappears. But this
   contradicts the locked D5-14 single-endpoint contract.

**Warning signs:** Importing a 50 MB JSON file with 500 large
rows causes the binary to allocate ≈ 250 MB heap before the handler
runs. Profile early with a representative payload.

### Pitfall 3: BOM consumed by `csv.Reader` as part of first column name (Pitfall 5.1)

**What goes wrong:** Excel-on-Windows saves CSV with UTF-8 BOM (3
bytes: 0xEF 0xBB 0xBF) at the start. Go's `encoding/csv` does NOT
strip BOM. The first cell of the header row reads as `\xEF\xBB\xBFexternal_id`,
header validation fails with "missing required column external_id",
admin gets a confusing 400.

**Why it happens:** Phase 2 RESEARCH and PITFALLS 5.1 documented this;
the stdlib decision is to leave BOM handling to callers.

**How to avoid:** Pattern 3's `stripBOM` helper. Required to write a
`testdata/windows-excel-agents.csv` fixture (UTF-8 BOM + CRLF + at
least one quoted-with-embedded-comma field) and a regression test
asserting the BOM file imports successfully.

**Warning signs:** CSV imports work in dev (macOS/Linux LF, no BOM)
but fail on customer files. Always test with a real Excel-exported
file before phase close.

### Pitfall 4: SQLChecker rejects sweep query without org_id (Pitfall 4 from Phase 4)

**What goes wrong:** The crash-recovery sweep SQL is
`UPDATE import_jobs SET status='failed' WHERE status='pending' AND
updated_at < NOW() - INTERVAL '24 hours' RETURNING id, org_id`. No org_id
in the WHERE clause. SQLChecker's MustContainOrgFilter rejects this.

**Why it happens:** SQLChecker (`services/api/internal/db/sqlcheck.go:73`)
parses every SQL and requires org_id to appear in WHERE/INSERT/UPDATE
for tables listed in `tenantTables`. `import_jobs` MUST be added to
`tenantTables` per the new table's denormalized org_id column.

**How to avoid:** Two parts:

1. Add `"import_jobs": {}` to `tenantTables` in `sqlcheck.go:32`. The
   table has org_id so this is the right policy.
2. The crash-sweep goroutine wraps its ctx with `db.WithBypass(ctx,
   "import_crash_sweep")` per Phase 4 STATE-07 (`state/ttl.go:93`).
   The bypass emits a structured slog event (orgdb.go:142) and SKIPS
   the org_id-presence check — required because the sweep is org-agnostic.

**Warning signs:** sweep goroutine panics in dev (ValidationPanic mode)
with `orgdb.Exec preflight failed: orgdb: SQL string missing org_id
filter`. Fix: add the bypass.

### Pitfall 5: Missing `agent_states` INSERT on new agent import (Phase 4 Hazard 7)

**What goes wrong:** Phase 5's agent row processor upserts the `agents`
table but forgets to INSERT into `agent_states`. The agent appears in
GET /agents but GET /agents/{id}/status returns 404, breaking
IsRoutable downstream.

**Why it happens:** Phase 4 introduced the invariant that EVERY agent
has a sibling agent_states row, seeded atomically in the same Tx as
the agent insert (`catalog/agents.go:152`). Phase 5's wider import
path could miss this.

**How to avoid:** Phase 4 `04-PATTERNS.md:1272-1313` documents the
exact pattern Phase 5 must use. After every `UpsertAgentByExternalId`
call inside the chunk's savepoint Tx, immediately call
`qtx.InsertAgentState(ctx, ...)` with `ON CONFLICT (agent_id) DO
NOTHING` so re-imports don't reset the state machine to Offline. A
regression test (`TestBulkImport_SeedsAgentStatesForNewAgents`) is
mandatory.

**Warning signs:** Importing 3 agents via CSV; GET /agents/{each}/status
returns 404 for any of them.

### Pitfall 6: Per-row cache invalidation before chunk commit

**What goes wrong:** Handler calls `cache.Del(key)` for each successful
row immediately. If the chunk commit fails, the cache is now empty for
rows that never landed in the DB. Next GET re-loads stale data from
DB (which doesn't have the row) — appears correct (returns 404) but
the cache state diverged.

**Why it happens:** Over-eager cleanup; mirrors a tempting
"DRY" instinct.

**How to avoid:** Buffer successful row UUIDs per chunk in a local
`succeededInChunk []uuid.UUID` slice. ONLY after `outerTx.Commit()`
succeeds, walk that slice and call `cache.Del`. Mirror Phase 3's
post-commit pattern (`catalog/agents.go:190`).

**Warning signs:** Tests fail intermittently with "cache hit returned
stale row" after chunk-commit-failure injection.

### Pitfall 7: `Content-Length` header trust without MaxBytesReader fallback (D5-21)

**What goes wrong:** Handler trusts `Content-Length` only. Client
sends `Content-Length: 1000` but the actual body is 100 MB (chunked
transfer encoding, or a malicious client). Pre-flight passes
(`1000 < 50<<20`); handler reads body, OOMs.

**Why it happens:** Content-Length is a hint, not a guarantee.

**How to avoid:** D5-21 explicitly mandates BOTH the pre-flight AND
`http.MaxBytesReader` wrap. The Pattern 2 middleware does both. Tests
MUST include a "lying Content-Length" case (set Content-Length: 100,
send 100MB body — assert 413 from mid-stream MaxBytesError).

**Warning signs:** Smoke test with a real curl + `--data-binary @bigfile.csv`
where `bigfile.csv` is 100 MB → server doesn't OOM but doesn't return
413 either — it just hangs reading. Indicates MaxBytesReader isn't
wired.

### Pitfall 8: External-id batch lookup N+1 (D5-16 / D5-17)

**What goes wrong:** Agent row processor calls `q.GetSkillByExternalId(ctx,
{orgID, code})` per skill per row. 50 rows × 3 skills = 150 round trips.

**Why it happens:** Naïve per-row implementation.

**How to avoid:** Pattern 6's `ResolveSkillExternalIds(ctx, orgID,
[]codes)` query takes a single text array (`text[]`) and returns one
batch. Handler collects all unique skill external_ids across the
50-row chunk, issues ONE lookup, builds `map[string]uuid.UUID`,
re-uses for every row in the chunk.

**Warning signs:** Performance tests show 10x slower per-row latency
than expected. EXPLAIN ANALYZE shows 150 separate index scans where 1
should be.

### Pitfall 9: Idempotency-Key replay with `succeeded[]` reconstruction temptation

**What goes wrong:** Engineer sees that `import_jobs` stores counters
+ errors but not the original `succeeded[]` UUIDs. To avoid the
"return empty succeeded" limitation, they decide to add a
`succeeded_ids JSONB` column AT THE LAST MINUTE without fully
thinking through the storage shape, then ship a broken replay.

**Why it happens:** D5-13's known limitation looks easy to fix.

**How to avoid:** D5-13 EXPLICITLY locks v0.1 to return empty
`succeeded[]` + `idempotent_replay: true`. The CONTEXT.md surfaces
this as a known limitation. The planner MUST NOT relitigate. v0.2
deferred to `succeeded_ids JSONB` enhancement.

**Warning signs:** A new column appears in the migration without
corresponding CONTEXT.md decision.

### Pitfall 10: Tx.Begin on already-committed/rolled-back outer Tx (pgx savepoint footgun)

**What goes wrong:** Engineer puts the per-row savepoint Begin AFTER
some failure handling that already rolled back the outer Tx; pgx
returns `tx closed` error on `outerTx.Begin(ctx)`.

**Why it happens:** Mixing defer rollback patterns with explicit
rollback.

**How to avoid:** Structure the chunk loop as:
- outer Tx open
- per-row savepoint inside loop (Begin → process → Commit/Rollback the savepoint)
- after loop: outer Commit
- defer outer Rollback handles only the no-op-after-commit case

The deferred Rollback after a Commit is a pgx no-op
(`catalog/agents.go:111` documents this: "safe after commit — pgx
ignores."). This pattern is already proven in Phase 3.

**Warning signs:** Test fails with `tx closed: nothing to roll back to`
after the first failure.

## Code Examples

(See Architecture Patterns §1-9 above. All code examples there are
verified against the working tree or stdlib docs; sources are cited
inline.)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Per-row autocommit | Batched-savepoint with chunk size 50 | D5-09 | ~10× faster than per-row; chunk-atomic semantics. |
| Eager JSON load via `io.ReadAll` then `json.Unmarshal` | Streaming `json.Decoder` token-mode | D5-23 (aspirational) | Memory O(1 row). ⚠️ Defeated by strict-server eager decode in v0.1 — Pitfall 2. |
| Charset auto-detect (chardet) | UTF-8 strict + BOM strip | D5-08 + Pitfall 5.1 | Eliminates silent corruption (the #1 source per Pitfall 5.1). |
| Per-row N+1 lookup of skill external_id | Batch `ANY($1::text[])` resolve | D5-16, D5-17 | 150 round trips → 1 per chunk. |
| `oneOf` for polymorphic request body | `items: {}` + `?entity=` dispatch | This research [VERIFIED: oapi-codegen issue #1620] | Hard library constraint — oapi-codegen v2 doesn't generate request-body union helpers. |
| `setInterval`-style WrapUp TTL (Phase 4 STATE-07 research) | `time.Ticker` + `clockwork.Clock` + WaitGroup | Phase 4 ship | Phase 5 inherits identical pattern. |

**Deprecated/outdated:**
- The pgx v4 SAVEPOINT pattern (manual `tx.Exec("SAVEPOINT ...")`) is
  superseded by pgx v5's `tx.Begin()` returning a child Tx [VERIFIED:
  pkg.go.dev/github.com/jackc/pgx/v5]. Phase 5 uses the typed v5 idiom.
- ❌ `github.com/segmentio/encoding` for streaming JSON — third-party
  alternative the v0.1 stack doesn't need.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | oapi-codegen v2.7.0's strict-server generates `json.NewDecoder(r.Body).Decode(&body)` for application/json bodies | Pitfall 2 | Low — verified in working tree at services/api/internal/api/server.gen.go:5629-5641. |
| A2 | Adding `"import_jobs": {}` to `tenantTables` is sufficient for SQLChecker to accept the new table's queries | Pitfall 4 | Low — exactly mirrors Phase 4's `agent_states` addition (sqlcheck.go:41). |
| A3 | Phase 3's per-entity CACHE invalidation via `cache.Del(cache.Key(orgID, "agents", id))` extends 1:1 to bulk import without modification | Don't Hand-Roll table | Medium — Phase 3's pattern caches **detail** GET responses; bulk import has no detail return. The post-commit Del per succeeded row is still correct because future GETs will repopulate. Verified pattern in `catalog/agents.go:190`. |
| A4 | `bufio.Reader.Peek(3)` is non-destructive (does not advance read pointer) and works correctly on `*http.MaxBytesReader`-wrapped readers | Pattern 3 | Low — pkg.go.dev/bufio#Reader.Peek docs explicit; MaxBytesReader returns `io.ReadCloser` which bufio wraps fine. |
| A5 | `pgx.Tx.Begin()` on a transaction (creating a savepoint) is safe to call inside a hot loop and the child Tx's Commit/Rollback do not interfere with the parent's Commit | Pattern 5 | Low — documented pgx v5 idiom; verified in pgx v5 docs and used in Phase 1-3 tests. |
| A6 | Memory cost of `200-300 MB live for 50 MB raw JSON map[string]interface{}` is acceptable at v0.1 catalog scale | Pitfall 2 | Medium — depends on deployment RAM. Most cloud Postgres-fronting Go binaries have ≥ 1 GB. Should be flagged at deploy time. v0.1 docker-compose default is unconstrained. |
| A7 | The CONTEXT.md path reference `services/api/internal/server/stubs.go:230,254` is stale and the actual stubs live in `services/api/internal/catalog/notimpl.go:28,34` | Summary | High — VERIFIED via grep + Read of the working tree. Planner must update its file pointers. |
| A8 | Phase 4's `state.Server.Start/Stop` lifecycle pattern (clockwork + WaitGroup + ctx cancel) ports 1:1 to Phase 5's crash-sweep | Pattern 8 | Low — explicit RESEARCH+CONTEXT statement (D5-11) + verified Phase 4 ship behavior. |
| A9 | The recommended chi middleware path-prefix scoping for `BodyLimit` works because the import URL has a unique prefix `/v1/orgs/{org_id}/catalog/import` and no other endpoint shares it | Pattern 2 | Low — verified via openapi.yaml `paths:` survey. |
| A10 | Per-org rate limiting being deferred (D5 deferred ideas) is acceptable for v0.1 — a hostile authenticated client could flood the endpoint with 50-MB / 500-row POSTs in a loop | Security Domain | Medium — explicitly accepted in v0.1; OrgContext is stub auth so all clients are "trusted" by current contract. Production hardening flagged in deferred items. |

**If this table is empty:** It is not. 10 assumptions captured. A6, A7,
and A10 are MEDIUM-or-higher risk and the planner must surface them in
the plan-checker pass.

## Open Questions

1. **Idempotency-Key replay response shape — should `BulkImportResult`
   gain `idempotent_replay: boolean`?**
   - What we know: D5-27 says yes, add the field. The CONTEXT.md
     explicitly calls this out as "Implementation Decision 5-27."
   - What's unclear: Whether the planner can add this field to
     `BulkImportResult` without breaking the existing 200/207/422
     consumers (the field is `nullable: true` with default false, so
     existing consumers ignore it). Need to verify no v0.1 admin UI
     bug from the new field.
   - Recommendation: Add as `nullable: true, default: false, readOnly:
     true`. Codegen-drift CI will catch any consumer break.

2. **Should the crash-sweep goroutine fall back to per-org iteration
   if `db.WithBypass` audit-log volume becomes a concern?**
   - What we know: db.WithBypass emits one structured slog event per
     bypass call. 1-hour ticker × N orgs = N log events per sweep.
     For v0.1 docker-compose with a handful of test orgs, negligible.
   - What's unclear: Production v1 with 100s of orgs makes this
     noisy.
   - Recommendation: Accept bypass for v0.1; tag as v0.2 hardening item.

3. **Should `Import*Request` schemas be inline or in `openapi/components/imports.yaml`?**
   - What we know: Current spec is 2829 lines; Phase 2 deferred the
     spec-splitting until ~3000 lines.
   - What's unclear: 6 new schemas adds ~250 lines, putting spec at
     ~3080. Crosses the threshold.
   - Recommendation: Inline is fine for v0.1; flag as v0.2
     refactor. Splitting now would require Phase 2 codegen pipeline
     changes that exceed the v0.1 risk budget.

4. **For the `adapters` and `break_reasons` entities (which have NO
   external_id), how does `Upsert{Entity}` key the conflict?**
   - What we know: `adapters.UNIQUE` is just PRIMARY KEY (id);
     `break_reasons.UNIQUE` is `(org_id, name)`.
   - What's unclear: An import row for adapters has no caller-stable
     identifier to upsert on — the user can't supply `id`. Same for
     break_reasons except `name` is unique.
   - Recommendation: For adapters, `UpsertAdapterByName` keyed on
     `(org_id, name)` — adapter rows have a `name` column. For
     break_reasons, `UpsertBreakReasonByName` keyed on `(org_id, name)`.
     For both, the schema needs a `UNIQUE (org_id, name)` constraint
     on adapters (NOT currently present — see `migrations/000002_catalog_v0_1.up.sql:120-130`).
     ⚠️ **This is a SCHEMA CHANGE** required by Phase 5 — the migration
     `000002_catalog_v0_1.up.sql` is editable per D-61 only until v0.1
     ships, so Phase 5 can add `UNIQUE (org_id, name)` to adapters
     **either by editing the existing migration in place (if Phase 3+4
     considered acceptable per the editable-migration policy) OR by a
     new migration 000003**. Recommend the latter for cleaner audit. The
     planner picks.

5. **Where does the 422 partial-failure case fire vs the 207 case?**
   - What we know: D5-* says 200 = all succeed; 207 = some failed
     AND some succeeded; 422 = ALL failed (zero succeeded).
   - What's unclear: An import with zero rows after header validation
     (admin uploads an empty CSV after the header) — is that 200 with
     empty `succeeded[]` / empty `failed[]`? Or 422?
   - Recommendation: 200 with empty arrays (the IMP-05 contract: 422
     is "no rows succeeded"; an empty input vacuously has no failures
     either). Document.

6. **Does `bodyLimit` middleware need to run BEFORE `orgContextMiddleware`?**
   - What we know: Mid-stream MaxBytesError happens during request
     body read — that's typically inside the strict-server's
     `json.NewDecoder(r.Body).Decode(&body)` call. orgContextMiddleware
     only reads the X-Org-Id header (cheap).
   - What's unclear: Ordering for the rejection path. If a 100MB body
     arrives with missing X-Org-Id, do we want 400 (missing org_id)
     or 413 (body too large)?
   - Recommendation: 400 first (cheap rejection before body read).
     Run `orgContextMiddleware` BEFORE `bodyLimit` so unauth requests
     short-circuit without ever wrapping the body. This matches
     Phase 2 hardening pattern for `uuidv7PathParamsMiddleware`
     (server.go:146 — orgContext before path validation).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Source build | ✓ (assumed for all phases) | 1.25.10 | — |
| PostgreSQL 17 | Migration + integration tests | ✓ (via docker-compose + testcontainers) | 17 | — |
| Redis 7+ | Phase 3 cache (Phase 5 reuses) | ✓ (via docker-compose + miniredis test fallback) | 9.x client | — |
| sqlc CLI | Codegen for new queries | ✓ (Phase 1+2 CI install verified) | v1.31.1 | — |
| oapi-codegen | Regen after spec edits | ✓ (Phase 2 install verified) | v2.7.0 | — |
| golang-migrate | Apply 000003 migration | ✓ (Phase 1 CI install verified) | v4.19.1 | — |
| Excel / LibreOffice (for golden testdata) | Generate windows-excel-agents.csv real Excel file | ⚠️ Development machine only | macOS/Linux Excel or LibreOffice | Use a hex editor + python `b'\xef\xbb\xbf...'` to author bytes manually if Excel unavailable. |

**Missing dependencies with no fallback:** none.

**Missing dependencies with fallback:** Excel binary access — author
golden CSV via byte-level scripting if needed. Risk of NOT using a
real Excel export: the file may pass tests but not catch real-world
edge cases. Recommendation: have the user generate one Excel export
during Wave 5 testing.

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
| IMP-02 | UTF-8 BOM is stripped; CRLF handled; embedded commas/quotes/newlines decoded | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_WindowsExcel_BOM_CRLF` (uses `testdata/windows-excel-agents.csv` golden file) | ❌ Wave 5 |
| IMP-02 | Invalid UTF-8 → 400 csv_not_utf8 | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_InvalidUTF8_400` | ❌ Wave 5 |
| IMP-03 | Upsert keyed by (org_id, external_id); same payload twice → no duplicates | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_Idempotent_DoubleRun_NoDuplicate` | ❌ Wave 5 |
| IMP-04 | Failed rows return structured errors with row + field + message | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_FailedRow_Structure` | ❌ Wave 5 |
| IMP-05 | 200 all succeed; 207 partial; 422 all fail | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_HTTPStatus_(200\|207\|422)` | ❌ Wave 5 |
| IMP-06 | import_jobs persisted; GET /imports/{id} returns it | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_GetImportJob_RoundTrip` | ❌ Wave 5 |
| IMP-07 | 50 MB → 413; 500 rows → 413 | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_Oversize_413` (50MB+ body + 501-row CSV) | ❌ Wave 5 |
| IMP-08 | CSV requires ?schema_version=v0.1; mismatch → 400 with supported versions | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_SchemaVersion_Mismatch_400` | ❌ Wave 5 |
| D5-01 | Typed coercion pipeline: trim→lower→split→parse per field type | unit | `go test -count=1 ./internal/imports/ -run TestCoerce_(Bool\|Int\|Multi)` | ❌ Wave 2 |
| D5-09 | Chunked-savepoint behaviour: per-row failure rollback; chunk-level commit | integration (testcontainers Postgres) | `go test -count=1 ./internal/imports/ -run TestChunk_SavepointRollback` | ❌ Wave 3 |
| D5-11 | Crash sweep flips pending → failed after 24h | unit (clockwork) | `go test -count=1 ./internal/imports/ -run TestSweep_PendingOver24h_FlipsToFailed` | ❌ Wave 5 |
| D5-13 | Idempotency-Key replay returns prior result; new key proceeds; replay has idempotent_replay=true | integration | `go test -count=1 ./internal/imports/ -run TestIdempotencyKey_(Replay\|MissProceed)` | ❌ Wave 4 |
| D5-15 | Per-entity Import*Request schemas; FK by external_id; unknown external_id → unknown_skill | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_UnknownSkillExternalId` | ❌ Wave 3 |
| D5-18 | Skill MERGE on update; existing skill not in import retained | integration | `go test -count=1 ./internal/imports/ -run TestAgentImport_SkillMerge_PreservesExisting` | ❌ Wave 3 |
| Hazard 7 (Phase 4 carry) | New agent import seeds agent_states row | integration | `go test -count=1 ./internal/imports/ -run TestBulkImport_SeedsAgentStatesForNewAgents` | ❌ Wave 3 |
| FOUND-08 | Cross-org isolation: orgA imports, orgB GET /imports/{id} → 404 | integration | `go test -count=1 ./test/isolation/ -run TestImport_CrossOrg_404` | ❌ Wave 5 |
| Pitfall 8 (cache) | Per-entity cache.Del fires only after chunk commit | unit (miniredis) | `go test -count=1 ./internal/imports/ -run TestChunk_CacheInvalidation_PostCommitOnly` | ❌ Wave 3 |
| Pitfall 7 (Content-Length lie) | 100MB body with Content-Length:1000 → 413 via MaxBytesError | integration | `go test -count=1 ./internal/middleware/ -run TestBodyLimit_LyingContentLength_413` | ❌ Wave 1 |

### Sampling Rate

- **Per task commit:** `task test:quick` (covers internal/imports/ unit tests, sub-30-second target).
- **Per wave merge:** `task test` (full suite including testcontainers Postgres; ≈ 1-2 minutes given Phase 4's full suite is currently 404 tests / sub-2min).
- **Phase gate:** Full suite green + `task gen` produces no diff + isolation suite passes + `task lint` clean before `/gsd:verify-work`.

### Wave 0 Gaps

- [ ] `services/api/internal/imports/coerce_test.go` — covers D5-01 / D5-02 / D5-04 / D5-06 / D5-07 / D5-08
- [ ] `services/api/internal/imports/parser_csv_test.go` — covers IMP-02 / D5-08 / Pitfall 5.1
- [ ] `services/api/internal/imports/parser_json_test.go` — covers D5-23 / Pitfall 2 (eager-decode-is-acceptable-with-MaxBytes)
- [ ] `services/api/internal/imports/header_test.go` — covers D5-05 strict header policy
- [ ] `services/api/internal/imports/chunk_test.go` — covers D5-09 / D5-19 / Hazard 7
- [ ] `services/api/internal/imports/jobs_test.go` — covers IMP-06 / D5-10
- [ ] `services/api/internal/imports/idempotency_test.go` — covers D5-13
- [ ] `services/api/internal/imports/sweep_test.go` — covers D5-11 (clockwork-backed)
- [ ] `services/api/internal/imports/handlers_test.go` — entity × format matrix end-to-end (≥ 12 cases: 6 entities × 2 formats)
- [ ] `services/api/internal/imports/testutil_test.go` — shared httptest harness
- [ ] `services/api/internal/imports/testdata/windows-excel-agents.csv` — UTF-8 BOM + CRLF golden file (Pitfall 5.1 NON-NEGOTIABLE)
- [ ] `services/api/internal/imports/testdata/<entity>-<scenario>.{json,csv}` — at minimum 4 scenarios per entity
- [ ] `services/api/internal/middleware/bodylimit_test.go` — covers D5-21 / Pitfall 7
- [ ] `services/api/test/isolation/imports_test.go` — covers FOUND-08 cross-org probes

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (stub) | X-Org-Id header via OrgContext (Phase 1 carry-forward). v0.1 stub auth; full auth in AUTH milestone. |
| V3 Session Management | no | Stateless REST; no sessions in v0.1. |
| V4 Access Control | yes | OrgContext + SQLChecker enforce org-scope on every query. Phase 5 inherits — no new ACL surface. |
| V5 Input Validation | yes | D5-01..D5-08 typed coercion pipeline; D5-05 strict header policy; oapi-codegen schema validation (Layer 1) + handler-side Layer 2 per Phase 3 D-74. |
| V6 Cryptography | no | No crypto authored in Phase 5. UUIDv7 minted via `uuid.NewV7` is timestamp-derived, NOT a credential. |
| V8 Data Protection | yes | `BulkImportFailedRow.reason` deliberately does NOT echo raw row content (Phase 2 OQ-2A) → eliminates PII leakage. |
| V9 Communication | inherited | TLS/HTTPS terminated upstream; not Phase 5's surface. |
| V10 Malicious Code | yes | No new third-party deps (slopcheck N/A). Existing libraries verified in Phase 1-4. |
| V12 Files & Resources | yes | Body size (D5-21) + row count (D5-22) limits. ⚠️ See Pitfall 7 + Threat T-05-2. |
| V13 API & Web Service | yes | OpenAPI 3.0 spec contract; strict-server enforces. |
| V14 Configuration | inherited | docker-compose env; not Phase 5's surface. |

### Known Threat Patterns for {chi + pgx + stdlib CSV/JSON + Redis}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Body-size DoS (multi-GB POST) | Denial of Service | `http.MaxBytesReader` middleware + Content-Length pre-flight (D5-21, Pattern 2) |
| Row-count DoS (millions of tiny rows) | Denial of Service | Inline counter; exit at row 501 with 413 (D5-22) |
| Slow-loris on body stream | Denial of Service | `srv.ReadHeaderTimeout: 10s` (cmd/api/main.go:218) + chi's default request timeout. ⚠️ no per-request body-read timeout in v0.1; flag as v0.2 hardening. |
| JSON bomb (deeply nested) | Denial of Service | `json.Decoder` doesn't impose depth limits; relies on body size cap. 50 MB cap bounds worst-case stack depth to ~5 GB which Go can handle (1 GB stack default). Acceptable for v0.1. Flag for hardening. |
| Billion-laughs-analog CSV (very wide row) | Denial of Service | `csv.Reader` doesn't impose field count limit; relies on body size cap. 50 MB / row count combined → ~100 MB max row width worst case. Acceptable for v0.1; the spec-derived per-entity column count caps real-world width. |
| SQL injection via CSV cell | Tampering | All writes go through sqlc-generated parameterized queries via OrgDB. Cells are passed as positional args, never concatenated into SQL. SQLChecker enforces org_id. |
| Cross-org write via crafted external_id | Information Disclosure | `(org_id, external_id)` upsert key is org-scoped by construction. Phase 1 D-04 SQLChecker rejects any query missing org_id; Phase 5's new upsert SQL includes org_id in INSERT params and the ON CONFLICT key. Integration test required: orgA imports `external_id=FOO`; orgB GET shows nothing. |
| Idempotency-Key replay against stale data | Tampering | Replay returns PERSISTED prior result; does NOT re-execute. Hash-collision attack mitigated by `UNIQUE (org_id, idempotency_key)` constraint — duplicate key from different orgs is impossible (partial unique scoped by org). |
| Idempotency-Key exhaustion | Denial of Service | Client-supplied UUIDv7 → 2^122 keyspace. Per-org spam still possible (50,000 distinct keys × 50MB payload = 2.5TB DB growth in v0.1 — accept; rate limiting deferred per D5 deferred). |
| Org_id propagation lost in goroutine | Information Disclosure | Crash-sweep goroutine uses `db.WithBypass(ctx, "import_crash_sweep")` per D-04 carry-forward; structured slog audit event on every bypass. NO request ctx leaks into the goroutine — fresh ctx with `context.Background()` per Phase 4 ttl.go:71. |
| Eager-decode memory blowup → OOM kill | Denial of Service | MaxBytesReader bounds raw to 50 MB → live ≤ ~300 MB. Below 1 GB Go binary default. Acceptable for v0.1. |
| `slog.Debug("import: int_truncation", ..., "raw", "7.5")` log inflation | Denial of Service | D5-07 emits debug-level logs only on the truncation path. Per-row in a 500-row import = 500 log lines worst case. Acceptable. |
| BOM bytes consumed as field data (Pitfall 5.1) | Tampering | Pattern 3 stripBOM + utf8.Valid → 400 csv_not_utf8 on invalid. |
| Schema-version-skew silent corruption (Pitfall 5.4) | Tampering | D5-05 strict header policy: unknown column → 400 at batch level. Forces explicit schema migration when v0.2 ships. |

**v0.1-accepted risks (explicit):**
- Per-org rate limiting is deferred. A hostile authenticated client can flood with valid 50MB / 500-row POSTs.
- Distributed lock for crash-sweep is deferred. Multi-replica deployment races (benign: idempotent UPDATE) but generates duplicate slog warnings.
- Reverse-proxy body-size alignment is a deploy concern (Pitfall 5.3).

## Sources

### Primary (HIGH confidence)

- **Working tree code (verified by Read + grep):**
  - `services/api/internal/api/server.gen.go:3640` (BulkImportCatalogRequestObject)
  - `services/api/internal/api/server.gen.go:5624-5661` (strict-server eager JSON decode)
  - `services/api/internal/api/types.gen.go:1124` (BulkImportCatalogJSONBody = []interface{})
  - `services/api/internal/catalog/notimpl.go:28,34` (current stubs — CONTEXT.md path drift)
  - `services/api/internal/state/ttl.go` (Phase 4 STATE-07 goroutine pattern to mirror)
  - `services/api/internal/state/handlers.go` (Phase 4 Server lifecycle pattern)
  - `services/api/internal/db/orgdb.go:223` (OrgTx.Begin pattern; Phase 5 must extend with BeginSavepoint)
  - `services/api/internal/db/sqlcheck.go:32-42` (tenantTables — must add `import_jobs`)
  - `services/api/internal/db/queries/agents.sql` (Phase 3 query template — NO upsert helper exists)
  - `services/api/internal/catalog/agents.go:71,113,152,190` (atomic tx + cache.Del + InsertAgentState patterns)
  - `services/api/internal/cache/cache.go` (Phase 3 cache contract)
  - `services/api/internal/middleware/httputil.go:34-67` (WriteError contract)
  - `services/api/cmd/api/main.go:178-194` (ApiHandlers composite)
  - `migrations/000002_catalog_v0_1.up.sql:191-212` (agent_states pattern to inherit)
  - `openapi/openapi.yaml:1100-2829` (BulkImport schemas + paths + Create*Request templates)
  - `services/api/go.mod` (current versions: pgx v5.9.2, oapi-codegen v2.7.0, clockwork v0.4.0, uuid v1.6.0)

- **Official Go stdlib docs:**
  - [pkg.go.dev/encoding/csv](https://pkg.go.dev/encoding/csv) — CRLF auto-conversion, LazyQuotes default false, FieldsPerRecord 0/-/+ semantics
  - [pkg.go.dev/net/http#MaxBytesReader](https://pkg.go.dev/net/http#MaxBytesReader) — typed `*MaxBytesError`, connection-close behaviour
  - [pkg.go.dev/encoding/json#Decoder](https://pkg.go.dev/encoding/json#Decoder) — Token+More streaming + error is non-recoverable
  - [pkg.go.dev/bufio#Reader.Peek](https://pkg.go.dev/bufio#Reader.Peek) — non-destructive Peek
  - [pkg.go.dev/unicode/utf8#Valid](https://pkg.go.dev/unicode/utf8#Valid)
  - [pkg.go.dev/strconv](https://pkg.go.dev/strconv), [pkg.go.dev/math#Trunc](https://pkg.go.dev/math#Trunc)
  - [pkg.go.dev/github.com/jackc/pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5) — pgx v5 SAVEPOINT idiom via `Tx.Begin()`

- **Phase 1-4 RESEARCH + CONTEXT + PATTERNS + VERIFICATION docs (CARRY-FORWARD LOCKED):**
  - `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` — D-01..D-31 (orgDB, UUIDv7, RequestID, slog/OTel)
  - `.planning/phases/02-openapi-contract-codegen/02-CONTEXT.md` — D-32..D-48 (strict-server, error envelope)
  - `.planning/phases/02-openapi-contract-codegen/02-HARDENING-SUMMARY.md` — RequestID injection middleware exhaustive type-switch
  - `.planning/phases/03-catalog-crud-go/03-VERIFICATION.md` — D-49..D-77 verified state
  - `.planning/phases/04-agent-state-machine-go/04-PATTERNS.md:1249-1318` (Hazard 7 + Phase 5 Inheritance Reminder — MANDATORY)
  - `.planning/phases/04-agent-state-machine-go/04-VERIFICATION.md` — STATE-07 goroutine pattern verified state
  - `.planning/research/PITFALLS.md` §5 — Bulk import pitfalls 5.1-5.4

### Secondary (MEDIUM confidence — WebSearch verified with stdlib/repo docs)

- [github.com/oapi-codegen/oapi-codegen issue #1620 (oneOf request body)](https://github.com/oapi-codegen/oapi-codegen/issues/1620) — confirms AsX/FromX NOT generated for request-body oneOf in v2
- [oapi-codegen union docs](https://github.com/oapi-codegen/oapi-codegen) — AsX/FromX/MergeX pattern for responses
- [Go issue #9588 (CSV BOM not stripped)](https://github.com/golang/go/issues/9588) — confirms stdlib decision to leave BOM to callers
- [RFC 4180](https://datatracker.ietf.org/doc/html/rfc4180) — CSV base format
- [Idempotency-Key draft RFC](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/) — header semantics for D5-13

### Tertiary (LOW confidence — single-source / community)

- Various Go forum posts on BOM handling (corroborate the manual-strip pattern, marked LOW; the canonical fix is in the official Go issue tracker above)

## Metadata

**Confidence breakdown:**

- **Standard stack:** HIGH — every package is already in `services/api/go.mod`; versions verified via working tree + `go list -m`.
- **Architecture patterns:** HIGH — patterns 1-9 are either direct ports of shipped Phase 1-4 code (verified by Read) or stdlib idioms (verified by pkg.go.dev). The only MEDIUM-confidence item is Pattern 4 (streaming JSON under strict-server eager decode) — the resolution proposed is the only viable v0.1 path but represents a design compromise that the planner must surface for user confirmation.
- **Pitfalls:** HIGH — pitfalls 1-10 each cite either a working-tree code line, a Phase 4 PATTERNS.md hazard, or a Pitfall 5.x research entry. No speculation.
- **Validation architecture:** HIGH — the Phase 1-4 test framework (`go test` + testcontainers + miniredis fallback) is shipped and proven; Phase 5 inherits 1:1.
- **Security domain:** MEDIUM — ASVS categories mapped; threats enumerated. Two MEDIUM-risk items explicit (eager-decode memory, rate-limiting absence).
- **Open questions:** HIGH — 6 specific open questions with recommendations; planner can lock in plan-check pass.
- **Architectural risk surfacing:** HIGH — three architecture-shaping discoveries (Phase 3 missing upsert SQL, eager JSON decode, oneOf request body gap) are explicitly called out in the Summary so the planner cannot miss them.

**Research date:** 2026-05-17

**Valid until:** 2026-06-16 (30 days for stable; recheck oapi-codegen v2.7+ release notes if any new minor lands during execution since issue #1620 is open).
