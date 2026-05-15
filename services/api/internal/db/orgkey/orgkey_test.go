package orgkey

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestSetThenGet_Roundtrip proves that SetOrgID followed by OrgIDFromContext
// returns the exact same uuid.UUID value with ok=true.
func TestSetThenGet_Roundtrip(t *testing.T) {
	t.Parallel()
	want := uuid.Must(uuid.NewV7())
	ctx := SetOrgID(context.Background(), want)

	got, ok := OrgIDFromContext(ctx)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	if got != want {
		t.Fatalf("orgkey roundtrip mismatch: want=%s got=%s", want, got)
	}
}

// TestOrgIDFromContext_MissingKey proves that a ctx without SetOrgID
// returns (uuid.Nil, false). This is the contract bypass paths and
// pre-middleware code paths rely on (slog TracingHandler in Plan 04
// uses the bool to decide whether to attach the org_id log field).
func TestOrgIDFromContext_MissingKey(t *testing.T) {
	t.Parallel()
	got, ok := OrgIDFromContext(context.Background())
	if ok {
		t.Fatalf("expected ok=false for naked ctx, got true with id=%s", got)
	}
	if got != uuid.Nil {
		t.Fatalf("expected uuid.Nil when missing, got %s", got)
	}
}

// TestSetOrgID_ParentNotMutated proves SetOrgID returns a derived ctx
// (parent is unchanged) — standard context.WithValue semantics.
func TestSetOrgID_ParentNotMutated(t *testing.T) {
	t.Parallel()
	parent := context.Background()
	_ = SetOrgID(parent, uuid.Must(uuid.NewV7()))
	if _, ok := OrgIDFromContext(parent); ok {
		t.Fatalf("parent ctx must NOT carry org_id after SetOrgID returns a derived ctx")
	}
}
