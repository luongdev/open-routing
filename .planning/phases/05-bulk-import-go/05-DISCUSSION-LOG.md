# Phase 5: Bulk Import (Go) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-05-17
**Phase:** 5-Bulk Import (Go)
**Areas discussed:** CSV cell type coercion, Transactional model + jobs, N:M skills in import, Body limit + parse strategy

---

## Gray-Area Selection

Presented 4 gray areas; user selected ALL FOUR after asking for clearer
explanations of each.

| Area | Selected |
|------|----------|
| 1. CSV cell type coercion | ✓ |
| 2. Transactional model + jobs | ✓ |
| 3. N:M relationships (agent skills) | ✓ |
| 4. Body limit + parse strategy | ✓ |

**Note:** Initial AskUserQuestion was returned with "Chưa hiểu gì mấy cái
options này, giải thích cho dễ chọn đi" — re-presented with concrete
examples and Vietnamese-language plain-text explanations of each option's
implications before the user committed.

---

## Area 1: CSV Cell Type Coercion

### Question 1.1 — Bool coercion strictness

| Option | Description | Selected |
|--------|-------------|----------|
| Strict: only `true`/`false` | Case-sensitive; one canonical form; reject everything else | |
| Loose: many formats | Accept `true|false|TRUE|FALSE|True|False|1|0|yes|no`; case-insensitive | ✓ |
| Strict + auto-lowercase | Accept `true`/`false` case-insensitive but NOT `1`/`yes` | |

**User's choice:** Loose
**Notes:** User added: "khi biết kiểu trong db thì nên quyết định trước
là có trim, lower, split ,... trước không rồi mới process tiếp. Như vậy
đỡ được khối lỗi." → drove decision D5-01 (typed preprocessing pipeline
pattern dispatched by DB field type).

### Question 1.2 — Empty cell semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Empty = null/skip | Create uses default; update unchanged; required → fail row | ✓ |
| Empty = "" string | Empty string written to DB for TEXT columns | |
| Explicit NULL marker | Require `NULL` or `\N`; empty = skip (Postgres COPY style) | |

**User's choice:** Empty = null/skip
**Notes:** Standard CSV interpretation; safest default.

### Question 1.3 — Multi-value field encoding

| Option | Description | Selected |
|--------|-------------|----------|
| Skip multi-value in CSV | Only scalar fields in CSV; multi-value requires JSON or PATCH | |
| Pipe-delimited in single cell | `"voice\|chat"` or `"S1:7\|S2:9"` for skills | ✓ |
| JSON literal in cell | `'["voice","chat"]'` in the cell | |

**User's choice:** Pipe-delimited
**Notes:** User added: "sẽ chấp nhận 1 số sep như , | ;" → drove decision
D5-04 (priority `|` > `;` > `,` to avoid CSV field-delimiter ambiguity).

### Question 1.4 — Header policy (missing/unknown columns)

| Option | Description | Selected |
|--------|-------------|----------|
| Strict batch-level | Missing required → 400; unknown column → 400; no rows processed | ✓ |
| Permissive (ignore unknown) | Required missing → per-row fail; unknown → silently ignored | |
| Permissive + warning | Unknown column imported but flagged; needs spec extension | |

**User's choice:** Strict
**Notes:** Aligns with PITFALLS 5.4 (silent column drop is the worst
failure mode for saved admin templates).

### Question 1.5 — Enum case sensitivity

| Option | Description | Selected |
|--------|-------------|----------|
| Case-insensitive + auto-lower | `Voice`/`VOICE`/`voice` → `voice` then match enum | ✓ |
| Strict lowercase only | Only `voice`/`chat`/`email`; rejection on `Voice` | |

**User's choice:** Case-insensitive + auto-lower
**Notes:** Friendly to Excel exports.

### Question 1.6 — Integer parsing tolerance

| Option | Description | Selected |
|--------|-------------|----------|
| Trim + strict int (`strconv.Atoi`) | `7.0` → fail; only integer strings parse | |
| Trim + tolerant float (round?) | `7.0` → 7; behaviour ambiguous on `7.5` | |
| Trim + tolerant float (truncate) | `7.0` → 7; `7.5` → 7 (truncation); never-fail on parseable numerics | ✓ |

**User's choice:** Tolerant float, truncate
**Notes:** Excel silently rewrites `7` as `7.0` on re-save; truncation
chosen over rounding for predictability. Decision D5-07 adds slog Debug
event when truncation discards non-zero fractional part.

### Question 1.7 — Charset handling

