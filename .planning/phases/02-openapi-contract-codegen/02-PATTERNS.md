# Phase 2: OpenAPI Contract & Codegen - Pattern Map

**Mapped:** 2026-05-15
**Files analyzed:** 22 files (4 sketch artifacts + 1 spec + 8 Go new + 4 Go modified + 5 TS new + 1 TS modified + 1 CI modified + 1 Taskfile modified, plus 1 new sqlc query path note)
**Analogs found:** 18 / 22 (3 are pure contract artifacts with no in-repo analog; 1 is greenfield TS module bootstrap)

## Greenfield-Within-Foundation Note

Phase 1 already shipped the Go HTTP foundation (chi router, middleware chain, scaffold handlers, sqlc codegen pipeline). Phase 2 is **NOT** greenfield for Go — it extends Phase 1's existing files (`tools.go`, `server.go`, `httputil.go`, `Taskfile.yml`, `ci.yml`) and adds the new `internal/api/` package whose pattern is **mirror sqlc's `internal/db/generated/` structure** (same single-package convention, same `Code generated... DO NOT EDIT.` discipline).

For TypeScript, `web/packages/ui/` is empty (Phase 1 stubbed only `index.ts` with `export {};`). The TS files in Phase 2 have NO in-repo analog — they bootstrap the first real module in `packages/ui`. The closest reference is the workspace conventions already encoded in `web/tsconfig.base.json` (`verbatimModuleSyntax`, `isolatedModules`, ESM-only) which dictate the import/export style.

The codegen config files (`oapi-codegen.yaml`) have a strong in-repo analog: **`services/api/sqlc.yaml`** — same config-file convention, same `gen:` block style, same `out:` pattern pointing at `internal/<package>/` directories.

## File Classification

### Summary by Layer

| Layer | New Files | Modified Files | Phase Decisions |
|-------|----------|----------------|-----------------|
| L1. Contract artifacts (paper sketches + OpenAPI spec) | 2 | 0 | D-32, D-33, D-34, D-37 |
| L2. Codegen config (Go side) | 1 | 1 | D-41, D-43 |
| L3. Generated Go code (oapi-codegen output) | 3 | 0 | D-41, D-42 |
| L4. Go server integration (middleware + mux + scaffold migration) | 0 | 3 | D-35, D-44, D-45, D-46 |
| L5. /openapi.yaml + /docs runtime handlers | 1 | 0 | D-45 |
| L6. TypeScript client distribution | 5 | 1 | D-38, D-39, D-40 |
| L7. CI drift gate + lint | 0 | 1 | D-47, D-48 |
| L8. Task orchestration | 0 | 1 | D-43 |
| L9. sqlc input modification (scaffold migration) | 0 | 0 | D-46 (note: sqlc queries unchanged) |
| **TOTAL** | **12 new** | **7 modified** | |

### Master File Inventory

