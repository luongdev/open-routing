# Phase 2: OpenAPI Contract & Codegen - Context

**Gathered:** 2026-05-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish the OpenAPI 3.1 contract authoring + codegen pipeline so Phases 3–5 implement against a single source of truth. Phase 2 delivers the **complete v0.1 API surface** in `openapi/openapi.yaml` (every catalog CRUD endpoint + agent state transitions + bulk import + scaffold/health), the Go server stubs via oapi-codegen under `services/api/internal/api/`, the TypeScript client via openapi-typescript + openapi-fetch distributed through `web/packages/ui`, and a CI drift gate that fails the build whenever committed generated code diverges from the spec.

Before authoring the spec, Phase 2 produces **ASCII paper-sketches of 4 high-complexity admin screens** (Agent detail/edit, Bulk import result, Agent status panel, Catalog list) in `02-UI-SKETCHES.md`. These sketches exist to validate API response shapes (embedded vs separate calls, error envelopes, pagination cursors) BEFORE the spec is written — they are NOT UI implementation.

**In scope:** `openapi/openapi.yaml` with every v0.1 endpoint specified; `services/api/internal/api/` generated strict-server stubs (decoders, validators, typed request/response objects); `web/packages/ui/src/api/` TS distribution layer (factory client + types + error helpers + Lit async helper); `GET /openapi.yaml` + `GET /docs` runtime endpoints on the Go API; CI drift gate (`go generate ./...` + `pnpm gen:api` produce zero diff against committed code); paper sketches of 4 screens.

**Out of scope:** real catalog handlers (Phase 3 implements against generated stubs); the agent state machine logic (Phase 4); bulk import endpoint logic (Phase 5); admin SPA screens (Phase 6 builds real UI on top of `packages/ui`); the Web Component embed bundle (Phase 7); real auth on `/docs` (stub auth in v0.1); `golangci-lint` rules tuning; sqlc query authoring for new tables.

</domain>

<decisions>
## Implementation Decisions

### Spec Scope + UI Validation Pass

- **D-32:** Phase 2 ships the **full v0.1 OpenAPI spec** — every endpoint that Phases 3–5 will implement is defined upfront. Phases 3–5 author handler bodies against generated stubs; they do NOT add new paths to `openapi.yaml`. Rationale: two UI consumers (admin SPA + Web Component embed) need contract stability; backend phases can run in parallel against a frozen contract; drift gate has the entire surface to enforce from Phase 2 forward.
- **D-33:** Before authoring `openapi.yaml`, Phase 2 produces **ASCII markdown paper-sketches** of 4 high-complexity admin screens in `.planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md`:
  1. **Agent detail/edit** — validates N:M `agent_skills` shape with `proficiency` (embedded array on agent detail vs separate `GET /agents/{id}/skills` call)
  2. **Bulk import result** — validates HTTP 207 multi-status shape, failed-rows array (IMP-04, IMP-05)
  3. **Agent status panel** — validates state-transition request body, `engaged_channel` parameter, WrapUp TTL display, `post_interaction_state` semantics (STATE-02 to STATE-07)
  4. **Catalog list (generic)** — validates cursor-based pagination shape, `?include_disabled=true`, case-insensitive `name` search response (CAT-09, CAT-10)
  Each sketch is ~30–50 lines: inputs, list columns, error states, loading states. Sketches are **contract validation artifacts**, not UI implementation — Phase 6 owns real UI work.
- **D-34:** Sketches MUST be authored and reviewed BEFORE the OpenAPI spec is committed. If a sketch reveals a shape problem (e.g., flat skills array on agent detail is awkward for list view), the spec adjusts before being locked. Planner sequences this as Wave 1: sketches; Wave 2: spec + codegen + CI.

### Error Envelope (Spec-Wide Lock)