| Option | Description | Selected |
|--------|-------------|----------|
| UTF-8 only; reject invalid | After BOM strip, validate UTF-8; non-UTF-8 → 400 | ✓ |
| UTF-8 + Latin1 fallback | Try UTF-8, fallback to `iso-8859-1` | |

**User's choice:** UTF-8 only
**Notes:** PITFALLS 5.1 calls out charset auto-detect as the #1 silent
corruption source.

---

## Area 2: Transactional Model + Jobs Lifecycle

### Question 2.1 — Transaction boundary for 500-row upsert

| Option | Description | Selected |
|--------|-------------|----------|
| Single tx + per-row SAVEPOINT | One tx for whole batch; savepoint per row for partial rollback | |
| Per-row tx (autocommit) | One tx per row; simple but 500 round-trips and no atomicity with job row | |
| Batched savepoints (chunk 50) | 10 chunks × 50 rows; each chunk = 1 tx + per-row savepoint | ✓ |

**User's choice:** Batched savepoints (chunk 50)
**Notes:** Middle-ground between transaction granularity and round-trip cost.

### Question 2.2 — `import_jobs` row lifecycle

| Option | Description | Selected |
|--------|-------------|----------|
| Create at start (pending), update at end | INSERT before chunk 1; UPDATE counters after chunk 10 commits | ✓ |
| Create at end (atomic) | INSERT only after all chunks committed; no pending state | |
| Create in last chunk tx | INSERT inside final chunk's tx for atomicity with last 50 rows | |

**User's choice:** Pending at start, finalise at end
**Notes:** Decision D5-10 + D5-26 (requires `ImportJob.status` enum
extension in openapi.yaml).

### Question 2.3 — Crash recovery for pending jobs

| Option | Description | Selected |
|--------|-------------|----------|
| Leave as-is (admin self-interprets) | Pending row persists forever; admin sees status and retries | |
| TTL cleanup after N hours | Background sweep flips `pending` > 24h → `failed` | ✓ |
| Atomic write-finalize | Conflict with Q2.2 choice; rejected | |

**User's choice:** TTL cleanup
**Notes:** D5-11 reuses Phase 4 STATE-07 goroutine pattern; 1-hour
ticker, 24-hour threshold, slog Warn on each transition.

### Question 2.4 — `errors` JSONB persisted shape

| Option | Description | Selected |
|--------|-------------|----------|
| Same shape as `BulkImportResult.failed[]` | `[{row, field, error, reason}]` — no transformation | ✓ |
| Add metadata (timestamp, attempt, raw_value) | Richer for debug but larger storage | |
| Truncate at N errors | Cap at 50 errors + `more_errors_omitted` counter | |

**User's choice:** Same shape as response
**Notes:** "có row thì cơ bản là cũng debug được rồi (số dòng)". Zero
transformation between POST response and GET response.

### Question 2.5 — Idempotency mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| None — rely on upsert idempotency | `(org_id, external_id)` upsert is row-level idempotent; new job row each POST | |
| Optional `Idempotency-Key` header | Persist key with UNIQUE; replay returns prior job | ✓ |
| Auto-key from payload hash | SHA-256 of body deduplicates within N minutes | |

**User's choice:** Optional `Idempotency-Key` header
**Notes:** Requires spec extension (D5-27 — header parameter +
`idempotent_replay` boolean on `BulkImportResult`). Known v0.1
limitation captured in D5-13: replay does not reconstruct
`succeeded[]` UUID list (deferred to v0.2 enhancement of
`import_jobs.succeeded_ids JSONB`).

---

## Area 3: N:M Skills in Import

### Question 3.1 — JSON nested skills support

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — nested per `CreateAgentRequest` shape | `skills: [{skill_id: UUID, proficiency: N}]` — requires UUIDs | |
| Yes — resolve `external_id` instead | `skills: [{skill_external_id: "CODE", proficiency: N}]` — human-readable | ✓ |
| No — skip N:M in import | Scalar-only import; PATCH after | |

**User's choice:** Resolve `external_id` (option 2)
**Notes:** User emphasised: "skill code, thay vì id, những cái gì mà
người dùng hay phải thao tác thì phải có code, chứ không phải cứ bắt
dùng cái uid dài loằng ngoằng được. Tư duy humanable." → drove decision
D5-15 (per-entity `Import*Request` schemas referencing FKs by
`external_id`).

### Question 3.2 — CSV agent imports with skills

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — single `skills` column `CODE:prof\|CODE:prof` | Pipe-separated `code:proficiency` pairs | ✓ |
| No — CSV skips skills | Admin must use JSON or post-import PATCH | |
| Yes — two parallel columns | `skill_codes` + `proficiencies` paired by index | |

