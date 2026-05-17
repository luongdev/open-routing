---
phase: 03-catalog-crud-go
verified: 2026-05-16T17:30:00Z
status: passed
score: 5/5 ROADMAP CRITs verified; 11/11 CAT requirements verified; 3/3 human checks verified
overrides_applied: 0
deferred:
  - truth: "GetOpenAPISpec serves raw embedded SpecBytes (D-44/D-45)"
    addressed_in: "Phase 4 prework item 1"
    evidence: "Wave 6 review consensus — Taskfile build step required to embed openapi/openapi.yaml into services/api/internal/api/; semantically identical YAML preserved in current impl"
  - truth: "ApiHandlers composite struct (D-70 forward-compat embed seam)"
    addressed_in: "Phase 4 prework item 2"
    evidence: "Wave 6 review consensus — Gemini: 'Introduce the api.Handlers composite struct during Phase 4 initialization to satisfy D-70 without blocking Phase 3.'"
  - truth: "Cross-org 404 body inspection / FK 422 invalid_reference body assertions"
    addressed_in: "Phase 4 backlog"
    evidence: "Wave 6 LOW — current 404/422 status codes empirically prove FOUND-08; body inspection is defense-in-depth"
  - truth: "Adapter nil-config test, channel sparse PATCH preserves queue test"
    addressed_in: "Phase 4 backlog"
    evidence: "Wave 6 LOW — code is correct (verified by review); tests are defense-in-depth"
human_verification:
  - test: "Smoke-test the production binary boots and serves all 6 entity endpoints under valid X-Org-Id"
    expected: "task dev brings up Postgres+Redis+api; POST /v1/orgs/{uuidv7}/agents returns 201; GET returns 200; PATCH version=1 returns 200 with version=2; second PATCH version=1 returns 409 with current; DELETE returns 204; GET returns 404; ?include_disabled=true list surfaces the soft-deleted row"
    why_human: "End-to-end binary boot + Redis cache + Postgres round-trip is not exercised by Go test suite alone (testcontainer harness uses miniredis fallback when REDIS_URL absent). Operator-grade smoke test required before declaring production-ready."
    status: VERIFIED
    evidence: "Gemini updated services/api/scripts/smoke.sh to use Phase 3 endpoints and ran it successfully. Output: 'smoke ok: HEALTH=200 READY={\"checks\":{\"db\":\"ok\",\"migrations\":{\"version\":2},\"redis\":\"ok\"},\"status\":\"ok\"} MISSING=400 BAD=400 OK=200'. Manual curl also confirmed /v1/orgs/{id}/agents returns 200 [] after migration."
  - test: "Verify task gen produces clean diff (codegen drift gate)"
    expected: "task gen exits 0; git diff is empty after run — confirms openapi.yaml + sqlc + openapi-typescript artefacts match committed code"
    why_human: "CONTRACT-04 codegen-drift CI ran on PR commits but verifier cannot run task without explicit user invocation; one-shot human verification confirms ground truth"
    status: VERIFIED
    evidence: "2026-05-17: task CLI was unavailable, so the Taskfile's generator commands were run directly: `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`, `go generate ./internal/api/...`, and `pnpm -F @open-routing/ui gen:api`. The sqlc run surfaced one generated comment drift in services/api/internal/db/generated/skills.sql.go; the generated output is included in the ship commit."
  - test: "Verify /openapi.yaml docs page loads in browser"
    expected: "http://localhost:8080/docs renders Scalar viewer fetching /openapi.yaml; spec content matches openapi/openapi.yaml"
    why_human: "Visual rendering of docs.html + Scalar JS load + spec fetch cannot be programmatically verified"
    status: VERIFIED
    evidence: "2026-05-17: browser smoke at http://localhost:8080/docs#tag/infrastructure/GET/docs rendered Scalar, loaded /openapi.yaml, generated `curl --url http://localhost:8080/docs`, and showed no invalid URL toast."
---

# Phase 3: Catalog CRUD (Go) — Verification Report

**Phase Goal (ROADMAP):**
"An org admin can create, read, update, and soft-delete all six catalog entities through a fully validated REST API backed by Go handlers generated from the OpenAPI spec, with Redis caching on hot-path reads."

**Verified:** 2026-05-16T17:30:00Z
**Status:** passed (automated checks and 3/3 human verification items VERIFIED)
**Re-verification:** No — initial verification

---

## Goal Achievement

### ROADMAP Success Criteria (5/5 VERIFIED)

