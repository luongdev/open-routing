// Package testsupport bundles reusable test infrastructure: a testcontainers
// Postgres 17 helper, programmatic golang-migrate application, fresh UUIDv7
// mint utilities, and HTTP helpers for exercising the API via httptest.
//
// Per D-06 every Go test package that touches Postgres should spin up its
// own container via StartPostgres (TestMain). This file is the single home
// for that bootstrap so internal/db, internal/scaffold, and test/isolation
// all share one implementation.
//
// Per Pitfall 6 / S8 every test mints fresh UUIDv7 org_ids — never reuse a
// package-level constant. FreshOrgID(t) is the only legitimate ID source.
//
// The HTTP helpers (PostScaffold, ListScaffolds, GetScaffoldStatus, DoBare,
// SeedScaffold) drive requests through net/http against an httptest.Server
// wrapping the production chi mux — per D-07 the FOUND-08 isolation proof
// MUST exercise the full HTTP chain, not just direct DB writes.
package testsupport
