// Package testsupport bundles reusable test infrastructure: a testcontainers
// Postgres 17 helper (postgres.go), programmatic golang-migrate application
// (migrate.go), fresh UUIDv7 mint utilities (uuid.go), and the minimal
// HTTP helper (httpclient.go::DoBare) used by FOUND-08 isolation tests
// when they need to drive a request with hand-crafted headers (e.g.
// no X-Org-Id, malformed X-Org-Id, header-vs-URL divergence).
//
// Per D-06 every Go test package that touches Postgres should spin up its
// own container via StartPostgres (TestMain). This package is the single
// home for that bootstrap so internal/db and test/isolation share one
// implementation.
//
// Per Pitfall 6 / S8 every test mints fresh UUIDv7 org_ids — never reuse a
// package-level constant. FreshOrgID(t) is the only legitimate ID source.
package testsupport