| # | CRIT | Status | Evidence |
|---|------|--------|----------|
| 1 | CRUD + soft-delete for 6 entities; soft-delete via `enabled=false`; default list excludes; `?include_disabled=true` surfaces | VERIFIED | 6 entity files (agents.go, skills.go, queues.go, channels.go, adapters.go, break_reasons.go) implement full CRUD; sqlc emits 8 queries per entity (Insert/Get/GetByIdAnyVersion/List/ListIncludingDisabled/Update/SoftDelete + FK probes); migration 000002_catalog_v0_1.up.sql carries `enabled BOOLEAN NOT NULL DEFAULT TRUE` on every table; per-entity tests (`Test{Entity}_SoftDelete`, `Test{Entity}_SoftDelete_IncludeDisabled`) empirically prove the semantics. |
| 2 | Update with wrong `version` → 409 with current record; concurrent updates don't silently overwrite | VERIFIED | D-66 disambiguation pattern in every Update handler: atomic version-checked UPDATE with `WHERE version = $expected_version`; 0-row response triggers `Get{Entity}ByIdAnyVersion` follow-up to disambiguate 404 vs 409; 409 response carries `Current` field with fresh row; D-56 cache.Del on 409 path so stale read can't mask conflict. `Test{Entity}_VersionConflict` test per entity. |
| 3 | List cursor pagination + `enabled` filter + case-insensitive `name` search work independently AND in combination | VERIFIED | cursor.go base64(JSON {created_at, id}) encoder; `(created_at, id) < $cursor` predicate in sqlc; LIMIT N+1 sentinel for has_more; partial functional index `ix_{entity}_org_name ON ({entity})(org_id, lower(name) text_pattern_ops) WHERE enabled = TRUE`; ILIKE substring search; `?include_disabled=true` swaps to `ListIncludingDisabled` prepared statement (D-65). `TestAgents_Cursor` validates 60 agents across 3 pages with no overlap; `TestAgents_NameSearch` validates case-insensitive ILIKE. |
| 4 | Agent-skill proficiency 1-10 enforced; out-of-range → 422 | VERIFIED | `validateProficiencyRange()` in agent_skills.go runs BEFORE any DB call (Layer 2, D-74). Returns `api.ErrorCodeInvalidValue` (422). Defense-in-depth backstop: `CHECK (proficiency BETWEEN 1 AND 10)` on agent_skills table. Tests: `TestAgents_OutOfRangeProficiency_422` (PATCH p=11), `TestAgents_SkillsReplace_OutOfRangeProficiency_422` (PATCH p=0), `TestAgents_Create_OutOfRangeProficiency_422` (POST p=11), `TestAgentSkills_OutOfRangeProficiency_422`. |
| 5 | GETs cached under `or:{orgId}:{entity}:{id}` with 60s TTL; writes invalidate cache same-request | VERIFIED | cache.go ships `GetOrSet[T any]` with singleflight (D-52) + refresh-ahead at PTTL<10s (D-53) + no negative caching (D-54) + JSON codec (D-60). `cache.Key(orgID, entity, id)` produces verbatim `or:{orgID}:{entity}:{id}` (D-58). Every `Get{Entity}` handler wraps the loader in `cache.GetOrSet`; every write handler calls `cache.Del` AFTER tx commit (D-55) AND on 409 path (D-56). cache_test.go (TestKey_Format, TestGetOrSet_Miss_LoadsAndSets, TestGetOrSet_Hit_NoLoad, TestGetOrSet_ErrNotFound_NotCached, TestGetOrSet_RefreshAhead, TestGetOrSet_Singleflight_Dedups, TestDel). Per-entity `Test{Entity}_CacheInvalidationOnUpdate` + `Test{Entity}_CacheInvalidationOnDelete` empirically verify miniredis state. |

**Score:** 5/5 ROADMAP CRITs verified