| # | File | New/Modified | Role | Data Flow | Closest Analog | Match Quality |
|---|------|--------------|------|-----------|----------------|---------------|
| 1 | `.planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md` | new | contract artifact | n/a (markdown) | none | n/a |
| 2 | `openapi/openapi.yaml` | new | contract spec | request-response | none (greenfield) | n/a |
| 3 | `services/api/internal/api/oapi-codegen.yaml` | new | codegen config | n/a (declarative) | `services/api/sqlc.yaml` | exact (codegen config convention) |
| 4 | `services/api/tools.go` | modified | dep pin | n/a | self (existing pattern) | exact |
| 5 | `services/api/internal/api/types.gen.go` | new (generated) | DTO struct generator output | n/a (generated) | `services/api/internal/db/generated/models.go` | exact (single-package generated output) |
| 6 | `services/api/internal/api/server.gen.go` | new (generated) | strict-server interface + chi wiring | request-response | `services/api/internal/db/generated/db.go` + `scaffold.sql.go` | partial (different framework, same single-package) |
| 7 | `services/api/internal/api/spec.gen.go` | new (generated) | embedded spec bytes | n/a (asset) | `services/api/internal/db/generated/models.go` (header pattern) | weak (no analog for `//go:embed` here) |
| 8 | `services/api/internal/middleware/httputil.go` | modified | response helpers | request-response | self (extend, don't replace) | exact |
| 9 | `services/api/internal/server/server.go` | modified | chi mux factory | request-response | self (extend NewMux signature) | exact |
| 10 | `services/api/internal/server/openapi.go` | new | `/openapi.yaml` + `/docs` handlers | request-response | `services/api/internal/server/health.go` (LiveHandler/MetricsHandler shape) | exact (same bypass-handler pattern) |
| 11 | `services/api/internal/scaffold/handler.go` | modified | scaffold-to-strict-server migration | request-response | self (full rewrite to strict-server interface) | exact (the file IS the migration target) |
| 12 | `services/api/internal/scaffold/handler_test.go` | modified | scaffold tests adjusted | request-response | self (signature changes only) | exact |
| 13 | `web/packages/ui/package.json` | modified | TS package manifest | n/a | self (add deps + gen:api script) | exact |
| 14 | `web/packages/ui/src/api/generated.ts` | new (generated) | TS types from openapi-typescript | n/a (generated) | `services/api/internal/db/generated/*.go` (commit-and-drift-gate convention) | exact (codegen output policy) |
| 15 | `web/packages/ui/src/api/client.ts` | new | factory client (openapi-fetch wrapper) | request-response | none in-repo (TS workspace is empty) | n/a |
| 16 | `web/packages/ui/src/api/errors.ts` | new | error helpers | request-response | none in-repo | n/a |
| 17 | `web/packages/ui/src/api/task.ts` | new | Lit-aware async helper | request-response | none in-repo | n/a |
| 18 | `web/packages/ui/src/api/index.ts` | new | barrel export | n/a | `web/packages/ui/src/index.ts` (style: ESM exports) | weak (existing is `export {};`) |
| 19 | `web/packages/ui/src/index.ts` | modified | root barrel | n/a | self (replace `export {};` with re-export `./api`) | exact |
| 20 | `.github/workflows/ci.yml` | modified | add `codegen-drift` job | n/a (declarative) | self (`go-vet` / `web-typecheck` / `migration-drift` jobs) | exact |
| 21 | `Taskfile.yml` | modified | extend `gen` target | n/a (declarative) | self (existing `gen` task already calls sqlc) | exact |
| 22 | `services/api/internal/db/queries/scaffold.sql` | unchanged | sqlc input | n/a | n/a | n/a (D-46 keeps sqlc queries as-is) |

---

## Pattern Assignments

### `services/api/internal/api/oapi-codegen.yaml` (codegen config)

**Analog:** `services/api/sqlc.yaml`

**Convention to copy (entire file shape):**

```yaml
# services/api/sqlc.yaml — exact in-repo precedent for codegen config
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/db/queries"
    schema: "../../migrations"
    gen:
      go:
        package: "generated"
        out: "internal/db/generated"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: false
        emit_exact_table_names: false
        emit_empty_slices: true
        emit_pointers_for_null_types: true
```

**Pattern to extract:**
1. `version: "2"` at top (sqlc convention; oapi-codegen uses different schema but same idea)
2. Single top-level config map with `gen:`-style block
3. `out:` always points at `internal/<package>/` directory (D-42 says `internal/api/`)
4. `package:` matches the directory name (`generated` for sqlc → `api` for oapi-codegen)
5. Codegen flags grouped under nested key (`go:` for sqlc; `output-options:` for oapi-codegen)

**Apply pattern for oapi-codegen.yaml:**
```yaml
# services/api/internal/api/oapi-codegen.yaml (planner authors)
package: api
generate:
  strict-server: true   # D-41
  models: true
  embedded-spec: true   # produces spec.gen.go per D-42
output: ./server.gen.go # planner splits via separate config files OR single config + multi-output
# additional output-options flags per D-Discretion bullet 3
```

(Planner: oapi-codegen v2 supports multiple outputs from one config OR multiple configs each generating one file. Decide layout based on first-pass output; either is consistent with D-42's three-file split.)

---

### `services/api/tools.go` (dependency pin extension)

**Analog:** Self (existing file — extend, don't replace)

**Imports pattern** (lines 22-53):
```go
//go:build tools

// Package tools tracks dev-time and runtime module dependencies so that
// `go mod tidy` does not prune them before per-package source files are
// added in later plans (Plans 02-07). Plan 01-01 documents the locked
// stack surface here; subsequent plans import these packages from real
// source files, at which point this file becomes redundant and can be
// deleted. Activate with `go vet -tags tools ./...`. The `tools` build
// tag isolates this file from production binary builds.
package tools

import (
	// HTTP + DB stack
	_ "github.com/go-chi/chi/v5"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/google/uuid"
	_ "github.com/jackc/pgx/v5"
	// ...
)
```

**Pattern to extract:**
- Single `import (` block grouped by category with `//` headers
- Underscore imports `_ "<module-path>"` (blank import — the only way `go mod tidy` keeps the module pinned without a real consumer)
- `//go:build tools` directive at top — keeps file out of production builds (CRITICAL)
- Comment block above import names the category (HTTP + DB stack, OTel SDK + exporters + contrib, Test stack)

**Apply pattern:** Add a new comment-headed group for codegen tools, with the oapi-codegen blank import:
```go
// Codegen tools (Phase 2 D-43)
_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
```

Same blank-import style; add it inside the existing `import (...)` block, before the Test stack section (keeps build-tag isolation working).

---

### `services/api/internal/api/types.gen.go`, `server.gen.go`, `spec.gen.go` (oapi-codegen output)

**Analog:** `services/api/internal/db/generated/db.go`, `models.go`, `scaffold.sql.go` (sqlc output — same generated-code discipline)

**Header pattern** (from `services/api/internal/db/generated/models.go:1-9`):
```go
// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.31.1

package generated

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type Scaffold struct {
	ID         pgtype.UUID        `json:"id"`
	OrgID      pgtype.UUID        `json:"org_id"`
	ExternalID string             `json:"external_id"`
	Name       string             `json:"name"`
	CreatedAt  pgtype.Timestamptz `json:"created_at"`
}
```

**DBTX constructor pattern** (from `services/api/internal/db/generated/db.go:14-32`):
```go
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type Queries struct {
	db DBTX
}
```

**Pattern to extract for oapi-codegen output:**
1. **Header comment** identifies generator + version: `// Code generated by oapi-codegen. DO NOT EDIT.` (oapi-codegen emits this automatically — no manual work).
2. **Single package** at the directory level (`package api` — mirrors `package generated`).
3. **Constructor function** `New<X>` accepting an interface (mirrors `func New(db DBTX) *Queries`). oapi-codegen's strict server emits `NewStrictHandler(ssi StrictServerInterface, middlewares []StrictMiddlewareFunc)` — same shape.
4. **DTO structs with JSON tags** (mirrors `Scaffold struct { ... json:"id" ... }`). For OpenAPI: `CreateAgentRequestObject`, `CreateAgentResponseObject` per D-41.
5. **`.golangci.yml` lax exclusions for generated/** — already in place at `services/api/.golangci.yml:19-20`:
   ```yaml
   - path: internal/db/generated/
     linters: [gosec, errcheck, gocritic, staticcheck]
   ```
   Planner MUST add a sibling rule for `internal/api/` (oapi-codegen output triggers similar lint warnings).

**`spec.gen.go` specifically** (no perfect analog — `//go:embed` is greenfield in this repo):
- File body (auto-generated by oapi-codegen `--generate spec`) contains a single `var openapiSpec = []byte{...}` with base64-decoded YAML bytes, plus a `GetSwagger()` helper. No manual work; the file is fully owned by the generator. D-45 consumes the bytes via `api.GetSwagger().Bytes` (or equivalent) for the `/openapi.yaml` handler.

---

### `services/api/internal/middleware/httputil.go` (extend WriteError to embed `request_id`)

**Analog:** Self (existing file — extend, don't replace)

**Current shape** (lines 31-54):
```go
type errorBody struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
}

// WriteError writes a JSON error response with the canonical shape.
//
// Parameters:
//   - status: HTTP status code (4xx / 5xx)
//   - code:   stable error identifier — e.g. "invalid_org_id", "not_found".
//     Clients (admin UI, future SDKs) branch on this value.
//   - reason: contextual detail — e.g. "missing_header", "malformed_uuid".
//     Suitable for log lines and developer debugging.
func WriteError(w http.ResponseWriter, status int, code, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(errorBody{Error: code, Reason: reason}); err != nil {
		slog.Error("httputil: encode error response", "err", err, "code", code)
	}
}
```

**Pattern to extract:**
- `errorBody` struct with `json:"error"` / `json:"reason"` tags
- Set Content-Type header BEFORE WriteHeader, BEFORE Encode (lines 49-51 — order matters because once status is written you can't add headers)
- Use `json.NewEncoder(w).Encode(...)` (not `Marshal` + `Write`) — single-allocation streaming
- On encode failure, slog.Error and return (no recovery once status is sent)

**Apply pattern for D-35 (embed `request_id`):**
- Add `RequestID string \`json:"request_id,omitempty"\`` to `errorBody` struct
- Change `WriteError` signature to: `func WriteError(ctx context.Context, w http.ResponseWriter, status int, code, reason string)`
  OR keep signature and pull RequestID from `r.Context()` via a new helper that takes `*http.Request`
- Pull request_id via existing `RequestIDFromContext(ctx)` (from `requestid.go:45-48`):
  ```go
  // existing accessor:
  func RequestIDFromContext(ctx context.Context) (uuid.UUID, bool) {
      id, ok := ctx.Value(requestIDCtxKey{}).(uuid.UUID)
      return id, ok
  }
  ```
- `omitempty` on the JSON tag so callers without ctx (bypass routes, tests) still produce valid responses

**Critical:** Phase 1's CONTEXT.md line 178 says "signature stays backward-compatible." Planner picks between (a) adding a ctx-aware variant `WriteErrorCtx(ctx, w, status, code, reason)` keeping the old one, or (b) changing the signature and updating all callers. Audit callers first:

```bash
# planner runs this — current callers of WriteError:
grep -rn "WriteError(" services/api/ --include="*.go"
# Expected hits: orgcontext.go (3), scaffold/handler.go (6)
```

Either approach works; (b) is cleaner if all 9 sites are easy to update.

---

### `services/api/internal/server/server.go` (extend `NewMux` for strict-server + `/openapi.yaml` + `/docs`)

**Analog:** Self (extend `Deps` and `NewMux`)

**Current `NewMux` shape** (lines 74-96):
```go
func NewMux(deps *Deps) http.Handler {
	r := chi.NewRouter()

	// (1) Recoverer first.
	r.Use(chimw.Recoverer)
	// (2) Our UUIDv7 RequestID — overrides chi's host/N-K counter (D-28).
	r.Use(appmw.RequestID)

	// (3) Bypass routes — D-21 says they MUST NOT carry OrgContext.
	r.Get("/healthz", LiveHandler())
	r.Get("/readyz", ReadyzHandler(deps.Pool, deps.Redis))
	r.Get("/metrics", MetricsHandler())

	// (4) /v1 sub-router.
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(appmw.OrgContext)
		v1.Mount("/orgs/{org_id}/_scaffold", scaffold.Routes(deps.OrgDB))
	})

	return r
}
```

**`Deps` struct shape** (lines 34-39):
```go
type Deps struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	OrgDB  *db.OrgDB
	Config *config.Config
}
```

**Pattern to extract:**
1. Bypass routes registered at root via `r.Get("/path", handler())` BEFORE `r.Route("/v1", ...)` block (preserves D-21 — middleware-inside-Route-closure scoping is the enforcement seam)
2. `/v1` block uses `chi.Router` closure with `v1.Use(...)` followed by `v1.Mount(...)`. Mount path strips prefix.
3. Each sub-router constructor (e.g., `scaffold.Routes(orgDB)`) returns `chi.Router`, not `http.Handler` — preserves chi's mounting semantics
4. `Deps` struct is the seam — bundle dependencies, not positional args. Per D-44, Phase 2 adds new fields (`StrictHandlers` + spec bytes — or generated `api.Handler` shaped value).

**Apply pattern for D-44 + D-45:**
- Extend `Deps`:
  ```go
  type Deps struct {
      Pool           *pgxpool.Pool
      Redis          *redis.Client
      OrgDB          *db.OrgDB
      Config         *config.Config
      StrictHandlers api.StrictServerInterface // D-44
      SpecBytes      []byte                    // D-45 (or call api.GetSwagger() inside NewMux)
  }
  ```
- Add bypass routes BEFORE `r.Route("/v1", ...)` — same pattern as `/healthz`:
  ```go
  // D-21 extension: /openapi.yaml + /docs are bypass routes (no OrgContext).
  r.Get("/openapi.yaml", OpenAPISpecHandler(deps.SpecBytes))
  r.Get("/docs", DocsHandler())
  ```
- Inside `/v1` closure, REPLACE the `v1.Mount("/orgs/{org_id}/_scaffold", scaffold.Routes(deps.OrgDB))` line with the generated strict-server handler mount. oapi-codegen v2 emits a `HandlerWithOptions(ssi, options) http.Handler` or `Handler(ssi) http.Handler` factory; mount it under `/v1`:
  ```go
  v1.Mount("/", api.HandlerWithOptions(deps.StrictHandlers, api.ChiServerOptions{
      BaseURL: "", // already inside /v1
      // ...
  }))
  ```
  (Planner verifies exact factory name on first codegen pass — oapi-codegen v2's chi backend exposes `HandlerFromMux`, `Handler`, `HandlerWithOptions`.)

---

### `services/api/internal/server/openapi.go` (new file: `/openapi.yaml` + `/docs` handlers)

**Analog:** `services/api/internal/server/health.go` — same bypass-handler pattern (LiveHandler / MetricsHandler return `http.HandlerFunc` constants).

**Pattern to copy** (from health.go:49-55 and 134-139):
```go
// LiveHandler returns 200 + {"status":"alive"} unconditionally.
//
// The body literal is written via w.Write (not json.Encoder) because the
// payload is a fixed string; this avoids the encoder allocation on every
// LB probe.
func LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"alive"}`))
	}
}

// MetricsHandler is the Phase 1 placeholder for /metrics ...
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(""))
	}
}
```

**Pattern to extract:**
1. **Factory style:** `func XHandler(deps...) http.HandlerFunc` — returns a closure, not a method receiver. Pattern matches across health.go, requestid.go, orgcontext.go.
2. **Content-Type set before WriteHeader before Write** — exact ordering.
3. **`_, _ = w.Write(...)`** for the discard-error idiom on fixed byte writes (gosec satisfied because golangci.yml waives errcheck inside `services/api` paths or via inline `_, _ =`).
4. **Method-package doc comment** explains why the handler exists (bypass list extension, no auth in v0.1) — same depth as the existing health.go top-of-file comment block.

**Apply pattern:**
```go
// Package server: openapi.go owns the /openapi.yaml and /docs bypass
// routes per D-45 (Phase 2). Both are mounted at root in NewMux BEFORE
// the /v1 sub-router so they reach without an X-Org-Id header.

// OpenAPISpecHandler returns 200 + the embedded spec bytes from
// api.spec.gen.go (oapi-codegen --generate spec). Content-Type
// application/yaml. specBytes is captured at NewMux time; serving the
// embedded bytes guarantees the spec matches the running binary (D-45).
func OpenAPISpecHandler(specBytes []byte) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/yaml")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write(specBytes)
    }
}

// DocsHandler returns 200 + a single static HTML page that loads the
// chosen interactive viewer (Scalar by D-Discretion default) from CDN.
// The HTML fetches /openapi.yaml from the same origin — no extra static
// assets in the repo.
func DocsHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write(docsHTML)
    }
}

// docsHTML is a const []byte of the ~40-line HTML viewer page.
// Planner: use //go:embed for the HTML file rather than a Go string
// literal so the HTML can be edited without escaping.
//
//go:embed docs.html
var docsHTML []byte
```

(`docs.html` companion file is implied; planner places it alongside `openapi.go`.)

---

### `services/api/internal/scaffold/handler.go` (migrate to strict-server interface)

**Analog:** Self (current file is the migration source AND the closest pattern reference)

**Current hand-written shape** (lines 40-47, 99-138):
```go
// Routes returns a chi.Router carrying the three Phase 1 _scaffold routes.
func Routes(orgDB *db.OrgDB) chi.Router {
	h := &handler{orgDB: orgDB}
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	return r
}

type handler struct {
	orgDB *db.OrgDB
}

// create handles POST /v1/orgs/{org_id}/_scaffold.
func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "missing_org_id_in_context")
		return
	}

	var body createRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed_json")
		return
	}
	if body.ExternalID == "" || body.Name == "" {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_body", "external_id_and_name_required")
		return
	}

	q := generated.New(h.orgDB)
	id := uuid.Must(uuid.NewV7())
	row, err := q.InsertScaffold(ctx, generated.InsertScaffoldParams{
		ID:         toPgUUID(id),
		OrgID:      toPgUUID(orgID),
		ExternalID: body.ExternalID,
		Name:       body.Name,
	})
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "insert_failed")
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, toResponse(row))
}
```

**Cross-org probe pattern (must preserve)** (lines 174-201):
```go
func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "missing_org_id_in_context")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_id", "malformed_uuid")
		return
	}
	q := generated.New(h.orgDB)
	row, err := q.GetScaffoldByID(ctx, generated.GetScaffoldByIDParams{
		ID:    toPgUUID(id),
		OrgID: toPgUUID(orgID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "no_such_scaffold")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "get_failed")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, toResponse(row))
}
```

**Pattern to extract (INVARIANTS the strict-server rewrite MUST preserve):**
1. **Authoritative org_id from ctx** (lines 106, 147, 176) via `orgkey.OrgIDFromContext(ctx)` — NEVER from URL params, NEVER from request body. URL `{org_id}` parameter is decorative.
2. **UUIDv7 minted server-side** at create (line 126): `id := uuid.Must(uuid.NewV7())` (D-19).
3. **`generated.New(orgDB)` per-request** (line 125) — Queries struct is constructed fresh each handler call, not memoized.
4. **`pgx.ErrNoRows` → 404 with `not_found` code** (line 194) — FOUND-08 cross-org leakage guard relies on this.
5. **`pgtype.UUID` conversion via `toPgUUID(id)` helper** (lines 95-97): `return pgtype.UUID{Bytes: id, Valid: true}` — needed because sqlc emits `pgtype.UUID`, not `google/uuid`.
6. **Response struct `scaffoldResponse`** (lines 70-90) — conversion via `toResponse()` because `pgtype.UUID` encodes as `{"Bytes":[...],"Valid":true}` (useless to clients); the public DTO uses `google/uuid` + `time.Time`.

**Apply pattern (strict-server migration):**

The hand-written handler becomes a strict-server interface implementation. oapi-codegen strict-server emits:

```go
// In services/api/internal/api/server.gen.go (auto-generated):
type StrictServerInterface interface {
    CreateScaffold(ctx context.Context, request CreateScaffoldRequestObject) (CreateScaffoldResponseObject, error)
    ListScaffolds(ctx context.Context, request ListScaffoldsRequestObject) (ListScaffoldsResponseObject, error)
    GetScaffoldById(ctx context.Context, request GetScaffoldByIdRequestObject) (GetScaffoldByIdResponseObject, error)
}

type CreateScaffoldRequestObject struct {
    Body *CreateScaffoldJSONRequestBody  // codegen marshals the JSON body
}

type CreateScaffold201JSONResponse Scaffold  // type alias for the 201 response body
type CreateScaffoldResponseObject interface { VisitCreateScaffoldResponse(w http.ResponseWriter) error }
```

The new `scaffold/handler.go` becomes:

```go
// Handler implements api.StrictServerInterface for the _scaffold routes (D-46).
//
// All three invariants from Phase 1 carry forward:
//   - Authoritative org_id from ctx (FOUND-05)
//   - UUIDv7 at write boundary (D-19)
//   - pgx.ErrNoRows → 404 not_found (FOUND-08)
type Handler struct {
    orgDB *db.OrgDB
}

func NewHandler(orgDB *db.OrgDB) *Handler {
    return &Handler{orgDB: orgDB}
}

func (h *Handler) CreateScaffold(ctx context.Context, req api.CreateScaffoldRequestObject) (api.CreateScaffoldResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        // Strict-server: typed response object instead of WriteError. The
        // generated 500 response object carries the same {error, reason,
        // request_id} shape because openapi.yaml schemas.ErrorResponse
        // says so.
        return api.CreateScaffold500JSONResponse{
            Error:  "internal",
            Reason: "missing_org_id_in_context",
        }, nil
    }
    // body access: req.Body is a typed pointer; codegen already validated
    // required fields per openapi.yaml's `required: [external_id, name]`.
    q := generated.New(h.orgDB)
    id := uuid.Must(uuid.NewV7())
    row, err := q.InsertScaffold(ctx, generated.InsertScaffoldParams{
        ID:         toPgUUID(id),
        OrgID:      toPgUUID(orgID),
        ExternalID: req.Body.ExternalId,
        Name:       req.Body.Name,
    })
    if err != nil {
        return api.CreateScaffold500JSONResponse{Error: "internal", Reason: "insert_failed"}, nil
    }
    return api.CreateScaffold201JSONResponse(toResponse(row)), nil
}

func (h *Handler) GetScaffoldById(ctx context.Context, req api.GetScaffoldByIdRequestObject) (api.GetScaffoldByIdResponseObject, error) {
    orgID, ok := orgkey.OrgIDFromContext(ctx)
    if !ok {
        return api.GetScaffoldById500JSONResponse{Error: "internal", Reason: "missing_org_id_in_context"}, nil
    }
    // strict-server: req.Id is already-parsed uuid.UUID (codegen handles
    // path parameter parsing per openapi.yaml's `format: uuid`). The
    // handler does NOT call uuid.Parse manually.
    q := generated.New(h.orgDB)
    row, err := q.GetScaffoldByID(ctx, generated.GetScaffoldByIDParams{
        ID:    toPgUUID(req.Id),
        OrgID: toPgUUID(orgID),
    })
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return api.GetScaffoldById404JSONResponse{Error: "not_found", Reason: "no_such_scaffold"}, nil
        }
        return api.GetScaffoldById500JSONResponse{Error: "internal", Reason: "get_failed"}, nil
    }
    return api.GetScaffoldById200JSONResponse(toResponse(row)), nil
}
```

**Helpers to keep verbatim** from the existing handler.go (lines 70-97 — `scaffoldResponse`, `toResponse`, `toPgUUID`). These remain in the scaffold package.

**`Routes()` constructor removed:** The strict-server mounts via `api.HandlerWithOptions(strictHandler, ...)` in `server.go` (per D-44). Old `Routes(orgDB)` returning `chi.Router` is deleted.

**Note on `request_id`:** Strict-server's typed response objects (e.g., `api.CreateScaffold500JSONResponse`) include a `RequestId` field per the openapi.yaml `ErrorResponse` schema. The strict-server middleware chain (oapi-codegen's `StrictMiddlewareFunc`) is where planner injects `request_id` from ctx into every response object. Pattern:
```go
// In server.go NewMux:
strictHandler := api.NewStrictHandler(scaffoldHandler, []api.StrictMiddlewareFunc{
    requestIDInjectionMiddleware,  // populates RequestId field on every response object
})
```

---

### `services/api/internal/scaffold/handler_test.go` (test updates for strict-server)

**Analog:** Self (current file — adjust callsites only, preserve test logic)

**Test helper that adapts** (current lines 144-164):
```go
func callWithOrg(t *testing.T, h chi.Router, orgID uuid.UUID, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req = req.WithContext(orgkey.SetOrgID(req.Context(), orgID))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
```

**Pattern to extract:**
- `httptest.NewRequest` + `httptest.NewRecorder` + `h.ServeHTTP(rec, req)` is the canonical Go HTTP test triple
- `req.WithContext(orgkey.SetOrgID(req.Context(), orgID))` simulates OrgContext middleware
- Test helper takes `chi.Router` (interface, accepts any router)

**Apply pattern:** Change the `h chi.Router` parameter to `h http.Handler` (strict-server's `api.HandlerWithOptions` returns `http.Handler`, not `chi.Router`). All `h.ServeHTTP` calls work identically.

**Test invariants to preserve verbatim** (no change to assertion logic):
- `TestScaffold_CreateAndGet_Roundtrip` (lines 170-214) — UUIDv7 check at line 198: `require.Equal(t, uint8(7), uint8(created.ID.Version()))`
- `TestScaffold_GetNotFound_Returns404` (lines 221-236) — 404 on missing/cross-org
- `TestScaffold_CreateInvalidBody_Returns400` (lines 241-256) — but the rejection now comes from oapi-codegen's request validation, NOT the handler's hand-rolled emptiness check. Reason string MAY change from `"external_id_and_name_required"` to the openapi-codegen default. Planner verifies and updates assertion.
- `TestScaffold_GetMalformedID_Returns400` (lines 262-274) — same: oapi-codegen's path parameter parser emits the 400 before the handler runs.

---

### `web/packages/ui/package.json` (add deps + gen:api script)

**Analog:** Self (existing minimal manifest at lines 1-10)

**Current shape:**
```json
{
  "name": "@open-routing/ui",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "typecheck": "tsc --noEmit",
    "lint": "eslint src --max-warnings 0"
  }
}
```

**Pattern to extract:**
- `"type": "module"` (ESM-only, mandated by `tsconfig.base.json` `module: "ESNext"`)
- Top-level scripts keyed by Turborepo task name (`typecheck`, `lint` — these are already in `turbo.json` pipeline)
- No `main` / `exports` field in v0.1 (consumers import via TypeScript path resolution from `src/index.ts`)

**Apply pattern:**
```json
{
  "name": "@open-routing/ui",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "typecheck": "tsc --noEmit",
    "lint": "eslint src --max-warnings 0",
    "gen:api": "openapi-typescript ../../../openapi/openapi.yaml -o src/api/generated.ts"
  },
  "dependencies": {
    "openapi-fetch": "^0.13.0",
    "@lit/task": "^1.0.0"
  },
  "devDependencies": {
    "openapi-typescript": "^7.0.0"
  }
}
```

(Exact versions per D-Discretion; planner pins precise patch level on first install. `openapi-fetch` is a runtime dep; `openapi-typescript` is dev-only because it's a generator CLI.)

**`gen:api` integration with Turborepo:** Add to `web/turbo.json`:
```json
{
  "tasks": {
    "typecheck": { "dependsOn": ["^typecheck"], "outputs": [] },
    "lint": { "outputs": [] },
    "gen:api": {
      "inputs": ["../../openapi/openapi.yaml"],
      "outputs": ["src/api/generated.ts"]
    }
  }
}
```

---

### `web/packages/ui/src/api/generated.ts` (openapi-typescript output — committed)

**Analog:** `services/api/internal/db/generated/*.go` — same "generated + committed + drift-gated" discipline

**Pattern to extract (policy, not code):**
1. **Committed to git** (not gitignored) — same as `services/api/internal/db/generated/`
2. **Drift gate in CI** — Phase 2 D-47 adds `codegen-drift` job that runs `pnpm gen:api` and fails on diff. Mirrors the existing `migration-drift` job structure at `.github/workflows/ci.yml:97-123`.
3. **Generator header** — openapi-typescript emits `/** This file was auto-generated by openapi-typescript. */` at top automatically. No manual header needed.
4. **Lint exemption** — add `web/packages/ui/.eslintignore` (or `eslint.config.js` ignore block) for `src/api/generated.ts`, mirroring `services/api/.golangci.yml`'s `path: internal/db/generated/` exclusion.

**Content shape** (per openapi-typescript docs — D-39 lists the structure):
```ts
// generated.ts (auto-generated, do not edit)
export interface paths {
  "/v1/orgs/{org_id}/_scaffold": {
    post: operations["createScaffold"];
    get: operations["listScaffolds"];
  };
  // ...
}
export interface components {
  schemas: {
    Scaffold: { /* ... */ };
    ErrorResponse: { error: string; reason: string; request_id?: string };
    // ...
  };
}
export interface operations { /* ... */ }
```

---

### `web/packages/ui/src/api/client.ts` (factory client)

**Analog:** No in-repo TS module. **External analog: openapi-fetch documentation** (https://openapi-ts.dev/openapi-fetch/) — D-39 reference.

**Pattern to extract (D-38 factory signature):**
```ts
// D-38 mandates this exact signature shape:
export function createApiClient(config: {
  baseURL: string;            // e.g., "https://api.open-routing.io" or relative ""
  getOrgId: () => string;     // admin reads env/login; embed reads Custom Element attribute
  fetch?: typeof fetch;       // override for testing
}): ApiClient;
```

**TypeScript conventions to apply (sourced from `web/tsconfig.base.json`):**
- `"verbatimModuleSyntax": true` → use `import type { paths } from './generated';` for type-only imports (compile error otherwise)
- `"isolatedModules": true` → every file must be independently compilable; no `enum` types that depend on declaration merging
- `"noUncheckedIndexedAccess": true` → `paths["/v1/..."]` returns `T | undefined`, must narrow before use
- `"strict": true` + `"esModuleInterop": true` → default imports work for ESM, but named imports preferred

**Pattern shape (based on openapi-fetch v0.13 idiom):**
```ts
import createClient, { type ClientOptions, type Middleware } from 'openapi-fetch';
import type { paths } from './generated';

export interface CreateApiClientConfig {
  baseURL: string;
  getOrgId: () => string;
  fetch?: typeof fetch;
}

export type ApiClient = ReturnType<typeof createClient<paths>>;

export function createApiClient(config: CreateApiClientConfig): ApiClient {
  const orgIdMiddleware: Middleware = {
    onRequest({ request }) {
      // D-38: getOrgId invoked per request, not at construction
      request.headers.set('X-Org-Id', config.getOrgId());
      return request;
    },
  };

  const client = createClient<paths>({
    baseUrl: config.baseURL,
    fetch: config.fetch,
  });
  client.use(orgIdMiddleware);
  return client;
}
```

**Why this signature shape matches the Go middleware shape:** OrgContext middleware on the Go side (orgcontext.go:48-78) reads `X-Org-Id` per-request from `r.Header.Get("X-Org-Id")`. The factory client mirrors this — `getOrgId()` is invoked per request inside the openapi-fetch middleware so each request gets the current org_id (allows embed instances to react to attribute changes without client recreation, per D-38).

---

### `web/packages/ui/src/api/errors.ts` (error helpers)

**Analog:** No in-repo TS analog. **Indirect analog:** the Go-side `errorBody` struct at `services/api/internal/middleware/httputil.go:31-34`, which defines the on-the-wire shape that this file's `ApiError` type mirrors.

**Cross-language contract** (single source of truth: openapi.yaml `components.schemas.ErrorResponse`):
- Go side: `errorBody struct { Error string; Reason string }` + new `RequestID string` per D-35
- TS side: `components["schemas"]["ErrorResponse"]` from `generated.ts` — has `error: string`, `reason: string`, `request_id?: string`

**Pattern to extract (D-39 listed three helpers):**

```ts
import type { components } from './generated';

// ApiError = the JSON body shape on 4xx/5xx responses.
// Type-aliased from generated.ts so it stays in sync with openapi.yaml.
export type ApiError = components['schemas']['ErrorResponse'];

// ErrorCodes is a const-asserted record so consumers get autocomplete
// AND compile-time exhaustiveness checks. D-36 lists the closed enum.
export const ErrorCodes = {
  INVALID_BODY: 'invalid_body',
  INVALID_ID: 'invalid_id',
  NOT_FOUND: 'not_found',
  INTERNAL: 'internal',
  VERSION_CONFLICT: 'version_conflict',
  CROSS_ORG: 'cross_org',
  INVALID_ORG_ID: 'invalid_org_id',
  INVALID_TRANSITION: 'invalid_transition',
  IMPORT_FAILED: 'import_failed',
  RATE_LIMITED: 'rate_limited',
} as const;
export type ErrorCode = (typeof ErrorCodes)[keyof typeof ErrorCodes];

// isApiError is a type guard. Use `unknown` for the input so the type
// system forces explicit narrowing in catch blocks.
export function isApiError(value: unknown): value is ApiError {
  return (
    typeof value === 'object' &&
    value !== null &&
    'error' in value &&
    typeof (value as { error: unknown }).error === 'string'
  );
}

// parseApiError attempts to read an HTTP Response as the canonical
// error body. Returns null when the body is not parseable as ApiError
// (per Claude's Discretion: "strict parse; network errors are caller's
// concern").
export async function parseApiError(response: Response): Promise<ApiError | null> {
  try {
    const body = (await response.json()) as unknown;
    return isApiError(body) ? body : null;
  } catch {
    return null;
  }
}
```

**Style conventions extracted from `tsconfig.base.json`:**
- `as const` assertions for closed enums (per D-39 autocomplete requirement)
- `(typeof X)[keyof typeof X]` pattern for deriving union from const record (idiomatic TS)
- `unknown` (not `any`) in type guards (`strict: true` rejects implicit any)
- Type-only imports: `import type { components } from './generated';`

---

### `web/packages/ui/src/api/task.ts` (Lit-aware async helper)

**Analog:** No in-repo TS analog. **External analog:** `@lit/task` documentation (https://lit.dev/docs/data/task/) — D-39 / canonical_refs.

**Pattern shape:**
```ts
import { Task } from '@lit/task';
import type { ReactiveControllerHost } from 'lit';
import type { ApiClient } from './client';

// createApiTask wraps @lit/task for the common admin SPA pattern:
//   "fetch X from API, render loading/error/data states".
//
// Phase 7 embed can skip this file and import only createApiClient.
export function createApiTask<TArgs extends readonly unknown[], TData>(
  host: ReactiveControllerHost,
  config: {
    task: (args: TArgs, signal: AbortSignal) => Promise<TData>;
    args: () => TArgs;
  }
): Task<TArgs, TData> {
  return new Task<TArgs, TData>(host, {
    task: config.task,
    args: config.args,
  });
}
```

**Style conventions:**
- Generic over `TArgs extends readonly unknown[]` (matches `@lit/task` signature)
- `import type { ReactiveControllerHost } from 'lit'` (type-only — `lit` is a peer dep declared in `packages/ui/package.json` once D-39 lands)
- Pass-through wrapper; not opinionated about the task body so admin SPA components can supply any async function

---

### `web/packages/ui/src/api/index.ts` (barrel)

**Analog:** `web/packages/ui/src/index.ts` (current `export {};` — Phase 1 stub; Phase 2 replaces with real exports).

**Pattern to extract:**
- Single `index.ts` per package, ESM `export` statements (no default exports — matches `verbatimModuleSyntax`)
- Barrel exports preserve named imports for consumers: `import { createApiClient } from '@open-routing/ui'`

**Apply pattern:**
```ts
// web/packages/ui/src/api/index.ts
export { createApiClient } from './client';
export type { ApiClient, CreateApiClientConfig } from './client';
export { isApiError, parseApiError, ErrorCodes } from './errors';
export type { ApiError, ErrorCode } from './errors';
export { createApiTask } from './task';
export type { paths, components, operations } from './generated';
```

```ts
// web/packages/ui/src/index.ts (replace `export {};` from Phase 1)
export * from './api';
```

This satisfies D-40's mandate: "Re-exported via `packages/ui` root `index.ts` so consumers write `import { createApiClient } from '@open-routing/ui'`."

---

### `.github/workflows/ci.yml` (add `codegen-drift` job)

**Analog:** Self — existing `migration-drift` job at `.github/workflows/ci.yml:97-123` is the closest pattern.

**Existing drift-gate pattern:**
```yaml
migration-drift:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_PASSWORD: drift
        options: >-
          --health-cmd "pg_isready"
          --health-interval 5s
          --health-timeout 3s
          --health-retries 5
        ports:
          - 5432:5432
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: services/api/go.mod
      - name: install migrate CLI
        run: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1
      - name: migrate up
        env:
          DATABASE_URL: "postgres://postgres:drift@localhost:5432/postgres?sslmode=disable"
        run: |
          migrate -path migrations -database "$DATABASE_URL" up
          migrate -path migrations -database "$DATABASE_URL" version
```

**Pattern to extract:**
1. **Job is its own top-level entry** under `jobs:` (D-47: "runs separately from `go-test` / `go-lint` / `web-typecheck` so a drift failure shows up as its own red check")
2. **Pinned tool versions** — `golangci-lint-action@v8`, `setup-go@v5`, `pnpm/action-setup@v4`, `setup-node@v4`. NEVER `@latest` (Phase 1 ASVS V14.3 requirement, comment at ci.yml lines 5-7).
3. **`actions/setup-go@v5` block with `go-version-file: services/api/go.mod`** — reuses existing block (D-47 explicit requirement).
4. **`pnpm/action-setup@v4` + `actions/setup-node@v4` with `cache: pnpm` + `cache-dependency-path: web/pnpm-lock.yaml`** — reuses existing block from `web-typecheck` (.github/workflows/ci.yml:59-77).
5. **`working-directory:` per-step** instead of `cd` in run scripts (chi convention for multi-module repos)

**Apply pattern (per D-47 + D-48):**
```yaml
codegen-drift:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      # Go side: oapi-codegen
      - uses: actions/setup-go@v5
        with:
          go-version-file: services/api/go.mod
          cache-dependency-path: services/api/go.sum
      # Node side: openapi-typescript + redocly lint
      - uses: pnpm/action-setup@v4
        with:
          version: 10
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - name: pnpm install
        working-directory: web
        run: pnpm install --frozen-lockfile
      # D-48: redocly lint BEFORE codegen
      - name: redocly lint openapi.yaml
        run: npx @redocly/cli@1.25.0 lint openapi/openapi.yaml --extends recommended
      # D-47: run codegen + diff
      - name: install task
        run: go install github.com/go-task/task/v3/cmd/task@v3.40.0
      - name: task gen
        run: $(go env GOPATH)/bin/task gen
      - name: git diff codegen output
        run: |
          git diff --exit-code services/api/internal/api/ web/packages/ui/src/api/generated.ts
```

(Planner pins `@redocly/cli` to a specific minor; CONTEXT.md says `npx`-invoked is fine because D-48 says no `package.json` dep — keeps lint surface decoupled from TS workspace.)

---

### `Taskfile.yml` (extend `gen` target)

**Analog:** Self — existing `gen` target at `Taskfile.yml:18-23` already invokes sqlc and is annotated as the Phase 2 extension point.

**Existing target:**
```yaml
# ---- Codegen ----
gen:
  desc: Run sqlc + (Phase 2) oapi-codegen + openapi-typescript
  cmds:
    # Phase 2: add oapi-codegen + openapi-typescript
    - cd services/api && sqlc generate
```

**Pattern to extract:**
- `task gen` is the single entry point for ALL codegen (D-08 + D-43 — "from one command")
- `cmds:` list runs sequentially; failures short-circuit
- `cd <dir> &&` prefix for sub-module commands (matches Taskfile convention; alternative is `dir:` key per-cmd but the current file uses inline cd)
- Comments inline the phase-progression intent (`# Phase 2: add oapi-codegen + openapi-typescript`)

**Apply pattern (per D-43):**
```yaml
# ---- Codegen ----
gen:
  desc: Run sqlc + oapi-codegen + openapi-typescript
  cmds:
    - cd services/api && sqlc generate
    - cd services/api && go generate ./...     # invokes oapi-codegen via //go:generate directive
    - cd web && pnpm -F @open-routing/ui gen:api
```

The `go generate ./...` command picks up a new `//go:generate` directive that the planner adds to a Go file inside `services/api/internal/api/`. Pattern reference: anywhere in the package, a single line:
```go
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.yaml ../../../openapi/openapi.yaml
```
(The `-config` and OpenAPI input paths are oapi-codegen v2 conventions; planner verifies exact flag names against tool docs.)

---

## Shared Patterns

### Pattern S1: Generated Code Discipline (`generated/` + drift gate)

**Source:** `services/api/internal/db/generated/*.go` + `services/api/.golangci.yml:19-20` + `.github/workflows/ci.yml:97-123`

**Apply to:**
- `services/api/internal/api/types.gen.go`, `server.gen.go`, `spec.gen.go`
- `web/packages/ui/src/api/generated.ts`

**Rules:**
1. **Committed to git** (NOT gitignored) — every consumer sees the same generated source as CI does.
2. **Lint exemptions** — for Go: add `path: internal/api/` to `services/api/.golangci.yml` exclusions matching the existing `internal/db/generated/` entry. For TS: add `src/api/generated.ts` to `.eslintignore` (or eslint config ignore block).
3. **CI drift gate** — `codegen-drift` job (D-47) fails on `git diff --exit-code` of generated files post-regen.
4. **Header comment from generator** — never edit; never add manual content to generated files. If a fix is needed, change the spec or the generator config and regenerate.

### Pattern S2: Bypass Routes at Root, `/v1/*` Behind OrgContext (D-21)

**Source:** `services/api/internal/server/server.go:74-96`

**Apply to:** New `/openapi.yaml` and `/docs` routes (D-45).

**Code excerpt to copy:**
```go
// (3) Bypass routes — D-21 says they MUST NOT carry OrgContext.
r.Get("/healthz", LiveHandler())
r.Get("/readyz", ReadyzHandler(deps.Pool, deps.Redis))
r.Get("/metrics", MetricsHandler())
// PHASE 2 ADDITIONS (D-45):
r.Get("/openapi.yaml", OpenAPISpecHandler(deps.SpecBytes))
r.Get("/docs", DocsHandler())

// (4) /v1 sub-router with OrgContext inside the Route closure.
r.Route("/v1", func(v1 chi.Router) {
    v1.Use(appmw.OrgContext)
    // v1.Mount(...)
})
```

**Enforcement seam:** Per Pitfall 1 in REQUIREMENTS, register bypass routes BEFORE the `r.Route("/v1", ...)` block. Never call `r.Use(appmw.OrgContext)` at root.

### Pattern S3: Canonical Error Envelope `{error, reason, request_id}`

**Source:** `services/api/internal/middleware/httputil.go:31-54` (extended for `request_id` per D-35)

**Apply to:**
- All hand-written error paths in `internal/api/*` (none in Phase 2 — strict-server generates them, but they use the same struct shape)
- All Go-side error responses across openapi.yaml `components.schemas.ErrorResponse`
- TypeScript `ApiError` type (`web/packages/ui/src/api/errors.ts`) — mirrored from the spec
- Strict-server `StrictMiddlewareFunc` that injects `request_id` from ctx into every response object

**Code excerpt (post-D-35 extension):**
```go
type errorBody struct {
    Error     string `json:"error"`
    Reason    string `json:"reason"`
    RequestID string `json:"request_id,omitempty"`
}
```

### Pattern S4: Authoritative `org_id` From Ctx (Never From URL/Body) (FOUND-05)

**Source:** `services/api/internal/scaffold/handler.go:106` (and lines 147, 176)

**Apply to:** Every strict-server handler in `scaffold/handler.go` post-migration, and every Phase 3/4/5 handler that mounts under `/v1/orgs/{org_id}/...`.

**Code excerpt:**
```go
orgID, ok := orgkey.OrgIDFromContext(ctx)
if !ok {
    // Defensive guard: OrgContext middleware guarantees presence inside
    // the /v1 sub-router; this branch only fires if the route is wired
    // outside that scope (a regression we want to surface loudly).
    return api.<Op>500JSONResponse{Error: "internal", Reason: "missing_org_id_in_context"}, nil
}
```

The URL `{org_id}` path parameter remains in the openapi.yaml spec (REST friendliness, log filtering) but handlers IGNORE it — read from ctx exclusively. This is the FOUND-08 cross-org isolation contract.

### Pattern S5: UUIDv7 at Write Boundary (D-19)

**Source:** `services/api/internal/scaffold/handler.go:126`

**Apply to:** Every strict-server `Create*` handler (scaffold migration + Phase 3 catalog CRUD).

**Code excerpt:**
```go
id := uuid.Must(uuid.NewV7())
```

`uuid.Must(uuid.NewV7())` panics only on entropy exhaustion (T-1-UUID-PANIC threat — acceptable per Phase 1 disposition). chi.Recoverer middleware catches and converts to 500. Do NOT use `uuid.NewV7()` without `Must` unless you're explicitly returning the entropy error.

### Pattern S6: `pgx.ErrNoRows` → 404 with `not_found` Code (FOUND-08)

**Source:** `services/api/internal/scaffold/handler.go:193-196`

**Apply to:** Every strict-server `Get*ById` and similar lookup handler.

**Code excerpt:**
```go
if err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        return api.<Op>404JSONResponse{Error: "not_found", Reason: "no_such_X"}, nil
    }
    return api.<Op>500JSONResponse{Error: "internal", Reason: "get_failed"}, nil
}
```

**Why this matters:** The sqlc query filters by both `id AND org_id`, so a cross-org probe (correct id, wrong org) returns `pgx.ErrNoRows` and maps to 404 — identical to a genuinely-missing row. This is the FOUND-08 leakage guard. **Never return 200 + null** or `403 forbidden` here — the response shape must be indistinguishable from a real 404.

### Pattern S7: pgtype.UUID Conversion (`toPgUUID` / `toResponse`)

**Source:** `services/api/internal/scaffold/handler.go:82-97`

**Apply to:** Every strict-server handler that calls sqlc with UUID parameters.

**Code excerpt:**
```go
func toPgUUID(id uuid.UUID) pgtype.UUID {
    return pgtype.UUID{Bytes: id, Valid: true}
}

func toResponse(s generated.Scaffold) scaffoldResponse {
    return scaffoldResponse{
        ID:         uuid.UUID(s.ID.Bytes),
        OrgID:      uuid.UUID(s.OrgID.Bytes),
        ExternalID: s.ExternalID,
        Name:       s.Name,
        CreatedAt:  s.CreatedAt.Time,
    }
}
```

**Why:** sqlc emits `pgtype.UUID` (struct with `Bytes [16]byte` + `Valid bool`) for both nullable AND NOT NULL columns. Default JSON encoding produces `{"Bytes":[...],"Valid":true}` — useless to API clients. Handler-side conversion to `google/uuid.UUID` + `time.Time` is required. Phase 3 + 4 catalog handlers MUST keep this pattern.

### Pattern S8: Codegen Config in `internal/<package>/<tool>.yaml` (D-43)

**Source:** `services/api/sqlc.yaml` (one level up from `internal/db/generated/`) — but the existing precedent is at the module root, not inside `internal/`. Phase 2 D-43 chooses to put `oapi-codegen.yaml` INSIDE `services/api/internal/api/` (matching D-42's package-local convention).

**Apply to:** `services/api/internal/api/oapi-codegen.yaml`

**Rationale:** sqlc.yaml lives at the module root because it predates the package structure decision. oapi-codegen.yaml living inside `internal/api/` keeps codegen config co-located with the generated output and the package it serves — better discoverability. This is a Phase 2 refinement of the Phase 1 sqlc location, not a contradiction.

### Pattern S9: TypeScript Factory Returning Strongly-Typed Client (D-38)

**Source:** No in-repo analog. Reference: D-38 explicit signature spec + `web/tsconfig.base.json` strict-mode rules.

**Apply to:** `createApiClient` in `web/packages/ui/src/api/client.ts`, and any future Lit components that consume `ApiClient`.

**Signature contract:**
```ts
export function createApiClient(config: {
  baseURL: string;
  getOrgId: () => string;
  fetch?: typeof fetch;
}): ApiClient;

export type ApiClient = ReturnType<typeof createClient<paths>>;
```

**Why factory vs singleton:** Admin SPA constructs one client at boot; Web Component embed instances may construct multiple clients with different `getOrgId` resolvers (one per Custom Element instance). `getOrgId` is invoked per request to enable embed attribute-change reactivity without client recreation.

### Pattern S10: ESM-Only TS Module Conventions (`tsconfig.base.json`)

**Source:** `web/tsconfig.base.json:1-15`

**Apply to:** All TS files in `web/packages/ui/src/api/*.ts`.

**Code excerpt:**
```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"]
  }
}
```

**Implications for new TS code:**
- `verbatimModuleSyntax: true` → split type-only imports: `import type { paths } from './generated';` vs `import createClient from 'openapi-fetch';`
- `isolatedModules: true` → no `const enum`, no namespace augmentation across files. Use `as const` records for enum-like values (D-39 `ErrorCodes`).
- `noUncheckedIndexedAccess: true` → indexed access into `paths` returns `T | undefined`; narrow before use.
- `strict: true` + `noImplicitOverride` → all function parameters explicitly typed; `override` keyword required when extending.
- `module: "ESNext"` + `"type": "module"` in package.json → top-level `await` allowed; no `require`; default exports discouraged (named exports are clearer for tree-shaking).

---

## No Analog Found

Files with no close match in the codebase (planner uses external references or D-XX explicit prescriptions):

| File | Role | Data Flow | Reason / Reference |
|------|------|-----------|--------------------|
| `.planning/phases/02-openapi-contract-codegen/02-UI-SKETCHES.md` | contract artifact | n/a | Greenfield markdown sketch. No in-repo or external code analog. Per D-33: ~30-50 lines per screen × 4 screens, ASCII art convention is Claude's Discretion. |
| `openapi/openapi.yaml` | contract spec | request-response | Greenfield. External reference: OpenAPI Specification 3.1.0 + REQUIREMENTS.md §CAT-01..CAT-11, §STATE-01..STATE-10, §IMP-01..IMP-08. The scaffold endpoints (D-46) get specced first as a smaller pattern; planner then duplicates the shape for catalog/state/import endpoints. |
| `web/packages/ui/src/api/client.ts` | TS module | request-response | Greenfield. External reference: openapi-fetch documentation. Signature shape locked by D-38. |
| `web/packages/ui/src/api/task.ts` | TS module | request-response | Greenfield. External reference: @lit/task documentation. Generic wrapper signature locked by D-39. |

---

## Metadata

**Analog search scope:**
- `services/api/internal/{api,scaffold,server,middleware,db,config,telemetry,testsupport}/`
- `services/api/cmd/{api,migrate}/`
- `services/api/{sqlc.yaml,tools.go,.golangci.yml,go.mod}`
- `web/{tsconfig.base.json,turbo.json,package.json,pnpm-workspace.yaml}`
- `web/packages/ui/{package.json,tsconfig.json,src/}`
- `web/apps/{admin,embed}/{package.json,tsconfig.json,src/}`
- `.github/workflows/ci.yml`
- `Taskfile.yml`
- `.planning/phases/01-foundation-polyglot-monorepo/01-PATTERNS.md` (cross-phase consistency check)

**Files scanned:** ~40 source files + ~10 config files

**Pattern extraction date:** 2026-05-15

**Cross-phase invariants enforced (Phase 1 carry-forward, MUST NOT regress):**
- D-17, D-21: bypass list at root, `/v1/*` behind OrgContext
- D-19: UUIDv7+ everywhere
- D-23: `internal/api/` is the home for generated code
- D-28: RequestID middleware mints UUIDv7
- D-30: CI shape — new jobs are additive, never replace existing required checks
- FOUND-05: authoritative org_id from ctx
- FOUND-08: cross-org probe returns 404, never 200

**Phase 2 D-decision → Pattern Assignment cross-reference:**
- D-32, D-33, D-34: `openapi/openapi.yaml` + sketches (greenfield; no analog)
- D-35: `httputil.go` extension (analog: self)
- D-36, D-37: `openapi.yaml` ErrorResponse schema + extension fields (greenfield; no analog)
- D-38, D-39, D-40: `web/packages/ui/src/api/{client,errors,task,index}.ts` (greenfield TS; tsconfig.base.json conventions apply)
- D-41, D-42: `services/api/internal/api/*.gen.go` (analog: sqlc generated/)
- D-43: `Taskfile.yml` gen target extension (analog: self)
- D-44: `server.go` NewMux extension (analog: self)
- D-45: `openapi.go` + `docs.html` new file (analog: health.go LiveHandler/MetricsHandler)
- D-46: `scaffold/handler.go` strict-server migration (analog: self — full rewrite)
- D-47: `ci.yml` codegen-drift job (analog: migration-drift job)
- D-48: redocly lint integration inside codegen-drift job (no analog; npx-invoked)
