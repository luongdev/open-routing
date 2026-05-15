// Package orgkey holds the canonical context key for the request-scoped org_id.
// Both internal/db and internal/middleware import this package to avoid a
// bidirectional import cycle. This is a leaf package — it has zero
// internal-package dependencies and depends only on `context` (stdlib) and
// `github.com/google/uuid`.
//
// The unexported empty-struct key type (S1) prevents ctx-key collisions across
// packages: any other package that wants to write the same ctx key would have
// to import orgkey, which means there is exactly one canonical writer.
package orgkey

import (
	"context"

	"github.com/google/uuid"
)

// orgIDKey is the unexported context key type. Empty struct + unexported name
// makes this package the only legal source for the key (S1).
type orgIDKey struct{}

// SetOrgID returns a copy of ctx carrying the org_id. Always store as
// uuid.UUID (not string) per D-20 — gives downstream consumers type safety.
func SetOrgID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, orgIDKey{}, id)
}

// OrgIDFromContext returns the stored org_id and a presence flag.
// Returns (uuid.Nil, false) when ctx has no org_id (pre-middleware code
// paths, bypass paths like /healthz, or tests that did not call SetOrgID).
func OrgIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(orgIDKey{}).(uuid.UUID)
	return id, ok
}