- **D-35:** Canonical error envelope across the whole spec carries forward Phase 1's shape: `{ "error": "<code>", "reason": "<detail>", "request_id": "<uuidv7>" }`. The existing `middleware.WriteError(w, status, error, reason)` helper at `services/api/internal/middleware/` is extended to embed the `request_id` from ctx (already minted by D-28 RequestID middleware) into every response body. NO migration to RFC 7807 Problem Details — keeps Phase 1 helper, avoids `application/problem+json` content-type churn.
- **D-36:** The `error` field is a **closed enum** in `openapi.yaml` `components.schemas.ErrorCode`. Initial v0.1 enum (locked in Phase 2):
  - `invalid_body` — malformed JSON, missing required fields, type mismatch
  - `invalid_id` — UUID parse failure, non-UUIDv7 ID
  - `not_found` — resource missing OR cross-org probe (FOUND-08 leakage guard returns 404, never 200)
  - `internal` — uncaught error (also covers infrastructure failures)
  - `version_conflict` — CAT-08 optimistic concurrency miss
  - `cross_org` — orgDB preflight rejection (defensive; should never reach client in correct flow)
  - `missing_org_id` — middleware rejection when `X-Org-Id` is absent or malformed
  - `invalid_transition` — STATE-03 agent-state transition not in allowed matrix
  - `import_failed` — IMP-04 row-level import failure (used inside the `failed[]` array of 207)
  - `rate_limited` — reserved for v0.2; specced now so codegen doesn't need rework
- **D-37:** Special-case response shapes (REQUIREMENTS.md-mandated) use the canonical envelope **plus extension fields**, not separate schemas:
  - **CAT-08 (HTTP 409 version mismatch):** `{ error: "version_conflict", reason, request_id, current: <EntitySchema> }` — the `current` field carries the server-side record so client can show diff/merge UI.
  - **STATE-03 (HTTP 409 invalid transition):** `{ error: "invalid_transition", reason, request_id, from: <AgentState>, to: <AgentState> }` — from/to are the requested transition pair.
  - **IMP-05 (HTTP 207 multi-status):** NOT an error envelope — separate schema `BulkImportResult` with `{ succeeded: [<EntityId>], failed: [{ row: int, field: string, error: "import_failed", reason: string }] }`. The `failed[]` items use the error envelope shape internally for consistency, but the top-level response is its own schema.

### TypeScript Client Distribution