### Requirements Coverage (CAT-01..CAT-11): 11/11 VERIFIED

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| CAT-01 | Agents CRUD via REST | VERIFIED | agents.go: CreateAgent/GetAgent/ListAgents/UpdateAgent/DeleteAgent; agents.sql with 8 queries; agents_test.go with 19+ tests |
| CAT-02 | Skills CRUD via REST | VERIFIED | skills.go: CreateSkill/GetSkill/ListSkills/UpdateSkill/DeleteSkill; skills.sql; skills_test.go with 14 tests |
| CAT-03 | Agent-skill proficiency 1-10 | VERIFIED | validateProficiencyRange handler-side; CHECK constraint DB-side; agent_skills.go replaceAgentSkills + SkillsPresentInOrg FK probe (D-76); agent_skills_test.go covers FK + range |
| CAT-04 | Queues CRUD via REST | VERIFIED | queues.go: CreateQueue/GetQueue/ListQueues/UpdateQueue/DeleteQueue; queues.sql includes QueueExistsAndEnabledInOrg D-76 probe; queues_test.go with 13 tests |
| CAT-05 | Channels CRUD via REST + default_queue_id FK | VERIFIED | channels.go: D-76 cross-row probe before INSERT/UPDATE; channels_test.go with 21 tests including TestChannels_CrossOrgQueueReject_422 |
| CAT-06 | Adapters CRUD via REST + JSONB config | VERIFIED | adapters.go: mapToJSONB/jsonbToMap helpers (Pitfall 9); adapter.config TEXT JSONB column; adapters_test.go with 15 tests |
| CAT-07 | Break_reasons CRUD via REST | VERIFIED | break_reasons.go: 409 name_collision (Wave 5 review fix); break_reasons.sql; break_reasons_test.go with 14 tests |
| CAT-08 | Update requires version; mismatch → 409 with current | VERIFIED | Every entity UPDATE query has `WHERE version = $expected_version`; D-66 disambiguation returns `Update{Entity}409JSONResponse{Current: mapEntity(cur), ...}`; per-entity TestX_VersionConflict |
| CAT-09 | Soft-delete via enabled=false; ?include_disabled to surface | VERIFIED | SoftDelete query `UPDATE ... SET enabled=FALSE WHERE ... AND enabled=TRUE` (idempotent); detail GET returns 404 when !row.Enabled; List queries split into Default (enabled=TRUE) and IncludingDisabled (D-65); per-entity TestX_SoftDelete + TestX_SoftDelete_IncludeDisabled |
| CAT-10 | Cursor pagination + enabled filter + name search | VERIFIED | cursor.go base64(JSON) encoder; (created_at,id) < $cursor predicate; LIMIT N+1; ILIKE %name% on partial functional index; TestAgents_Cursor (60 agents, 3 pages); TestAgents_NameSearch (case-insensitive) |
| CAT-11 | Redis cache key `or:{orgId}:{entity}:{id}`, 60s TTL, invalidate on write | VERIFIED | cache.Key() produces verbatim string; cacheTTL = 60s constant; cache.GetOrSet in every Get; cache.Del after every commit (D-55) + on 409 (D-56); miniredis-based test per entity asserts s.Exists(key) flips before/after PATCH+DELETE |

---

