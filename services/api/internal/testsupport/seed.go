package testsupport

import (
	"testing"

	"github.com/google/uuid"
)

// ScaffoldSeed is a single row spec for batch seeding.
//
// Two-field struct (no id, no org_id, no created_at): id is server-minted
// (UUIDv7 per D-19), org_id comes from the X-Org-Id header, created_at is
// a Postgres default. Only external_id and name are caller-supplied — the
// same shape the createRequest body uses in scaffold/handler.go.
type ScaffoldSeed struct {
	ExternalID string
	Name       string
}

// SeedScaffold creates each row via PostScaffold so the test exercises the
// full HTTP chain (not direct DB inserts). This is intentional: it mirrors
// how Phase 1 / Phase 3+ tests should populate fixtures.
//
// Anti-pattern guard: per VALIDATION.md §"Two-Org Isolation Specification"
// the spec is "two orgs seeded with IDENTICAL external_id values". If the
// helper used pool.Exec directly it would bypass OrgContext + orgDB SQL
// validation entirely — the isolation test would then pass even on a
// regression that broke the middleware. By routing through PostScaffold the
// seeds traverse the exact path a real client would, so a regression that
// breaks ANY interception layer surfaces as a seeding failure.
func SeedScaffold(t testing.TB, baseURL string, orgID uuid.UUID, items []ScaffoldSeed) []ScaffoldRow {
	t.Helper()
	rows := make([]ScaffoldRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, PostScaffold(t, baseURL, orgID, item.ExternalID, item.Name))
	}
	return rows
}
