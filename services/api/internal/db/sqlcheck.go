package db

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
//   - DML (SELECT / UPDATE / DELETE): walk the WHERE clause for a ColumnRef
//     named "org_id"; DDL/utility rejected unless bypass is present.
//     This catches WHERE predicates and nested subquery WHERE refs while
//     correctly rejecting org_id appearing only in the projection list,
//     ORDER BY, or JOIN ON condition.
//   - INSERT: the column list must contain "org_id" (target_list shape).
//     A regex on "WHERE org_id" would miss every INSERT — INSERT has no
//     WHERE — which is precisely why this dispatch is split.
//   - DDL (CreateStmt, AlterTableStmt, DropStmt, IndexStmt, etc.) and
//     any other utility statement: rejected. Migrations always flow
//     through WithBypass (D-11) so DDL reaching the checker means bypass
//     is absent — fail safe.
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
		if stmt == nil {
			return ErrSQLMissingOrgFilter
		}
		switch stmt.Node.(type) {
		case *pg_query.Node_SelectStmt,
			*pg_query.Node_UpdateStmt,
			*pg_query.Node_DeleteStmt:
			var whereClause *pg_query.Node
			switch sv := stmt.Node.(type) {
			case *pg_query.Node_SelectStmt:
				whereClause = sv.SelectStmt.WhereClause
			case *pg_query.Node_UpdateStmt:
				whereClause = sv.UpdateStmt.WhereClause
			case *pg_query.Node_DeleteStmt:
				whereClause = sv.DeleteStmt.WhereClause
			}
			if !whereContainsOrgIDRef(whereClause) {
				return ErrSQLMissingOrgFilter
			}
		case *pg_query.Node_InsertStmt:
			ins := stmt.Node.(*pg_query.Node_InsertStmt).InsertStmt
			if !insertHasOrgIDColumn(ins) {
				return ErrSQLMissingOrgFilter
			}
		default:
			// DDL and utility statements must go through WithBypass.
			// If they reach the checker, bypass is absent — reject.
			return ErrSQLMissingOrgFilter
		}
	}
	return nil
}

// isOrgIDColumnRef returns true when the ColumnRef's last field is "org_id".
// ColumnRef.Fields is a []*Node where each node is typically Node_String_
// (for simple names) or Node_A_Star (for *). Only the last field is the
// column name; earlier fields are table qualifiers (e.g. "a" in a.org_id).
func isOrgIDColumnRef(cr *pg_query.Node_ColumnRef) bool {
	fields := cr.ColumnRef.GetFields()
	if len(fields) == 0 {
		return false
	}
	last := fields[len(fields)-1]
	s, ok := last.Node.(*pg_query.Node_String_)
	return ok && s.String_.GetSval() == "org_id"
}

// whereContainsOrgIDRef recursively walks a WHERE-clause node tree and returns
// true when any ColumnRef named "org_id" is present. Handles:
//   - ColumnRef  — leaf: check the field name
//   - BoolExpr   — AND/OR/NOT: recurse into Args
//   - A_Expr     — comparison (=, >, IN, etc.): recurse into Lexpr and Rexpr
//   - SubLink    — correlated subquery: recurse into the subselect's WHERE
//
// All other node types are ignored (no org_id ColumnRef can hide inside them
// in any well-formed SQL the application would emit).
func whereContainsOrgIDRef(node *pg_query.Node) bool {
	if node == nil {
		return false
	}
	switch n := node.Node.(type) {
	case *pg_query.Node_ColumnRef:
		return isOrgIDColumnRef(n)
	case *pg_query.Node_BoolExpr:
		for _, arg := range n.BoolExpr.GetArgs() {
			if whereContainsOrgIDRef(arg) {
				return true
			}
		}
	case *pg_query.Node_AExpr:
		return whereContainsOrgIDRef(n.AExpr.Lexpr) ||
			whereContainsOrgIDRef(n.AExpr.Rexpr)
	case *pg_query.Node_SubLink:
		if sub := n.SubLink.GetSubselect(); sub != nil {
			if sel, ok := sub.Node.(*pg_query.Node_SelectStmt); ok {
				return whereContainsOrgIDRef(sel.SelectStmt.WhereClause)
			}
		}
	}
	return false
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