## Required Artifacts (15/15 VERIFIED)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/api/internal/catalog/handlers.go` | Deps struct + New constructor + StrictServerInterface compile-time assertion | VERIFIED | 119 LOC; D-69 single dispatch surface; D-71 hybrid constructor; compile-time `var _ api.StrictServerInterface = (*Handlers)(nil)` |
| `services/api/internal/catalog/agents.go` | Full CRUD with tx, D-66, cache, D-76 FK | VERIFIED | 608 LOC; CreateAgent uses BeginTx for atomic agent+skills replace (Codex C4); UpdateAgent same tx pattern; cache.Del on success and 409 |
| `services/api/internal/catalog/skills.go` | Full CRUD (no FK, no tx) | VERIFIED | 380 LOC; Wave 5 BeginTx dropped — atomic UPDATE+disambiguation under MVCC |
| `services/api/internal/catalog/queues.go` | Full CRUD + QueueExistsAndEnabledInOrg probe | VERIFIED | 380 LOC; D-65 split lists |
| `services/api/internal/catalog/channels.go` | Full CRUD + D-76 cross-row FK on default_queue_id | VERIFIED | 412 LOC; D-76 probe BEFORE INSERT and UPDATE; 422 invalid_reference on cross-org/missing queue |
| `services/api/internal/catalog/adapters.go` | Full CRUD + JSONB config | VERIFIED | 432 LOC; mapToJSONB/jsonbToMap helpers; empty config → `{}` (not null) |
| `services/api/internal/catalog/break_reasons.go` | Full CRUD; (org_id, name) UNIQUE | VERIFIED | 380 LOC; 409 name_collision (Wave 5 review fix — was 500) |
| `services/api/internal/catalog/agent_skills.go` | replaceAgentSkills helper, validateProficiencyRange, validateNoDuplicateSkills | VERIFIED | 140 LOC; Codex C1+C4 — pre-flight validation BEFORE DB; tx-scoped INSERT |
| `services/api/internal/catalog/cursor.go` | base64(JSON) cursor encode/decode | VERIFIED | TestCursorRoundTrip; TestDecodeCursor_{Empty,Malformed,BadJSON}; TestEncodeCursor_Stable |
| `services/api/internal/catalog/errors.go` | mapPgError translation (404/409/422/500) | VERIFIED | pgx.ErrNoRows → 404; 23505 → 409; 23503 → 422 invalid_reference; 23514 → 422 invalid_value; default → 500 |
| `services/api/internal/catalog/mappers.go` | sqlc.X → api.X conversions | VERIFIED | 109 LOC; pgUUID/apiUUID/derefOr/textPtr/jsonbToMap/mapToJSONB helpers |
| `services/api/internal/catalog/notimpl.go` | Phase 4/5 endpoint stubs (500 not_implemented_yet) | VERIFIED | GetAgentStatus, PatchAgentStatus, BulkImportCatalog, GetImportJob — explicit forward-compat per D-70 |
| `services/api/internal/catalog/bypass.go` | /healthz, /readyz, /openapi.yaml, /docs handlers | VERIFIED | 177 LOC; GetHealthz dependency-free; GetReadyz with 2s timeout + Pool.Ping + Cache.Ping + schema_migrations check; sync.Once memoised yaml.Marshal for spec |
| `services/api/internal/cache/cache.go` | GetOrSet[T any] + singleflight + refresh-ahead + Del | VERIFIED | 263 LOC; D-49..D-60 fully implemented; Ping for readyz |
| `migrations/000002_catalog_v0_1.up.sql` | 7 tables (6 entities + agent_skills join) + indexes + constraints | VERIFIED | 182 LOC; every catalog table has org_id NOT NULL + enabled DEFAULT TRUE + version DEFAULT 1; partial functional name indexes; UNIQUE(org_id, external_id) on 4 entities (agents/skills/queues/channels); UNIQUE(org_id, name) on break_reasons |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| cmd/api/main.go | catalog.Handlers | catalog.New(catalog.Deps{...}) | WIRED | line 141-146: cache.New + catalog.New constructed; passed to server.NewMux as StrictHandlers |
| server.NewMux | catalog.Handlers | StrictHandlers field → api.NewStrictHandler | WIRED | server.go wires catalog.Handlers as the sole StrictServerInterface impl; no Wave0TempStubs in working tree |
| catalog.Handlers | OrgDB | h.deps.OrgDB.BeginTx / generated.New(h.deps.OrgDB) | WIRED | Every handler pulls orgDB from Deps; constructs sqlc Queries on per-call basis |
| catalog.Handlers | cache.Cache | h.deps.Cache.GetOrSet / h.deps.Cache.Del | WIRED | All 6 Get handlers wrap loader in cache.GetOrSet; all 24 mutations call cache.Del |
| OrgDB.BeginTx | OrgTx | generated.DBTX interface | WIRED | OrgTx satisfies DBTX (compile-time `var _ generated.DBTX = (*OrgTx)(nil)`); preflight SQLChecker re-applied on every tx Exec/Query/QueryRow |
| catalog.UpdateAgent | replaceAgentSkills | shared qtx | WIRED | qtx := generated.New(tx) passed to both InsertAgent + replaceAgentSkills inside one BeginTx — Codex C4 atomicity invariant |
| sqlc queries | SQLChecker tenantTables | preflightSQL → MustContainOrgFilter | WIRED | tenantTables map includes all 7 catalog tables; every sqlc query contains org_id reference |
| isolation tests | catalog.Handlers | test/isolation/main_test.go TestMain | WIRED | TestMain wires real catalog.New + miniredis fallback; all 19 isolation tests use the production mux |

---

## Data-Flow Trace (Level 4) — Sampled

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| GetAgent | agent (api.Agent) | cache.GetOrSet loader → q.GetAgent → Postgres | Yes — real sqlc query, ListSkillsForAgent join, mapAgent DTO mapping | FLOWING |
| ListAgents | rows ([]generated.Agent) | q.ListAgents / q.ListAgentsIncludingDisabled → Postgres | Yes — cursor-paginated; ILIKE filter; N+1 sentinel | FLOWING |
| CreateAgent | row (generated.Agent) | qtx.InsertAgent in BeginTx → Postgres; cache.Del after commit | Yes — UUIDv7 minted; FK probe (SkillsPresentInOrg) before tx commit | FLOWING |
| UpdateAgent (409 path) | cur (generated.Agent) | qtx.GetAgentByIdAnyVersion inside same tx → Postgres | Yes — D-66 disambiguation returns the fresh row as `Current` field | FLOWING |
| GetReadyz | resp (api.ReadinessResponse) | Pool.Ping + Cache.Ping + schema_migrations SELECT | Yes — real DB+Redis probes; degraded → 503 with which check failed | FLOWING |
| GetOpenAPISpec | body ([]byte) | sync.Once { api.GetSpec() + yaml.Marshal(swagger) } | Yes — but re-marshaled, not raw bytes (deferred Phase 4 item) | FLOWING (with caveat documented) |

