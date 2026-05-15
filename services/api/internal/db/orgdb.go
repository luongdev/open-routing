package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// ValidationMode selects how the SQL inspector reacts to a missing
// org_id filter (D-02). Dev/test installs use ValidationPanic so a
// regression surfaces immediately as an unrecoverable failure; prod
// installs use ValidationError so the request fails with a typed error
// but the process keeps serving other requests.
type ValidationMode int

const (
	// ValidationPanic — dev / test: preflight failure panics. Default.
	ValidationPanic ValidationMode = iota
	// ValidationError — prod: preflight failure returns the error
	// without panicking. Handler maps it to HTTP 500 + structured log.
	ValidationError
)

// ErrOrgIDMissingFromContext is returned when ctx reaches the db layer
// without an org_id (D-03). Indicates the request bypassed OrgContext
// middleware — either a routing bug or a code path that constructed a
// fresh ctx instead of deriving from the request ctx. The middleware
// bypass list (/healthz, /readyz, /metrics) never reaches orgDB so this
// error always indicates a regression.
var ErrOrgIDMissingFromContext = errors.New("orgdb: ctx missing org_id; refusing to query")

// OrgDB wraps *pgxpool.Pool and intercepts every SQL call to enforce
// org_id scoping (D-01). Constructed once per process; safe for
// concurrent use (the underlying pgxpool.Pool is itself goroutine-safe
// and SQLChecker is guarded by sync.RWMutex).
//
// OrgDB satisfies the sqlc-generated generated.DBTX interface — see the
// compile-time assertion at the bottom of this file (D-01, Shared
// Pattern S9). Handlers in Plan 06 will hand OrgDB to generated.New so
// every *Queries call routes through preflight.
type OrgDB struct {
	pool    *pgxpool.Pool
	checker *SQLChecker
	mode    ValidationMode
}

// NewOrgDB constructs the wrapper. checker is shared process-wide so the
// SHA-256 cache benefits from cross-handler reuse; pass the same
// *SQLChecker to every OrgDB instance you construct.
func NewOrgDB(pool *pgxpool.Pool, checker *SQLChecker, mode ValidationMode) *OrgDB {
	return &OrgDB{pool: pool, checker: checker, mode: mode}
}

// Exec runs preflight then delegates to pgxpool.Pool.Exec. Signature
// matches sqlc-generated DBTX exactly (positional ctx, no Context suffix,
// args ...interface{}).
func (o *OrgDB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if err := o.preflight(ctx, sql); err != nil {
		return pgconn.CommandTag{}, o.handlePreflightError("Exec", err)
	}
	return o.pool.Exec(ctx, sql, args...)
}

// Query runs preflight then delegates to pgxpool.Pool.Query. Same
// signature constraint as Exec.
func (o *OrgDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if err := o.preflight(ctx, sql); err != nil {
		return nil, o.handlePreflightError("Query", err)
	}
	return o.pool.Query(ctx, sql, args...)
}

// errRow is returned by QueryRow when preflight fails in ValidationError mode.
// Scan always returns the stored error, satisfying pgx.Row without panicking.
type errRow struct{ err error }

func (r errRow) Scan(_ ...any) error { return r.err }

// QueryRow runs preflight then delegates to pgxpool.Pool.QueryRow.
// On preflight failure: panics in ValidationPanic mode (dev/test), returns
// errRow in ValidationError mode (prod) so the error surfaces at Scan
// without crashing the process. Callers reach this only via sqlc-generated
// :one queries; in prod a preflight failure indicates missing org_id scoping.
func (o *OrgDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if err := o.preflight(ctx, sql); err != nil {
		return errRow{err: o.handlePreflightError("QueryRow", err)}
	}
	return o.pool.QueryRow(ctx, sql, args...)
}

func (o *OrgDB) handlePreflightError(op string, err error) error {
	if o.mode == ValidationPanic {
		panic(fmt.Errorf("orgdb.%s preflight failed: %w", op, err))
	}
	return err
}

// preflight is the gate every DB call passes through. Three steps:
//
//  1. Bypass marker present → skip remaining checks but emit
//     slog.Warn(event=orgdb_bypass) with reason + caller + sql_hash so
//     the audit trail captures who authorized the bypass (D-04, D-05).
//  2. ctx must carry org_id (D-03). This proves the request has been
//     through OrgContext middleware. Presence does NOT mean the org_id
//     reaches the SQL — that is step 3's job.
//  3. SQL must reference org_id (D-02), validated by SQLChecker. On
//     failure: panic in ValidationPanic, return error in ValidationError.
//
// The slog event shape (event, reason, caller, sql_hash, org_id_attempted)
// matches what cmd/migrate emits at startup so a future audit-events
// table can ingest both producers with one schema.
func (o *OrgDB) preflight(ctx context.Context, sql string) error {
	if reason, ok := BypassReason(ctx); ok {
		caller, _ := BypassCaller(ctx)
		var orgIDAttempted string
		if id, present := orgkey.OrgIDFromContext(ctx); present {
			orgIDAttempted = id.String()
		}
		slog.WarnContext(ctx, "orgdb bypass",
			"event", "orgdb_bypass",
			"reason", reason,
			"caller", caller,
			"sql_hash", hashSQL(sql),
			"org_id_attempted", orgIDAttempted,
		)
		return nil
	}

	if _, ok := orgkey.OrgIDFromContext(ctx); !ok {
		return ErrOrgIDMissingFromContext
	}

	if err := o.checker.MustContainOrgFilter(sql); err != nil {
		return err
	}
	return nil
}

// Compile-time guarantee: OrgDB satisfies the sqlc-generated DBTX
// interface (D-01, Shared Pattern S9). If sqlc regenerates with a
// different signature this fails to compile rather than silently
// permitting a divergent wrapper.
var _ generated.DBTX = (*OrgDB)(nil)

// Compile-time guarantee: errRow satisfies pgx.Row. pgx v5 reserves the right
// to add methods to Row outside semver; this assertion catches any such
// addition at build time before a runtime failure.
var _ pgx.Row = errRow{}
