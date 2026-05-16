---
phase: 02-openapi-contract-codegen
plan: "03"
subsystem: codegen-pipeline
tags: [oapi-codegen, strict-server, codegen, chi, openapi, generated-code, golangci]
dependency_graph:
  requires:
    - openapi/openapi.yaml (from plan 02-02)
    - services/api/tools.go
    - services/api/go.mod
    - services/api/.golangci.yml
    - Taskfile.yml
  provides:
    - services/api/internal/api/types.gen.go (DTO structs, 88 types)
    - services/api/internal/api/server.gen.go (StrictServerInterface + chi routing, 41 ops)
    - services/api/internal/api/spec.gen.go (embedded spec bytes via GetSwagger)
    - services/api/internal/api/gen.go (//go:generate directives)
    - services/api/internal/api/oapi-codegen.types.yaml
    - services/api/internal/api/oapi-codegen.server.yaml
    - services/api/internal/api/oapi-codegen.spec.yaml
  affects:
    - services/api/go.mod (oapi-codegen v2.7.0 + runtime v1.4.0 + go.yaml.in v3.0.4)
    - services/api/.golangci.yml (new exclusion rule for internal/api/)
    - Taskfile.yml (gen task extended)
    - openapi/openapi.yaml (downgraded OAS 3.1 → 3.0 for codegen compatibility)
    - Plan 04 (implements StrictServerInterface for Scaffold handlers)
    - Plan 06 (CI drift gate uses go generate ./... from Taskfile gen)
tech_stack:
  added:
    - "github.com/oapi-codegen/oapi-codegen/v2 v2.7.0 — Go server codegen tool"
    - "github.com/oapi-codegen/runtime v1.4.0 — runtime library for generated code"
    - "go.yaml.in/yaml/v3 v3.0.4 — transitive dep of oapi-codegen CLI (vanity domain pinned via GONOSUMDB)"
  patterns:
    - "Three-file split (D-42): types.gen.go + server.gen.go + spec.gen.go in package api"
    - "go generate ./internal/api/... driver pattern (D-43)"
    - "pkg/codegen blank import in tools.go to pin module version"
    - "go.yaml.in pinned in tools.go to survive go mod tidy"
key_files:
  created:
    - services/api/internal/api/gen.go
    - services/api/internal/api/oapi-codegen.types.yaml
    - services/api/internal/api/oapi-codegen.server.yaml
    - services/api/internal/api/oapi-codegen.spec.yaml
    - services/api/internal/api/types.gen.go
    - services/api/internal/api/server.gen.go
    - services/api/internal/api/spec.gen.go
  modified:
    - services/api/tools.go
    - services/api/go.mod
    - services/api/go.sum
    - services/api/.golangci.yml
    - Taskfile.yml
    - openapi/openapi.yaml
decisions:
  - "oapi-codegen v2.7.0 selected (v2.4.1 resolved but had OAS 3.1 type resolution issues; v2.5.1/v2.6.0 had compilation errors with kin-openapi v0.135.0; v2.7.0 is stable at the correct dep versions)"
  - "Three-file split (canonical D-42): types.gen.go + server.gen.go + spec.gen.go; single-file fallback NOT needed"
  - "go.yaml.in/yaml/v3 pinned in tools.go as blank import to survive go mod tidy after GONOSUMDB fetch"
  - "openapi/openapi.yaml downgraded OAS 3.1 → 3.0 (Rule 1 bug fix: oapi-codegen v2 does not support OAS 3.1 nullable type arrays per github.com/oapi-codegen/oapi-codegen/issues/373)"
  - "oapi-codegen/runtime v1.4.0 added as direct dep (required by generated code at compile time)"
metrics:
  duration: "approx 45 minutes"
  completed: "2026-05-16"
  tasks_completed: 2
  files_created: 7
  files_modified: 6
---

# Phase 2 Plan 03: oapi-codegen Pipeline + Generated Strict-Server Stubs Summary

**One-liner:** oapi-codegen v2.7.0 pinned in tools.go; three-file strict-server split (types.gen.go + server.gen.go + spec.gen.go) generated from openapi.yaml via go generate, lint-exempt, and wired into Taskfile gen target.

## What Was Built

### Task 1 — Pin oapi-codegen + Write Codegen Configs

Extended `services/api/tools.go` to blank-import `github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen`
(pinning the module version) and `go.yaml.in/yaml/v3` (keeping the oapi-codegen CLI's transitive
dep in go.sum after `go mod tidy`). Pinned oapi-codegen v2.7.0 in go.mod.

Created three codegen config files (canonical D-42 three-file split):
- `oapi-codegen.types.yaml` → `types.gen.go` (DTO structs, `models: true`)
- `oapi-codegen.server.yaml` → `server.gen.go` (chi-server + strict-server, `chi-server: true` + `strict-server: true`)
- `oapi-codegen.spec.yaml` → `spec.gen.go` (embedded bytes, `embedded-spec: true`)

Created `internal/api/gen.go` with three `//go:generate` directives (one per config file).

**Key deviation (Rule 1 — Bug Fix): OAS 3.1 → 3.0 downgrade of openapi.yaml**

oapi-codegen v2.7.0 does not support OAS 3.1 nullable type arrays
(`type: [string, null]`) per https://github.com/oapi-codegen/oapi-codegen/issues/373.
The spec was downgraded from `openapi: 3.1.0` to `openapi: 3.0.0`:

- 6 primitive nullable fields converted: `type: [string, null]` → `type: string\nnullable: true`
- 8 `$ref` nullable fields converted: `anyOf: [{$ref}, {type: null}]` → `allOf: [{$ref}]\ntype: string\nnullable: true`
- 1 object nullable field: `anyOf: [{type: object}, {type: null}]` → `type: object\nnullable: true`
- 1 array nullable field: `anyOf: [{type: array, items: {$ref}}, {type: null}]` → `type: array\nnullable: true\nitems: {$ref}`

All semantic contract content is preserved. Redocly lint: **0 errors, 5 warnings** (unchanged from plan 02-02).

### Task 2 — Run Codegen, Commit Generated Files, Lint + Taskfile

Ran `go generate ./internal/api/...` to produce the three-file split:

| File | Lines | Contents |
|------|-------|----------|
| `types.gen.go` | 1259 | 88 DTO types from components.schemas |
| `server.gen.go` | 6422 | `StrictServerInterface` (41 methods) + `ServerInterface` + chi routing |
| `spec.gen.go` | 338 | `GetSwagger()` returning embedded openapi.yaml bytes |
| **Total** | **8019** | |

Added `github.com/oapi-codegen/runtime v1.4.0` to go.mod (required by generated code).

**W-3 Layout Choice: Three-file split (canonical, preferred)**

- `types.gen.go` exists: YES
- `server.gen.go` exists: YES
- `spec.gen.go` exists: YES
- Single-file fallback: NOT needed

Added golangci-lint exclusion rule in `.golangci.yml`:
```yaml
- path: internal/api/
  linters: [gosec, errcheck, gocritic, staticcheck]
```
(mirrors the existing `internal/db/generated/` exclusion)

Extended `Taskfile.yml` `gen` target:
```yaml
cmds:
  - cd services/api && sqlc generate
  - cd services/api && go generate ./internal/api/...
  - cd web && pnpm -F @open-routing/ui gen:api   # green after Plan 05
```
The pnpm line will fail until Plan 05 adds the `gen:api` script to `packages/ui/package.json`.

**Determinism verified:** Second `go generate` run produces empty git diff (exit 0).

## oapi-codegen Version Details

- **Pinned version:** `github.com/oapi-codegen/oapi-codegen/v2 v2.7.0`
- **Runtime dep:** `github.com/oapi-codegen/runtime v1.4.0`
- **Tool binary:** invoked via `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen`
  (no global install required; module cache handles binary)

Version selection rationale:
- v2.4.1: compiled, but `type: [string, null]` in OAS 3.1 failed at code-generation time
- v2.5.1, v2.6.0: failed compilation (kin-openapi v0.135.0 broke `element.Ref` type)
- v2.7.0: stable with kin-openapi v0.135.0; `go run` requires `go.yaml.in/yaml/v3` in go.sum

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Pin oapi-codegen + three-file split configs | e260f81 | tools.go, go.mod, go.sum, openapi.yaml, gen.go, 3x oapi-codegen.*.yaml |
| 2 | Generate strict-server stubs, lint exemption, Taskfile | cdc33dc | types.gen.go, server.gen.go, spec.gen.go, .golangci.yml, Taskfile.yml, go.mod, go.sum |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] OAS 3.1 nullable type arrays not supported by oapi-codegen**
- **Found during:** Task 1 — first codegen run with v2.4.1 and v2.7.0
- **Issue:** `type: [string, null]` (OAS 3.1 nullable array syntax) causes
  `error resolving primitive type: unhandled Schema type: &[string null]` in all
  tested oapi-codegen v2 versions. The warning `"You are using an OpenAPI 3.1.x
  specification, which is not yet supported"` confirms this is a known limitation
  (github.com/oapi-codegen/oapi-codegen/issues/373).
