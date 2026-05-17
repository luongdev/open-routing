package db

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
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

// TestOrgDB_BeginTx_PreservesValidator proves that the SQL validator
// applies to every Exec/Query/QueryRow call routed through an OrgTx
// (OQ-5, H5). Without this guarantee a handler that opens a transaction
// (e.g. the agent_skills full-replace in Plan 03-09) could bypass the
// validator and emit cross-org queries through the tx.
//
// The test constructs an OrgTx directly (not via BeginTx) so it can run
// without a real Postgres connection. The validator branching is in
// preflightSQL + handlePreflightError, both of which OrgTx and OrgDB
// share — exercising the tx path proves they cannot diverge.
func TestOrgDB_BeginTx_PreservesValidator(t *testing.T) {
	t.Parallel()

	// Sub-test: ValidationPanic mode on a tx WITHOUT org_id in ctx.
	// preflight must panic before the underlying tx is touched (nil tx
	// is safe — preflight fails first).
	t.Run("ExecPanicsOnMissingOrgIDInCtx", func(t *testing.T) {
		t.Parallel()
		tx := &OrgTx{tx: nil, checker: NewSQLChecker(), mode: ValidationPanic}
		ctx := context.Background()
		requirePanicIs(t, ErrOrgIDMissingFromContext, func() {
			_, _ = tx.Exec(ctx, `UPDATE _scaffold SET name = $1 WHERE org_id = $2`, "ignored", "ignored")
		})
	})

	// Sub-test: ValidationError mode on a tx WITHOUT org_id in ctx.
	// preflight must return ErrOrgIDMissingFromContext without panicking.
	t.Run("ExecReturnsErrorOnMissingOrgIDInCtx", func(t *testing.T) {
		t.Parallel()
		tx := &OrgTx{tx: nil, checker: NewSQLChecker(), mode: ValidationError}
		ctx := context.Background()
		requireNoPanic(t, func() {
			_, err := tx.Exec(ctx, `UPDATE _scaffold SET name = $1 WHERE org_id = $2`, "ignored", "ignored")
			if !errors.Is(err, ErrOrgIDMissingFromContext) {
				t.Fatalf("expected ErrOrgIDMissingFromContext, got %v", err)
			}
		})
	})

	// Sub-test: ValidationError mode on a tx — Query path.
	t.Run("QueryReturnsErrorOnMissingOrgIDInCtx", func(t *testing.T) {
		t.Parallel()
		tx := &OrgTx{tx: nil, checker: NewSQLChecker(), mode: ValidationError}
		ctx := context.Background()
		requireNoPanic(t, func() {
			_, err := tx.Query(ctx, `SELECT id FROM _scaffold WHERE org_id = $1`, "ignored")
			if !errors.Is(err, ErrOrgIDMissingFromContext) {
				t.Fatalf("expected ErrOrgIDMissingFromContext, got %v", err)
			}
		})
	})

	// Sub-test: QueryRow surfaces preflight failures via Scan, never panics
	// in ValidationError mode (mirrors OrgDB.QueryRow's errRow contract).
	t.Run("QueryRowSurfacesErrorViaScanInValidationError", func(t *testing.T) {
		t.Parallel()
		tx := &OrgTx{tx: nil, checker: NewSQLChecker(), mode: ValidationError}
		ctx := context.Background()
		row := tx.QueryRow(ctx, `SELECT id FROM _scaffold WHERE org_id = $1`, "ignored")
		if err := row.Scan(new(string)); !errors.Is(err, ErrOrgIDMissingFromContext) {
			t.Fatalf("expected ErrOrgIDMissingFromContext, got %v", err)
		}
	})

	// Sub-test: preflight rejects an unscoped SQL even when ctx HAS org_id
	// (D-02 SQLChecker enforces org_id in WHERE/INSERT-col-list).
	t.Run("ExecRejectsUnscopedSQLEvenWithOrgIDInCtx", func(t *testing.T) {
		t.Parallel()
		tx := &OrgTx{tx: nil, checker: NewSQLChecker(), mode: ValidationError}
		ctx := orgkey.SetOrgID(context.Background(), uuid.New())
		_, err := tx.Exec(ctx, `UPDATE _scaffold SET name = $1 WHERE id = $2`, "ignored", "ignored")
		if !errors.Is(err, ErrSQLMissingOrgFilter) {
			t.Fatalf("expected ErrSQLMissingOrgFilter, got %v", err)
		}
	})
}

// Note: a `TestOrgDB_BeginTx_RealTx` happy-path integration test using a
// postgres testcontainer is intentionally NOT included here. The preflight
// guarantee is the security-critical surface; the actual pgxpool→pgx.Tx
// chain is exercised by every package that uses BeginTx in its own
// integration test (services/api/test/isolation will add a tx-based probe
// alongside the catalog handlers in Wave 3 Plan 03-09). Adding a
// testcontainer-backed test here would add ~10s of startup overhead to
// the lightweight `go test -short` lane without a proportional safety
// payoff over the preflight sub-tests.
