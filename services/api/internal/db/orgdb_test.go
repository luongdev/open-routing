package db

import (
	"context"
	"errors"
	"testing"
)

func requirePanicIs(t *testing.T, want error, fn func()) {
	t.Helper()

	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("expected panic")
		}
		err, ok := got.(error)
		if !ok {
			t.Fatalf("expected error panic, got %T", got)
		}
		if !errors.Is(err, want) {
			t.Fatalf("expected panic wrapping %v, got %v", want, err)
		}
	}()

	fn()
}

func requireNoPanic(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if got := recover(); got != nil {
			t.Fatalf("expected no panic, got %v", got)
		}
	}()

	fn()
}

// TestOrgDB_Exec_ValidationPanic_NoOrg proves ValidationPanic panics for
// missing org_id in ctx before touching the pool.
func TestOrgDB_Exec_ValidationPanic_NoOrg(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	checker := NewSQLChecker()
	o := NewOrgDB(nil, checker, ValidationPanic)

	requirePanicIs(t, ErrOrgIDMissingFromContext, func() {
		_, _ = o.Exec(ctx, `UPDATE _scaffold SET name = $1 WHERE org_id = $2`, "ignored", "ignored")
	})
}

// TestOrgDB_Query_ValidationPanic_NoOrg proves ValidationPanic panics for
// missing org_id in ctx before touching the pool.
func TestOrgDB_Query_ValidationPanic_NoOrg(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	checker := NewSQLChecker()
	o := NewOrgDB(nil, checker, ValidationPanic)

	requirePanicIs(t, ErrOrgIDMissingFromContext, func() {
		_, _ = o.Query(ctx, `SELECT id FROM _scaffold WHERE org_id = $1`, "ignored")
	})
}

// TestOrgDB_Exec_ValidationError_NoOrg proves ValidationError returns a typed
// error without panicking when ctx lacks org_id.
func TestOrgDB_Exec_ValidationError_NoOrg(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	checker := NewSQLChecker()
	o := NewOrgDB(nil, checker, ValidationError)

	requireNoPanic(t, func() {
		if _, err := o.Exec(ctx, `UPDATE _scaffold SET name = $1 WHERE org_id = $2`, "ignored", "ignored"); !errors.Is(err, ErrOrgIDMissingFromContext) {
			t.Fatalf("expected ErrOrgIDMissingFromContext, got %v", err)
		}
	})
}

// TestOrgDB_Query_ValidationError_NoOrg proves ValidationError returns a typed
// error without panicking when ctx lacks org_id.
func TestOrgDB_Query_ValidationError_NoOrg(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	checker := NewSQLChecker()
	o := NewOrgDB(nil, checker, ValidationError)

	requireNoPanic(t, func() {
		if _, err := o.Query(ctx, `SELECT id FROM _scaffold WHERE org_id = $1`, "ignored"); !errors.Is(err, ErrOrgIDMissingFromContext) {
			t.Fatalf("expected ErrOrgIDMissingFromContext, got %v", err)
		}
	})
}

// TestOrgDB_QueryRow_ValidationError_NoOrg proves that QueryRow in
// ValidationError mode returns a typed error from Scan instead of panicking
// when preflight fails due to a missing org_id in ctx (D-02, HIGH-3).
//
// A nil pool is safe here: preflight fails on missing ctx org_id before
// pool.QueryRow is ever called, so no real DB connection is required.
func TestOrgDB_QueryRow_ValidationError_NoOrg(t *testing.T) {
	t.Parallel()
	// ctx has no org_id — preflight returns ErrOrgIDMissingFromContext.
	ctx := context.Background()
	checker := NewSQLChecker()
	// nil pool: QueryRow only reaches pool.QueryRow if preflight passes;
	// preflight fails on missing ctx org_id before touching the pool.
	o := NewOrgDB(nil, checker, ValidationError)
	row := o.QueryRow(ctx, `SELECT id FROM _scaffold WHERE org_id = $1`, "ignored")
	if err := row.Scan(new(string)); !errors.Is(err, ErrOrgIDMissingFromContext) {
		t.Fatalf("expected ErrOrgIDMissingFromContext, got %v", err)
	}
}