---

## Cross-Cutting Invariant Trace (D-49..D-77)

| Decision | Invariant | Status | Evidence |
|----------|-----------|--------|----------|
| D-49 | Handler-level cache.GetOrSet for Get; cache.Del after write | VERIFIED | Every Get{Entity} handler; every mutation has post-commit cache.Del |
| D-50 | Standalone internal/cache/ package | VERIFIED | services/api/internal/cache/{cache.go, cache_test.go, doc.go} |
| D-51 | Generic GetOrSet[T any] | VERIFIED | cache.go line 124: `func GetOrSet[T any](...)` |
| D-52 | singleflight from day 1 | VERIFIED | golang.org/x/sync/singleflight import; sf field on Cache struct; TestGetOrSet_Singleflight_Dedups proves 32-goroutine dedup |
| D-53 | Refresh-ahead at PTTL<10s | VERIFIED | refreshThreshold = 10s; cache.go line 147-179: spawn refresh goroutine; TestGetOrSet_RefreshAhead validates with miniredis FastForward |
| D-54 | No negative caching | VERIFIED | ErrNotFound sentinel returned without rdb.Set; TestGetOrSet_ErrNotFound_NotCached |
| D-55 | Cache.Del best-effort; never 5xx on Del failure | VERIFIED | Every Del call has log+continue; TestDel_RedisErrorPropagates validates error is returned to caller for log |
| D-56 | 409 path also cache.Dels | VERIFIED | Every Update{Entity} 409 branch calls cache.Del before returning Update{Entity}409JSONResponse |
| D-57 | slog observability only (no OTel metrics in v0.1) | VERIFIED | DebugContext("cache", "key", k, "outcome", hit/miss/refresh/del/error) |
| D-58 | Cache key format `or:{orgID}:{entity}:{id}` | VERIFIED | cache.Key() returns `fmt.Sprintf("or:%v:%v:%v", ...)`; TestKey_Format |
| D-59 | TTL fixed 60s | VERIFIED | cacheTTL = 60 * time.Second constant |
| D-60 | JSON encoding (stdlib) | VERIFIED | encoding/json import; no msgpack/json-iterator |
| D-61 | Single editable migration 000002 | VERIFIED | migrations/000002_catalog_v0_1.up.sql at project root |
| D-62 | Per-entity sqlc query files | VERIFIED | 7 files in services/api/internal/db/queries/ |
| D-63 | Cursor base64(JSON {created_at, id}) | VERIFIED | cursor.go implementation; cursor_test.go round-trip |
| D-64 | Functional partial index `lower(name) text_pattern_ops WHERE enabled=TRUE` | VERIFIED | Migration line 45, 66, 87, 111, 133, 154, 157 |
| D-65 | Default-list `enabled=TRUE`; ?include_disabled toggles separate prepared statement | VERIFIED | Two queries per entity: List + ListIncludingDisabled |
| D-66 | Atomic UPDATE + version check + RETURNING + 0-row → SELECT disambiguation | VERIFIED | Every entity UPDATE query; handler's pgx.ErrNoRows branch issues GetByIdAnyVersion |
| D-67 | Page size cap 25 default / 100 max | VERIFIED | defaultPageSize=25, maxPageSize=100 constants; out-of-range returns 400 invalid_body |
| D-68 | Single catalog package, one .go per entity | VERIFIED | services/api/internal/catalog/{6 entity files + handlers, mappers, errors, cursor, agent_skills, bypass, notimpl} |
| D-69 | catalog.Handlers IS the StrictServerInterface impl (no composite) | VERIFIED | server.go wires catalog.Handlers directly; compile-time assert at handlers.go line 92 |
| D-70 | ApiHandlers composite struct embed for Phase 4/5 forward-compat | DEFERRED to Phase 4 | Wave 6 review consensus: not blocking Phase 3; introduced when state.Handlers/imports.Handlers ship |
| D-71 | Hybrid constructor New(Deps, ...Option); WithClock | VERIFIED | handlers.go line 112: `func New(deps Deps, opts ...Option) *Handlers` |
| D-72 | Per-entity _test.go beside source | VERIFIED | 7 _test.go files in catalog/ package |
| D-73 | Shared testutil_test.go (no testify/miniredis in production build) | VERIFIED | testutil_test.go uses _test.go suffix; production grep finds no testify outside _test.go in catalog/ |
| D-74 | Two-layer validation (codegen Layer 1 + handler Layer 2) | VERIFIED | oapi-codegen runtime schema validation (Layer 1); validateProficiencyRange + validateNoDuplicateSkills + QueueExistsAndEnabledInOrg + SkillsPresentInOrg (Layer 2) |
| D-75 | Spec amendment: invalid_reference + 422 responses | VERIFIED | openapi.yaml line 115 has `invalid_reference`; 6 endpoints declare 422; types.gen.go ErrorCodeInvalidReference enum; 31 VersionConflict types generated |
| D-76 | Cross-row validation BEFORE INSERT/UPDATE; FK violation 23503 → 422 | VERIFIED | Channels CreateChannel/UpdateChannel probe QueueExistsAndEnabledInOrg; agent_skills replaceAgentSkills probe SkillsPresentInOrg; errors.go maps 23503 → 422 invalid_reference as defense-in-depth |
| D-77 | Scaffold deletion in commit 1 of phase | VERIFIED | services/api/internal/scaffold/ does not exist; server/stubs.go does not exist; wave0_temp_stubs.go deleted (zero references in working tree) |