- **Fix:** Downgraded `openapi/openapi.yaml` from `openapi: 3.1.0` to `openapi: 3.0.0`.
  Converted 16 nullable field declarations to OAS 3.0 syntax (nullable: true).
  Semantic contract unchanged. Redocly lint passes with 0 errors.
- **Files modified:** `openapi/openapi.yaml`
- **Commit:** e260f81

**2. [Rule 1 - Bug] cmd/oapi-codegen is a main package (not blank-importable)**
- **Found during:** Task 1 — `go vet -tags tools ./...` exit 1
- **Issue:** The plan template imports `cmd/oapi-codegen` directly in tools.go, but
  that's a main package. `go vet` exits 1 with "import is a program, not an
  importable package".
- **Fix:** Changed to import `github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen`
  (a library package from the same module) to pin the module version.
- **Files modified:** `services/api/tools.go`
- **Commit:** e260f81

**3. [Rule 1 - Bug] go.yaml.in/yaml/v3 vanity domain breaks go.sum after mod tidy**
- **Found during:** Task 1 — `go run` for cmd/oapi-codegen missing go.sum entry
- **Issue:** `go.yaml.in` is a vanity domain that does not resolve via the Go checksum
  database (GOPROXY). After `go mod tidy`, the entry for `go.yaml.in/yaml/v3` is
  removed from go.sum because no importable package in the module graph references it
  directly. Subsequent `go run cmd/oapi-codegen` fails with "missing go.sum entry".
