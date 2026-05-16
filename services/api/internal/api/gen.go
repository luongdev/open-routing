// Package api hosts the oapi-codegen output for the v0.1 OpenAPI spec
// at openapi/openapi.yaml (D-41, D-42). The hand-written content of this
// file is the //go:generate directives; the rest of the package's source
// files (*.gen.go) are produced by `go generate ./...`.
//
// Codegen invariants (Phase 2 D-41..D-43):
//   - strict-server mode: typed request/response objects + StrictServerInterface
//   - chi-server: transport layer that strict-server wraps
//   - generated files committed; drift gate in CI fails on any uncommitted diff
//   - oapi-codegen config(s) pinned; do not pass flags on the CLI
//
// Layout (D-42, W-3): three-file split
//   - types.gen.go   → components.schemas DTO structs
//   - server.gen.go  → StrictServerInterface + ServerInterface + chi route wiring
//   - spec.gen.go    → embedded openapi.yaml bytes via GetSwagger()
//
// Note on spec version: the input openapi/openapi.yaml was authored as OAS 3.1
// but is stored as OAS 3.0 for oapi-codegen v2 compatibility (the tool does not
// yet fully support OAS 3.1 nullable type arrays; see github.com/oapi-codegen/oapi-codegen/issues/373).
// All semantic content is preserved; nullable fields use OAS 3.0 nullable: true syntax.
//
// Note on go.yaml.in: oapi-codegen v2.7.0's cmd/ entrypoint imports go.yaml.in/yaml/v3
// which uses a non-standard vanity domain. It is pinned in tools.go and go.sum.
// If `go mod tidy` removes it, re-run: GONOSUMDB=go.yaml.in go mod tidy.
//
// Plan 04 implements StrictServerInterface for the Scaffold routes (D-46).
// Plan 06 enforces the codegen-drift CI gate (D-47).
package api

// Three-file split (canonical per D-42):

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.types.yaml ../../../../openapi/openapi.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.server.yaml ../../../../openapi/openapi.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen.spec.yaml ../../../../openapi/openapi.yaml