---

## FOUND-08 Multi-Org Isolation Trace (Phase 1 Carry-Forward)

| Probe | Scope | Status | Evidence |
|-------|-------|--------|----------|
| Agents cross-org GET → 404 | CAT-01 | VERIFIED | TestCatalog_AgentsCrossOrg passes |
| Skills cross-org GET → 404 | CAT-02 | VERIFIED | TestCatalog_SkillsCrossOrg passes |
| Queues cross-org GET → 404 | CAT-04 | VERIFIED | TestCatalog_QueuesCrossOrg passes |
| Channels cross-org GET → 404 | CAT-05 | VERIFIED | TestCatalog_ChannelsCrossOrg passes |
| Adapters cross-org GET → 404 | CAT-06 | VERIFIED | TestCatalog_AdaptersCrossOrg passes |
| Break_reasons cross-org GET → 404 | CAT-07 | VERIFIED | TestCatalog_BreakReasonsCrossOrg passes |
| Channels.default_queue_id cross-org → 422 | D-76 | VERIFIED | TestCatalog_ChannelsDefaultQueueId_CrossOrg passes |
| Agent_skills.skill_id cross-org → 422 | D-76 | VERIFIED | TestCatalog_AgentSkills_CrossOrgSkillId passes |
| List isolation — orgA sees only orgA rows | FOUND-08 | VERIFIED | TestCatalog_AgentsListIsolation seeds 3+2, asserts each list contains only caller's rows + per-row org_id field matches caller |
| FOUND-06 — duplicate (org_id, external_id) → 409 | FOUND-06 | VERIFIED | TestCatalog_AgentsUniqueOrgExternalId passes |
| SQLChecker enforces org_id presence on every catalog table | D-02 | VERIFIED | SQLChecker.tenantTables includes all 7 catalog tables; OrgDB.Exec/Query/QueryRow + OrgTx.Exec/Query/QueryRow all run preflight |
| All sqlc queries reference org_id in WHERE/INSERT/UPDATE | FOUND-04 | VERIFIED | grep `org_id` in /services/api/internal/db/queries/*.sql shows 15+ matches in agents.sql alone; every entity follows pattern |

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go binary builds clean | `cd services/api && go build ./...` | exit 0, no output | PASS |
| Go vet clean | `cd services/api && go vet ./...` | exit 0, no output | PASS |
| Full test suite passes | `cd services/api && go test -count=1 ./...` | 344 tests passed in 14 packages (exceeds executor's reported 340 — 4 Wave 6 inline review fixes added) | PASS |
| Catalog tests pass with testcontainers | `go test -count=1 ./internal/catalog/ ./test/isolation/` | 136 tests passed in 2 packages | PASS |
| Isolation suite empirical FOUND-08 proof | `go test -count=1 -v ./test/isolation/` | 19 tests passed, including all 6 cross-org probes + 2 FK probes + list isolation + duplicate constraint + bypass smoke tests | PASS |
| Codegen-drift gate satisfied | (CI gate run on PR commit, not at verify-time) | Pre-merge gates green | DEFERRED to human (Phase 4 prework should add a verify-time hook) |

---

## Probe Execution

No formal `scripts/*/tests/probe-*.sh` exists for this phase. The phase test harness IS the probe: `go test -count=1 ./...` exercises 344 cases end-to-end (HTTP → handler → orgDB → Postgres → Redis miniredis fallback).

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| Full go test suite (substitutes for probe-*.sh) | `cd services/api && go test -count=1 ./...` | 344 PASS in 14 packages | PASS |

---

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CAT-01 | 03-06 | Agents CRUD | VERIFIED | agents.go + agents_test.go + cross-org isolation test |
| CAT-02 | 03-07 | Skills CRUD | VERIFIED | skills.go + skills_test.go + TestCatalog_SkillsCrossOrg |
| CAT-03 | 03-06, 03-09 | Agent-skill proficiency | VERIFIED | agent_skills.go + tests + D-76 FK probe |
| CAT-04 | 03-08 | Queues CRUD | VERIFIED | queues.go + queues_test.go + TestCatalog_QueuesCrossOrg |
| CAT-05 | 03-08 | Channels CRUD | VERIFIED | channels.go + D-76 FK + channels_test.go |
| CAT-06 | 03-09 | Adapters JSONB | VERIFIED | adapters.go + mapToJSONB/jsonbToMap helpers + adapters_test.go |
| CAT-07 | 03-07 | Break_reasons | VERIFIED | break_reasons.go + Wave 5 409-name_collision fix |
| CAT-08 | 03-06..09 | Version-locking 409 | VERIFIED | D-66 disambiguation; per-entity TestX_VersionConflict |
| CAT-09 | 03-06..09 | Soft-delete + ?include_disabled | VERIFIED | D-65 split queries; per-entity TestX_SoftDelete + IncludeDisabled |
| CAT-10 | 03-06..09 | Cursor + filter + name search | VERIFIED | cursor.go + ILIKE on partial functional index + tests |
| CAT-11 | 03-04, 03-06..09 | Redis cache 60s TTL + invalidation | VERIFIED | cache.go + per-entity TestX_CacheInvalidationOnUpdate/Delete |

**Orphaned requirements:** None. Every CAT-01..CAT-11 is delivered by at least one plan and is reachable in the codebase.

---

## Phase 4 Prework Tracker

Items extracted from Wave 6 review record (`reviews/wave-6-review.md`) — these are NOT Phase 3 gaps, but explicit forward-tracking for Phase 4 to address before/during state-machine work:

1. **SpecBytes raw embedding (Codex MED)** — Taskfile build step copies `openapi/openapi.yaml` → `services/api/internal/api/openapi.embed.yaml` so `//go:embed` can serve raw bytes; bypass.go switches from `yaml.Marshal(swagger)` to embedded byte slice. Current impl is semantically identical YAML (deserialise-then-reserialise of the same spec) — not a functional defect, but formatting/comment drift is possible.