- **Fix:** Added `go.yaml.in/yaml/v3` as a blank import in tools.go so mod tidy keeps it.
  Requires `GONOSUMDB=go.yaml.in go mod tidy` on first setup (or re-setup).
  Documented in gen.go package comment.
- **Files modified:** `services/api/tools.go`, `services/api/go.mod`, `services/api/go.sum`
- **Commit:** e260f81

**4. [Rule 3 - Missing dep] oapi-codegen/runtime missing from go.mod**
- **Found during:** Task 2 — `go build ./...` failed
- **Issue:** Generated code imports `github.com/oapi-codegen/runtime` and
  `github.com/oapi-codegen/runtime/types` which are not in go.mod.
- **Fix:** `go get github.com/oapi-codegen/runtime` added v1.4.0 + apapsch/go-jsonmerge.
- **Files modified:** `services/api/go.mod`, `services/api/go.sum`
- **Commit:** cdc33dc

## Threat Flags

None beyond what was registered in the plan's threat model:

- T-02-SC (supply chain): oapi-codegen v2.7.0 is the established community tool
  (DeepMap origin, now CNCF-adjacent governance). Pinned via go.mod require.
- T-02-SC-02 (generated output injection): Scanned all *.gen.go files for
  `eval`, `exec`, `os.Setenv`, `os/exec` import, unsafe package — **none found**.
  Only legitimate imports: `net/http` (HTTP server), `context`, `encoding/json`,
  `fmt`, `io`, `net/url`, `strings`, `time`, `github.com/go-chi/chi/v5`,
  `github.com/oapi-codegen/runtime`.
- T-02-INF-01 (spec embedded bytes): accepted per plan — GetSwagger() embeds the exact
  spec the binary was built against (D-45 desired behavior).

## Known Stubs

None. Both deliverables are complete and fully wired:
1. `go generate ./internal/api/...` is functional and deterministic
2. Generated `StrictServerInterface` has all 41 operationId methods
3. `GetSwagger()` in spec.gen.go returns embedded spec bytes
4. golangci-lint exclusion in effect
5. Taskfile gen extended (pnpm line pending Plan 05)

The pnpm line in `task gen` will fail until Plan 05 adds `gen:api` to `packages/ui`.
This is intentional and documented in the Taskfile comment.

## Self-Check: PASSED

- [x] `gen.go` exists: `services/api/internal/api/gen.go`
- [x] `server.gen.go` exists: `services/api/internal/api/server.gen.go`
- [x] `types.gen.go` exists: `services/api/internal/api/types.gen.go`
- [x] `spec.gen.go` exists: `services/api/internal/api/spec.gen.go`
- [x] SUMMARY.md exists: `.planning/phases/02-openapi-contract-codegen/02-03-SUMMARY.md`
- [x] Task 1 commit `e260f81` exists in git log
- [x] Task 2 commit `cdc33dc` exists in git log
- [x] `go build ./...` passes (confirmed)
- [x] `go.mod` has `github.com/oapi-codegen/oapi-codegen/v2 v2.7.0`
- [x] `StrictServerInterface` has 41 methods (one per operationId in spec)
- [x] `GetSwagger()` accessible in `spec.gen.go`
- [x] Three-file split: `types.gen.go` + `server.gen.go` + `spec.gen.go`
- [x] golangci-lint exclusion: `path: internal/api/` in `.golangci.yml`
- [x] Taskfile gen extended with `go generate ./internal/api/...`
- [x] Codegen determinism: second run produces empty diff
- [x] Phase 1 tests pass (TestWriteError|TestRequestID|TestOrgContext)
- [x] Security scan: no eval/exec/os.Setenv in generated files
- [x] Redocly lint: 0 errors, 5 warnings on downgraded OAS 3.0 spec
