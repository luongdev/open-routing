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
		return pgconn.CommandTag{}, handlePreflightError(o.mode, "Exec", err)
	}
	return o.pool.Exec(ctx, sql, args...)
}

// Query runs preflight then delegates to pgxpool.Pool.Query. Same
// signature constraint as Exec.
func (o *OrgDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if err := o.preflight(ctx, sql); err != nil {
		return nil, handlePreflightError(o.mode, "Query", err)
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
		return errRow{err: handlePreflightError(o.mode, "QueryRow", err)}
	}
	return o.pool.QueryRow(ctx, sql, args...)
}

// handlePreflightError converts a preflight failure into either a panic
// (ValidationPanic — dev/test) or a typed error (ValidationError — prod).
// Extracted to a package-level function so OrgTx can reuse the exact
// branching (OQ-5; the tx wrapper would otherwise diverge from the pool
// wrapper subtly).
func handlePreflightError(mode ValidationMode, op string, err error) error {
	if mode == ValidationPanic {
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
	return preflightSQL(ctx, sql, o.checker)
}

// preflightSQL is the shared bypass-or-validate logic used by both
// OrgDB and OrgTx (OQ-5). Extracted as a package-level function so the
// transaction wrapper does not re-implement (and subtly diverge from)
// the pool wrapper's enforcement. The mode-specific panic/error branch
// lives in handlePreflightError so callers control the call-site message
// (operation name) without forking the validator.
func preflightSQL(ctx context.Context, sql string, checker *SQLChecker) error {
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

	if err := checker.MustContainOrgFilter(sql); err != nil {
		return err
	}
	return nil
}

// OrgTx wraps pgx.Tx and re-applies preflight on every Exec/Query/QueryRow
// so transactions preserve the FOUND-04/D-02 SQL validator guarantee
// (OQ-5, H5). Constructed via (*OrgDB).BeginTx; Commit/Rollback delegate
// to the underlying pgx.Tx without preflight (those statements are
// transaction-control, not DML, and pg_query rejects them via
// MustContainOrgFilter — they must never reach the checker).
//
// OrgTx satisfies generated.DBTX exactly so Wave 3's agent_skills replace
// can do: `tx, _ := orgDB.BeginTx(ctx); qtx := generated.New(tx); qtx.
// DeleteAgentSkills(...); qtx.InsertAgentSkill(...); tx.Commit(ctx)`.
type OrgTx struct {
	tx      pgx.Tx
	checker *SQLChecker
	mode    ValidationMode
}

// Exec on a transaction. Preflight then delegate to pgx.Tx.Exec.
func (t *OrgTx) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if err := preflightSQL(ctx, sql, t.checker); err != nil {
		return pgconn.CommandTag{}, handlePreflightError(t.mode, "Tx.Exec", err)
	}
	return t.tx.Exec(ctx, sql, args...)
}

// Query on a transaction. Preflight then delegate to pgx.Tx.Query.
func (t *OrgTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if err := preflightSQL(ctx, sql, t.checker); err != nil {
		return nil, handlePreflightError(t.mode, "Tx.Query", err)
	}
	return t.tx.Query(ctx, sql, args...)
}

// QueryRow on a transaction. Preflight then delegate to pgx.Tx.QueryRow.
// On preflight failure: panics in ValidationPanic mode (dev/test), returns
// errRow in ValidationError mode (prod) so the error surfaces at Scan.
func (t *OrgTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if err := preflightSQL(ctx, sql, t.checker); err != nil {
		return errRow{err: handlePreflightError(t.mode, "Tx.QueryRow", err)}
	}
	return t.tx.QueryRow(ctx, sql, args...)
}

// Commit ends the transaction successfully. Delegates directly to
// pgx.Tx.Commit — preflight does not apply (BEGIN/COMMIT are TCL, not DML).
func (t *OrgTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

// Rollback aborts the transaction. Delegates directly to pgx.Tx.Rollback —
// preflight does not apply.
func (t *OrgTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

// BeginTx starts a transaction inheriting the parent OrgDB's checker and
// mode. The returned OrgTx satisfies generated.DBTX so handler code can
// pass it to generated.New(tx) for sqlc-typed transactional queries
// (OQ-5, H5 — used by the agent_skills full-replace in Plan 03-09).
// Default pgx.TxOptions (read-write, default isolation) — handlers
// needing read-only or stricter isolation should add an Options-accepting
// overload in v0.2.
func (o *OrgDB) BeginTx(ctx context.Context) (*OrgTx, error) {
	tx, err := o.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("orgdb: begin tx: %w", err)
	}
	return &OrgTx{tx: tx, checker: o.checker, mode: o.mode}, nil
}

// Compile-time guarantee: OrgDB satisfies the sqlc-generated DBTX
// interface (D-01, Shared Pattern S9). If sqlc regenerates with a
// different signature this fails to compile rather than silently
// permitting a divergent wrapper.
var _ generated.DBTX = (*OrgDB)(nil)

// Compile-time guarantee: OrgTx satisfies the sqlc-generated DBTX
// interface (OQ-5). Required so handlers can do
// `generated.New(orgDB.BeginTx(ctx))` for transactional queries while
// preserving the org_id validator.
var _ generated.DBTX = (*OrgTx)(nil)

// Compile-time guarantee: errRow satisfies pgx.Row. pgx v5 reserves the right
// to add methods to Row outside semver; this assertion catches any such
// addition at build time before a runtime failure.
var _ pgx.Row = errRow{}