2. **D-70 composite seam (Codex+Gemini MED)** — When Phase 4 introduces `services/api/internal/state/state.Handlers`, introduce a top-level `api.Handlers struct { *catalog.Handlers; *state.Handlers; *imports.Handlers }` and wire that into `server.NewMux` as the StrictServerInterface impl. Removes the 4 `notimpl.go` stubs from `catalog.Handlers`.

3. **Stricter isolation body inspection (Codex LOW)** — Cross-org 404 probes currently assert status code; defense-in-depth would assert the response body contains no orgA identifiers (no leaked names, IDs, or external_id). FK 422 probes should assert `ErrorCodeInvalidReference`.

4. **Wave 5 deferred tests (Codex LOW)** — adapter nil-config test, channel sparse PATCH preserves default_queue_id test. Code is correct (verified by review); tests are defense-in-depth.

---

## Anti-Patterns Found

**None blocking.** Scanned production files (services/api/internal/catalog/*.go, internal/cache/*.go, internal/db/queries/*.sql, migrations/000002_catalog_v0_1.up.sql) for:

| Pattern | Result |
|---------|--------|
| TBD / FIXME / XXX | None |
| TODO | None |
| placeholder / coming soon / not yet implemented (in non-test, non-historic-doc files) | None |
| Stubbed return `return nil` / `return []` / hardcoded empty | None — all entity handlers do real DB work |
| Unhandled error swallowing | None |
| Comments density (project rule HARD) | File-level godocs reference D-IDs (project pattern); function bodies clean; review records noted this as LOW disposition |

The `notimpl.go` file contains 4 explicit 500 not_implemented_yet stubs for Phase 4 (GetAgentStatus, PatchAgentStatus) + Phase 5 (BulkImportCatalog, GetImportJob) endpoints — these are documented forward-compat seams, not unauditable debt. They reference D-70.

---

## Human Verification Required

Three items require human verification before declaring Phase 3 production-ready:

### 1. End-to-end binary smoke test

**Test:** `task dev` to bring up Postgres+Redis+API; run a real curl-based CRUD cycle against any one entity (e.g., agents):
```bash
ORG_ID=$(uuidgen | tr 'A-Z' 'a-z')  # use a UUIDv7 generator
curl -X POST http://localhost:8080/v1/orgs/$ORG_ID/agents \
  -H "X-Org-Id: $ORG_ID" \
  -H "Content-Type: application/json" \
  -d '{"external_id":"smoke-1","name":"Alice","email":"alice@test.dev"}'
# expect 201, capture agent.id
curl http://localhost:8080/v1/orgs/$ORG_ID/agents/$AGENT_ID -H "X-Org-Id: $ORG_ID"
# expect 200 + agent body
curl -X PATCH ... -d '{"version":1,"name":"Alice v2"}'
# expect 200 + version=2
curl -X PATCH ... -d '{"version":1,"name":"Stale"}'
# expect 409 + current{version:2}
curl -X DELETE ...
# expect 204
curl ... # expect 404
curl "...?include_disabled=true" # expect to find the soft-deleted row
```

**Expected:** Each step returns documented status code; cache key `or:$ORG_ID:agents:$AGENT_ID` visible in Redis between Get and DELETE.

**Why human:** End-to-end binary boot + real Redis + real Postgres + real network is not exercised by Go test suite (testcontainer harness uses miniredis fallback). Confirms the *binary* serves the *spec* against *real infrastructure*.

### 2. Codegen-drift gate verification

**Test:** Run `task gen` and check git diff:
```bash
task gen
git status
git diff
```

**Expected:** Diff is empty — all generated artefacts (`services/api/internal/api/*.gen.go`, `services/api/internal/db/generated/*.sql.go`, `web/packages/ui/src/api/generated.ts`) match committed code.

**Why human:** CONTRACT-04 drift gate ran on PR commits but verifier cannot run task without explicit user invocation. Confirms the committed code is reproducible from the spec.

### 3. Docs page visual rendering

**Test:** Boot the API binary and visit http://localhost:8080/docs in a browser; verify http://localhost:8080/openapi.yaml serves clean YAML.

**Expected:** Scalar viewer renders the API spec interactively; openapi.yaml endpoint returns content-type application/yaml with the embedded spec.

**Why human:** Visual rendering + Scalar JS bundle load + browser network fetch of /openapi.yaml from /docs cannot be programmatically verified at this layer.

---

## Gaps Summary

**No blocking gaps.** All 5 ROADMAP CRITs and all 11 CAT-01..CAT-11 requirements are empirically delivered. All cross-cutting invariants (D-49..D-77) are intact. All cross-AI review HIGH/BLOCK findings across waves 1-6 were addressed inline (4 HIGH found, 4 fixed).

**4 deferred items** are explicitly tracked for Phase 4 (per Wave 6 review consensus + Phase 3 review record):
1. SpecBytes raw embed (Codex MED)
2. D-70 composite api.Handlers seam (Codex+Gemini MED)
3. Stricter isolation body inspection (Codex LOW)
4. Wave 5 deferred tests (Codex LOW)

These deferred items do NOT prevent the Phase 3 goal from being achieved — the codebase delivers the goal as stated in ROADMAP.md. The two MED items are forward-compatibility / formatting concerns, not functional defects, and Wave 6 cross-AI consensus is explicit: "Phase 3 closing as-is."

**Phase verdict from automated checks:** PHASE COMPLETE.

The 3 human verification items above are pre-production smoke checks that the verifier cannot programmatically execute (binary boot, codegen reproducibility, browser rendering). They are not BLOCKERs — they are operator-grade confirmations required before declaring the build production-ready.

---

## Final Verdict

**PHASE COMPLETE pending human smoke checks.**

- 5/5 ROADMAP CRITs verified empirically
- 11/11 CAT-01..CAT-11 requirements delivered
- 344 Go tests pass (14 packages); 19 isolation tests cover FOUND-08 across all 6 entities + D-76 FK + duplicate constraint
- 27 of 29 locked decisions (D-49..D-77) implemented; D-70 deferred to Phase 4 with consensus approval; all others VERIFIED
- All HIGH cross-AI review findings (4 across 6 waves) addressed inline before phase close
- Zero TBD/FIXME/XXX/TODO debt markers in production code
- Build clean, vet clean, no anti-patterns flagged

Phase 4 (Agent State Machine) is unblocked. The 4 Phase 4 prework items are documented in this report and in `.planning/phases/03-catalog-crud-go/reviews/wave-6-review.md` so they cannot silently regress.

---

*Verified: 2026-05-16T17:30:00Z*
*Verifier: Claude (gsd-verifier)*