- **D-38:** `packages/ui` exposes a **factory-pattern** client (not a singleton) so admin SPA and Web Component embed can each supply their own `org_id` resolution strategy:
  ```ts
  // web/packages/ui/src/api/client.ts
  export function createApiClient(config: {
    baseURL: string;            // e.g., "https://api.open-routing.io" or relative ""
    getOrgId: () => string;     // admin reads env/login; embed reads Custom Element attribute
    fetch?: typeof fetch;       // override for testing
  }): ApiClient;
  ```
  `getOrgId` is invoked per request (not at construction time) so an embed instance can react to attribute changes without recreating the client. The returned client adds `X-Org-Id` to every request automatically; routes that bypass org middleware (`/healthz`, `/readyz`, `/metrics`, `/openapi.yaml`, `/docs`) are not exposed through this client (they're not in the typed `paths`).
- **D-39:** `packages/ui/src/api/` ships **all three layers**:
  1. **Generated types** — `openapi-typescript` output at `generated.ts` (committed; CI drift gate enforces no-diff against re-running `pnpm gen:api`).
  2. **openapi-fetch client + factory** — `createApiClient`, plus barrel-exports of `paths`, `components.schemas`, `components.responses` from the generated types.
  3. **Error helpers** — `isApiError(e): e is ApiError` type guard, `parseApiError(response): Promise<ApiError | null>` for response → typed error parsing, and `ErrorCodes` (a `const`-asserted record of the closed enum values so consumers get autocomplete: `ErrorCodes.VERSION_CONFLICT`).
  4. **Lit-aware async helper** — wraps `@lit/task` for the common loading/error/data pattern: `createApiTask(client, paramsResolver, request)` returns a `Task` that admin SPA components can `subscribe()` to. Adds `@lit/task` as a `packages/ui` dep (already in the locked Lit ecosystem per PROJECT.md), no other new deps. Phase 7 embed can opt out by importing only the `createApiClient` + types and skipping the helper.
- **D-40:** Module layout:
  ```
  web/packages/ui/src/api/
  ├── generated.ts        # openapi-typescript output (committed, drift-gated)
  ├── client.ts           # createApiClient factory
  ├── errors.ts           # isApiError, parseApiError, ErrorCodes
  ├── task.ts             # createApiTask Lit-aware helper
  └── index.ts            # barrel re-export of everything above + paths/components types
  ```
  Re-exported via `packages/ui` root `index.ts` so consumers write `import { createApiClient } from '@open-routing/ui'`.

### oapi-codegen Server Flavor + Generated Output

- **D-41:** oapi-codegen runs in **strict-server mode** (`--generate strict-server,types,spec`). Generated handlers are pure functions:
  ```go
  type StrictServerInterface interface {
      CreateAgent(ctx context.Context, req CreateAgentRequestObject) (CreateAgentResponseObject, error)
      GetAgent(ctx context.Context, req GetAgentRequestObject)       (GetAgentResponseObject, error)
      // ... per endpoint
  }
  ```
  Codegen lays request decoding, body validation, and response marshaling. Handlers focus on business logic. Compile-time enforcement: if `openapi.yaml` changes a response shape, every affected handler's signature changes and the build fails — drift cannot reach runtime.
- **D-42:** Generated code lives under `services/api/internal/api/` (matches CONTRACT-02). File layout:
  ```
  services/api/internal/api/
  ├── types.gen.go        # request/response structs from components.schemas
  ├── server.gen.go       # StrictServerInterface, request/response objects, chi route wiring
  └── spec.gen.go         # embedded openapi.yaml bytes (oapi-codegen --generate spec)
  ```
  Single package `api` — no per-tag splitting in v0.1 (~30 endpoints fits comfortably in one package; revisit if it grows past ~80 endpoints in v0.2).
- **D-43:** Driver: `go generate ./...` runs from `services/api/` and invokes oapi-codegen with config pinned at `services/api/internal/api/oapi-codegen.yaml`. The Taskfile target `task gen` (introduced in D-08) runs both Go codegen and `pnpm -F @open-routing/ui gen:api` from one command. Config files committed; tool versions pinned in `tools.go` (Go) and `package.json` (TS).
- **D-44:** Handlers integrate via a thin adapter at `services/api/internal/server/server.go`: the existing `NewMux` accepts a new `StrictServerHandlers` arg and wires the generated routes inside the `/v1` sub-router (preserving D-21 bypass list at root). Phase 3 implements the concrete handlers; Phase 2 only proves the wiring with **scaffold migration** (see D-46) or a placeholder handler that returns `{error: "not_implemented"}`.

### /openapi.yaml + /docs Runtime Serving

- **D-45:** API binary serves the spec and an interactive viewer at root level, both bypassing org middleware (D-21 already-bypassed list extended):
  - `GET /openapi.yaml` — returns the embedded spec bytes from `spec.gen.go` (always exactly the spec the running binary was built against; no risk of stale file). Content-Type `application/yaml`.
  - `GET /docs` — returns a static HTML page that loads a CDN-hosted interactive viewer (Scalar API Reference or RapiDoc — planner picks; both are single-script HTML + CDN). The HTML fetches `/openapi.yaml` from the same origin. No Go HTML templates, no extra static assets in the repo — just a ~40-line `index.html` baked into a `//go:embed` string.
  - In v0.1 with stub auth, both routes are public. In v1 with real auth, `/docs` will be gated behind admin role (deferred).

### Phase 1 Scaffold Migration

- **D-46:** Phase 2 migrates the existing `_scaffold` HTTP handlers (at `services/api/internal/scaffold/handler.go`) to be the **first concrete consumer of the generated strict-server interface**. The scaffold routes are added to `openapi.yaml` as the `Scaffold` tag, codegen produces `ScaffoldServerInterface`, and `scaffold.handler` implements it. The hand-written `(w, r)` handlers are replaced with strict-server handlers. Rationale: proves the codegen pipeline end-to-end against working code (FOUND-08 isolation test must still pass after the migration), avoids shipping a never-exercised stub. Phase 3's first migration deletes `_scaffold` and its OpenAPI paths — same as planned in D-18.

### CI Drift Gate

- **D-47:** A dedicated `codegen-drift` job runs on every PR. Steps: `task gen` (runs Go codegen + TS codegen) → `git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts`. Non-empty diff fails the job. The drift gate runs separately from `go-test` / `go-lint` / `web-typecheck` so a drift failure shows up as its own red check (clearer signal than buried inside another job). Job depends on Go + Node both being installed; reuses the existing `actions/setup-go@v5` and `pnpm/action-setup` blocks.
- **D-48:** Spec linting runs as part of `codegen-drift` via `npx @redocly/cli@latest lint openapi/openapi.yaml --extends recommended` BEFORE codegen. A spec that fails linting blocks the PR before drift can be evaluated. Redocly CLI is `npx`-invoked (no `package.json` dep) to keep the lint surface decoupled from the TS workspace.

### Claude's Discretion

- Exact paper-sketch HTML/wireframe character set (ASCII art convention) — planner picks the convention; consistency across the 4 screens matters more than the specific character set.
- Whether to use Scalar API Reference or RapiDoc at `/docs` — both are single-script HTML+CDN with ~equivalent UX. Planner picks based on bundle size and license. Default recommendation: Scalar (newer, better default styling, MIT).
- The `oapi-codegen.yaml` config flag set beyond `--generate strict-server,types,spec` — planner tunes `output-options.skip-prune` and `compatibility.always-prefix-enum-values` based on first-pass output.
- Whether `parseApiError` should retry-on-network-error or strictly parse — default to strict parse; network errors are caller's concern.
- The exact CDN URL pinning for Scalar/RapiDoc — pin to a specific version hash, not `@latest`; planner picks.
- Whether `generated.ts` lives at `web/packages/ui/src/api/generated.ts` (chosen) or `web/packages/ui/src/api/__generated__/index.ts` — naming convention only.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level locks
- `.planning/PROJECT.md` — Locked stack: OpenAPI 3.1 → oapi-codegen (Go) + openapi-typescript (TS). UI: Vite + Lit + Shoelace + TS. Embedding: Web Components. Polyglot monorepo with `services/api/` Go module + `web/` pnpm workspace.
- `.planning/REQUIREMENTS.md` §API Contract (OpenAPI) — **CONTRACT-01 through CONTRACT-04**. The acceptance criteria for this phase.
- `.planning/REQUIREMENTS.md` §Catalog CRUD — **CAT-01 through CAT-11**. These endpoints MUST be specced in `openapi.yaml` even though Phase 3 implements them. Note CAT-08 (version mismatch HTTP 409 with `current`) and CAT-10 (cursor pagination + filtering + name search) for special response shapes.
- `.planning/REQUIREMENTS.md` §Agent State Model — **STATE-01 through STATE-10**. State-transition endpoint specs go in `openapi.yaml`. STATE-03 (HTTP 409 with from/to) is the canonical extension-field example.
- `.planning/REQUIREMENTS.md` §Bulk Import — **IMP-01 through IMP-08**. The 207 multi-status `BulkImportResult` schema lives in `openapi.yaml` per D-37. IMP-07 size cap (50 MB / 500 rows) shapes `requestBody` constraints.
- `.planning/ROADMAP.md` §Phase 2 — Goal statement and 4 success criteria (spec lints in CI, `go generate ./...` regenerates Go, `pnpm gen:api` regenerates TS, drift gate fails build).
- `.planning/STATE.md` §Accumulated Context — Phase 1 closeout status; carries forward locked stack decisions.

### Phase 1 carry-forward (LOCKED contracts — do not rewrite)
- `.planning/phases/01-foundation-polyglot-monorepo/01-CONTEXT.md` — **All D-01 through D-31 are locked.** Especially:
  - **D-17, D-21:** Bypass list `{/healthz, /readyz, /metrics}` — Phase 2 extends with `/openapi.yaml`, `/docs`. Org middleware applies to everything else.
  - **D-19, D-20:** UUIDv7+ everywhere; org_id parsed as `uuid.UUID` from `X-Org-Id` header. OpenAPI spec MUST declare these constraints (path params + header schema).
  - **D-23:** Internal package layout — `internal/api/` is the home for generated code (matches CONTRACT-02 + Phase 2 D-42).
  - **D-28, D-29:** RequestID middleware mints UUIDv7 and writes `X-Request-Id` header; slog handler injects `trace_id`/`span_id`. Phase 2 D-35 embeds `request_id` in error body from the same ctx value.
  - **D-30:** GitHub Actions CI shape — Phase 2 adds a new `codegen-drift` job to this workflow.
- `services/api/internal/middleware/` (response.go) — `WriteJSON`, `WriteError(w, status, error, reason)` is the helper Phase 2 extends to embed `request_id` from ctx.
- `services/api/internal/scaffold/handler.go` — The handlers that get migrated to strict-server (D-46). Reference for the existing hand-written pattern that strict-server replaces.
- `services/api/internal/db/queries/scaffold.sql` — sqlc queries kept as-is during the migration.
- `services/api/internal/server/server.go` — The chi mux factory that strict-server routes mount into.
- `services/api/tools.go` — Where to add `_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"` so `go install` works.
- `services/api/sqlc.yaml` — Unrelated to Phase 2 but referenced by the generator pattern (planner reads to confirm config-file convention).

### Research (translation note: Phase 1 already applied the Go pivot)
- `.planning/research/SUMMARY.md` — Domain findings authoritative; stack pivots already absorbed.
- `.planning/research/PITFALLS.md` §1 (Multi-Org Isolation Pitfalls) — Still relevant for the `X-Org-Id` header spec shape and the `/openapi.yaml` bypass route (must not regress middleware-at-root).
- `.planning/research/FEATURES.md` — Catalog entity field shapes ground-truth for `components.schemas` design.

### External standards
- [OpenAPI Specification 3.1.0](https://spec.openapis.org/oas/v3.1.0) — Required reading for `requestBody` content negotiation, `oneOf` discriminators, and JSON Schema 2020-12 alignment.
- [oapi-codegen v2 documentation](https://github.com/oapi-codegen/oapi-codegen) — Strict server mode flags, config schema (`oapi-codegen.yaml`), and `--generate spec` for embedded bytes.
- [openapi-typescript documentation](https://openapi-ts.dev/) — CLI usage, `paths` / `components` type structure, JSON Schema mapping rules.
- [openapi-fetch documentation](https://openapi-ts.dev/openapi-fetch/) — Factory pattern, middleware hooks, response type narrowing.
- [@lit/task documentation](https://lit.dev/docs/data/task/) — Task lifecycle states (initial/pending/complete/error) consumed by D-39's Lit-aware helper.
- [Scalar API Reference](https://github.com/scalar/scalar) — `/docs` viewer choice; CDN-loaded `<script>` integration.
- [Redocly CLI lint rulesets](https://redocly.com/docs/cli/commands/lint) — `recommended` ruleset used by D-48.
- [RFC 9562 — UUID Formats](https://www.rfc-editor.org/rfc/rfc9562.html) §5.7 — UUIDv7 spec; referenced in OpenAPI schema descriptions.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`middleware.WriteError(w, status, error, reason)`** at `services/api/internal/middleware/response.go` — The canonical error writer. Phase 2 extends to embed `request_id` from ctx; signature stays backward-compatible.
- **`middleware.WriteJSON(w, status, body)`** at same location — JSON marshaling helper; strict-server's generated marshaling can coexist (different code paths).
- **`scaffold.Routes(orgDB)` chi sub-router** at `services/api/internal/scaffold/handler.go:40` — The pattern Phase 2 migrates to strict-server. Three routes (`POST /`, `GET /`, `GET /{id}`) become three strict-server methods on a `ScaffoldStrictHandlers` struct.
- **`generated.New(orgDB)` sqlc Queries constructor** — Unchanged by Phase 2; strict-server handlers continue to build per-request `*Queries` against the shared `*db.OrgDB`.
- **`server.NewMux`** at `services/api/internal/server/server.go` — Phase 2 adds two new params: the strict-server handlers struct and the spec-bytes for `/openapi.yaml`. Middleware chain stays D-30 contract.
- **`web/packages/ui/` empty package skeleton** — Phase 1 stubbed; Phase 2 fills `src/api/` and updates `package.json` with `openapi-typescript` + `openapi-fetch` + `@lit/task` deps.

### Established Patterns
- **Bypass routes at root, `/v1/*` gated by OrgContext** (D-21) — Phase 2 follows this: `/openapi.yaml` and `/docs` register at root, before the `Route("/v1", ...)` block in `server.NewMux`.
- **Response body shape `{error, reason}`** (Phase 1 scaffold handlers) — Phase 2 locks this in OpenAPI `components.schemas.ErrorResponse` and extends with `request_id`.
- **chi sub-router via `Routes()` constructor returning `chi.Router`** (scaffold pattern) — Generated strict-server output follows the same convention; planner verifies oapi-codegen output is mountable via `r.Mount("/v1", api.Handler(strictHandler))`.
- **UUIDv7 minted at write boundary** (D-19, scaffold `uuid.Must(uuid.NewV7())` at create) — Strict-server handlers continue this pattern; OpenAPI schemas declare UUIDv7 in field descriptions but do NOT enforce v7 in JSON Schema (would require custom format validator; defer to handler-side check matching D-20's middleware pattern).
- **Single sqlc package `generated/`** — Sets precedent for single `api/` package in D-42.

### Integration Points
- **Phase 3 (Catalog CRUD)** — Generates handlers from the v0.1 spec produced here; deletes `_scaffold` paths in the first migration. Adds the 6 entity schemas (Agent, Skill, Queue, Channel, Adapter, BreakReason) to `components.schemas`.
- **Phase 4 (Agent State Machine)** — Implements `PATCH /agents/{id}/status` handler against Phase 2's spec. State enum (`AgentState`) lives in `components.schemas` and is shared between admin and embed via the TS client.
- **Phase 5 (Bulk Import)** — Implements `POST /v1/orgs/{org_id}/catalog/import` against the `BulkImportResult` schema specced here (D-37).
- **Phase 6 (Shared UI Library & Standalone Admin)** — Consumes the Phase 2 TS client. Builds Lit components on top of `createApiClient` + `createApiTask`.
- **Phase 7 (Web Component Embed Bundle)** — Imports only `createApiClient` + types (no `@lit/task` helper if size matters). Embed's Custom Element passes `org-id` attribute through `getOrgId`.

</code_context>

<specifics>
## Specific Ideas

- **UI sketches are contract validation, not UI design** — User explicitly raised "should we design UI before API". Decision: API-first stays, but Phase 2 prepends paper-sketches as a de-risking step. Sketches are throwaway markdown — if a sketch reveals an awkward API shape, the spec adjusts before lock-in. Phase 6 owns real UI.
- **Strict-server, not chi-server** — User prefers contract-enforced shapes over per-handler flexibility. 30+ endpoints × CRUD boilerplate is the win; v0.1 has no streaming/file-download cases that strict-server makes awkward.
- **Closed enum for `error` field, not open string** — Locks the taxonomy; codegen drift gate catches accidental new codes. Reserved `rate_limited` even though v0.1 doesn't implement rate limiting — keeps client code stable when v0.2 adds it.
- **request_id in body AND header** — Redundant on purpose; users copy/paste error bodies into bug reports more often than headers, so embedding `request_id` in the body matters for support DX even though header already carries it.
- **Factory client, not singleton** — Web Component embed mandate. `getOrgId` is invoked per-request so embed instances react to attribute changes without client recreation.
- **All three TS layers ship in Phase 2** — User chose option "Cả 1,2,3" (factory + error helpers + Lit async helper). Phase 7 embed can opt out of the Lit helper by importing only `createApiClient` + types.
- **/openapi.yaml served from embedded bytes** — Always exactly the spec the running binary was built against. No risk of stale `openapi.yaml` on disk vs deployed binary mismatch.
- **Scaffold migration proves codegen end-to-end** — Phase 2 doesn't ship dead generated code. Migrating scaffold to strict-server forces the full pipeline (spec → codegen → handler → chi mount → ServeHTTP → response) to work before Phase 3 starts.

</specifics>

<deferred>
## Deferred Ideas

- **RFC 7807 Problem Details migration** — User chose to stick with Phase 1's `{error, reason}` shape. If v1 adds external API consumers that expect standard error envelopes, revisit then.
- **Real auth on `/docs`** — Phase 2 ships `/docs` public (stub auth in v0.1). v1 RBAC will gate `/docs` behind admin role.
- **Per-tag splitting of generated Go packages** — Single `api/` package suffices for ~30 v0.1 endpoints; revisit if v0.2 grows past ~80 endpoints.
- **`openapi-format` / `swagger-cli bundle` for spec splitting** — v0.1 keeps single `openapi/openapi.yaml`; if it crosses ~3000 lines and becomes hard to review, split into per-tag YAML files merged at build time.
- **Mock server from spec (Prism, MSW)** — UI sketches replace the need for a runtime mock server in Phase 2. If Phase 6 needs to develop ahead of Phase 3 handlers, spin up Prism then.
- **Spec contract tests (Schemathesis, Dredd)** — Property-based testing of every endpoint against the spec. Defer to a dedicated testing phase or sprinkle into Phase 3/4/5 as endpoints land.
- **Rate-limiting headers in spec** — `rate_limited` error code reserved in D-36; actual `X-RateLimit-*` response headers and 429 handling deferred to v0.2.
- **Webhook / async response specifications** — Out of v0.1 scope (no webhooks, no async patterns).
- **GraphQL or gRPC alternative surface** — PROJECT.md commits to REST for v0.1. Internal gRPC for runtime is a v1 concern.
- **API versioning beyond URL `/v1/`** — v0.1 has only `/v1`; no header-based versioning, no deprecation policy yet. Decide when v0.2 introduces breaking changes.

</deferred>

---

*Phase: 2-OpenAPI Contract & Codegen*
*Context gathered: 2026-05-15*