**User's choice:** Single column with `CODE:prof|CODE:prof` syntax
**Notes:** Decision D5-17. Documented in CSV column description.

### Question 3.3 — Update agent via import: skills merge vs replace

| Option | Description | Selected |
|--------|-------------|----------|
| Replace entire set (PUT semantics, matches UpdateAgentRequest) | Old skills deleted; new set installed atomically | |
| Merge (PATCH semantics) — skills add/update, never remove | Incremental partial imports preserve existing assignments | ✓ |
| Skip if column empty; replace if present | Hybrid; admin-friendly for incremental imports | |

**User's choice:** Merge (PATCH semantics)
**Notes:** Decision D5-18. Documented divergence from
`UpdateAgentRequest`'s PUT semantics. Trade-off: skill REMOVAL is not
possible via import (must use UI/PATCH). Captured in deferred ideas
for v0.2 if customer demand exists.

### Question 3.4 — Existing-skill proficiency conflict

| Option | Description | Selected |
|--------|-------------|----------|
| Import wins (UPSERT proficiency) | `ON CONFLICT DO UPDATE` — import is source of truth | ✓ |
| Existing wins (DO NOTHING) | Skill already assigned, leave proficiency alone | |
| Fail row | `skill_already_assigned` per-row error | |

**User's choice:** Import wins
**Notes:** D5-19. Consistent with D5-18 merge intent (the explicit
inclusion of a skill+proficiency in the import is an authoritative
statement).

---

## Area 4: Body Limit + Parse Strategy

### Question 4.1 — 50 MB body cap enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| Middleware: `http.MaxBytesReader` wrap | Stream-safe; mid-stream over-limit → 413 | |
| Content-Length pre-flight + MaxBytesReader fallback | Pre-flight rejects oversized; MaxBytesReader catches lying/chunked clients | ✓ |
| Eager `io.ReadAll` + length check | OOM-prone; rejected | |

**User's choice:** Content-Length pre-flight + MaxBytesReader fallback
**Notes:** Decision D5-21. Both fast-fail behaviour AND memory safety.

### Question 4.2 — 500-row cap enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| Streaming: count during parse, terminate at row 501 | Memory-safe; fast-fail on malicious large-N small-row CSVs | ✓ |
| Parse all then count | Simple but loads full body | |
| Spec-side `maxItems: 500` (JSON only) | Hybrid; CSV still needs manual counter | |

**User's choice:** Streaming count
**Notes:** Decision D5-22. JSON `json.Decoder` token-mode + CSV
`csv.Reader.Read` loop both increment row counter and exit on 501.

### Question 4.3 — JSON parse strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Streaming `json.Decoder` token-mode | O(1 row) memory; supports per-row error recovery | ✓ |
| `json.Unmarshal` eager | O(body) memory; simpler code | |

**User's choice:** Streaming
**Notes:** Decision D5-23. Pairs with D5-22 row-counting.

### Question 4.4 — Content-Type dispatch

| Option | Description | Selected |
|--------|-------------|----------|
| Header-only (strict, no fallback) | `application/json` → JSON; `text/csv` → CSV; else 415 | ✓ |
| Header + magic-byte fallback | Peek first byte if header missing/unknown | |
| Query param `?format=json|csv` | Override header; non-REST | |

**User's choice:** Header-only
**Notes:** Decision D5-24. Aligns with generated request object's
`JSONBody` (oapi-codegen-populated for `application/json`) vs `Body
io.Reader` (CSV) dispatch.

---

## Claude's Discretion

Captured in CONTEXT.md `<decisions>` § "Claude's Discretion":
- Exact internal package layout (`internal/import_pkg/` vs
  `internal/catalog/import/`).
- Goroutine sweep tick interval (recommend 1h).
- `import_jobs` migration filename ordering vs Phase 3.
- Whether `Import*Request` schemas live inline in `openapi.yaml` or in
  a separate components bundle.
- Test data file convention (must include
  `testdata/windows-excel-agents.csv` Excel-exported golden file).

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` section:
- Async import pathway (IMP-09)
- Dry-run import mode (IMP-10)
- Error CSV download (IMP-11)
- `succeeded_ids` JSONB for full idempotent-replay payload
- Skill REMOVAL via import (PATCH-style negative syntax)
- Multi-entity import in single POST
- Distributed lock for cleanup goroutine (v1 multi-replica)
- Reverse-proxy ingress body limit alignment
- OTel per-row spans
- Per-org rate limiting
- Schema_version v0.2 migration compatibility layer
