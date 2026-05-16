// testutil_test.go — shared per-entity test scaffolding (D-73).
//
// Filename suffix `_test.go` is the idiomatic Go convention for
// test-only files: the Go build tool excludes _test.go files from
// production builds, so miniredis, testify, and other test-only deps
// never end up in the shipped binary. No build tag (`//go:build test`)
// is needed — the _test.go suffix alone gates the file.
//
// This is the Plan 03-05 SKELETON — Plan 03-06 fleshes out
// newTestHandlers, cleanCatalogTables, httpPOST/httpGET, and the
// top-level main_test.go TestMain that brings up the shared pg
// testcontainer. Plan 03-05 ships only the `sharedPool` package-var
// declaration so cursor_test.go compiles standalone in the scaffold.
package catalog

import "github.com/jackc/pgx/v5/pgxpool"

// sharedPool is the package-level pgxpool reused across every entity
// _test.go in the catalog package (D-73). Plan 03-06 creates a top-
// level main_test.go that wires `sharedPool` to a real testcontainer
// pool in TestMain. Until then, sharedPool stays nil and any helper
// that needs it (newTestHandlers, cleanCatalogTables) MUST t.Skip()
// when sharedPool == nil — pattern documented in 03-05 PLAN.md.
var sharedPool *pgxpool.Pool //nolint:unused // referenced by Plan 03-06 helpers
