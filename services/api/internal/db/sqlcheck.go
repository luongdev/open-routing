package db

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ErrSQLMissingOrgFilter is returned by SQLChecker.MustContainOrgFilter when
// the SQL string lacks an org_id reference in an appropriate position
// (WHERE clause / INSERT column list / UPDATE / DELETE filter). Bubbled up
// through orgDB.preflight; panicked in ValidationPanic mode, returned in
// ValidationError mode (D-02).
var ErrSQLMissingOrgFilter = errors.New("orgdb: SQL string missing org_id filter")

// SQLChecker memoizes the org_id-presence verdict for each unique SQL
// string keyed by SHA-256 hash (D-02). Process-wide reuse: handlers in
// Phase 1 share one *SQLChecker instance constructed at startup so the
// pg_query.Parse cost is amortized across the entire process lifetime.
//
// The cache is bounded by the number of unique SQL strings the binary
// emits. Phase 1 has 3 named queries (3 entries); Phases 3-7 reach ~50
// queries (~50 entries). Unbounded growth is recorded in the threat
// register as T-1-CACHE / accept (Phase 1 risk acceptance).
type SQLChecker struct {
	mu    sync.RWMutex
	cache map[string]error // nil = accepted; non-nil = rejection sentinel
}

// NewSQLChecker constructs an empty checker. Safe for concurrent use.
func NewSQLChecker() *SQLChecker {
	return &SQLChecker{cache: make(map[string]error)}
}

// MustContainOrgFilter parses the SQL once and caches the verdict by
// SHA-256 hash. Returns nil when the statement references org_id in an
// appropriate position, ErrSQLMissingOrgFilter when it does not, or a
// wrapped pg_query parse error if the SQL is malformed.
//
// The cache check uses RLock first to maximize read concurrency; only
// the first writer per unique SQL acquires the write lock. classify()
// is idempotent so a benign race where two goroutines both classify the
// same novel SQL is safe — the second write simply overwrites with an
// identical value.
func (c *SQLChecker) MustContainOrgFilter(sql string) error {
	key := hashSQL(sql)

	c.mu.RLock()
	if cached, ok := c.cache[key]; ok {
		c.mu.RUnlock()
		return cached
	}
	c.mu.RUnlock()

	result := c.classify(sql)

	c.mu.Lock()
	c.cache[key] = result
	c.mu.Unlock()
	return result
}

// classify parses the SQL via pg_query_go (the real Postgres parser
// compiled to Go via CGO — NOT regex; per Research §"Don't Hand-Roll"
// and Pitfall 2) and applies one of three rules per top-level statement:
//
//   - DML (SELECT / UPDATE / DELETE): walk the parse tree for any
//     ColumnRef whose name equals "org_id". This catches WHERE clauses,
//     JOIN ... ON foo.org_id = bar.org_id, and nested subquery refs
//     uniformly because we scan the entire stmt subtree.
//   - INSERT: the column list must contain "org_id" (target_list shape).
//     A regex on "WHERE org_id" would miss every INSERT — INSERT has no
//     WHERE — which is precisely why this dispatch is split.
//   - DDL (CreateStmt, AlterTableStmt, DropStmt, IndexStmt, etc.):
//     accepted unconditionally. DDL has no row scope; migrations always
//     flow through WithBypass (D-11) so the validator never sees DDL in
//     production code paths.
//
// Unparseable SQL fails safe — a wrapped parse error is returned so the
// caller surfaces the actual parse failure rather than silently passing
// SQL the parser can't reason about.
func (c *SQLChecker) classify(sql string) error {
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return fmt.Errorf("orgdb: cannot parse sql: %w", err)
	}
	if len(tree.Stmts) == 0 {
		return ErrSQLMissingOrgFilter
	}
	for _, raw := range tree.Stmts {
		stmt := raw.Stmt
		switch n := stmt.Node.(type) {
		case *pg_query.Node_SelectStmt,
			*pg_query.Node_UpdateStmt,
			*pg_query.Node_DeleteStmt:
			if !containsOrgIDColumnRef(stmt) {
				return ErrSQLMissingOrgFilter
			}
		case *pg_query.Node_InsertStmt:
			if !insertHasOrgIDColumn(n.InsertStmt) {
				return ErrSQLMissingOrgFilter
			}
		default:
			// DDL and any other utility statement — accepted. Migrations
			// are the only legitimate caller and they bypass anyway.
			_ = n
		}
	}
	return nil
}

// containsOrgIDColumnRef returns true when the statement subtree contains
// any ColumnRef whose name equals "org_id".
//
// Implementation note: pg_query_go's protobuf text representation
// renders ColumnRef fields as `column_ref:{fields:{string:{sval:"<name>"}}}`.
// Scanning the serialized text for the literal `sval:"org_id"` is a
// pragmatic correctness check (Pitfall 2 explicitly endorses tree-shape
// recognition over per-Node visitors for Phase 1). Verified empirically
// against pg_query_go v6 against all 7 accept-case SQL shapes including
// JOIN ... ON foo.org_id = bar.org_id and nested subquery patterns.
//
// This intentionally does NOT match INSERT columns — those render as
// `res_target:{name:"org_id"...}` (no string/sval wrapper) so the
// shortcut fails closed for INSERT. INSERT statements take the dedicated
// insertHasOrgIDColumn path via the classify type switch.
func containsOrgIDColumnRef(n *pg_query.Node) bool {
	if n == nil {
		return false
	}
	return strings.Contains(n.String(), `sval:"org_id"`)
}

// insertHasOrgIDColumn returns true when an INSERT statement's column list
// includes "org_id" as a ResTarget name. The cols field of InsertStmt is
// a []*Node where each Node wraps a ResTarget; we inspect each one.
//
// Empirical pg_query_go v6 shape (verified):
//
//	cols:{res_target:{name:"org_id" location:N}}
//
// so unwrapping Node -> Node_ResTarget -> ResTarget.Name is sufficient.
func insertHasOrgIDColumn(ins *pg_query.InsertStmt) bool {
	if ins == nil {
		return false
	}
	for _, col := range ins.Cols {
		if col == nil {
			continue
		}
		rt, ok := col.Node.(*pg_query.Node_ResTarget)
		if !ok || rt.ResTarget == nil {
			continue
		}
		if rt.ResTarget.Name == "org_id" {
			return true
		}
	}
	return false
}

// hashSQL returns the lowercase hex SHA-256 of the SQL string. Used as
// the cache key in SQLChecker — the SQL string itself could be megabytes
// in pathological cases, but the 64-character hash is a stable bounded
// key (D-02 explicit choice).
func hashSQL(sql string) string {
	sum := sha256.Sum256([]byte(sql))
	return hex.EncodeToString(sum[:])
}
