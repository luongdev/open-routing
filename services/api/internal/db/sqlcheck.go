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
//   - DML (SELECT / UPDATE / DELETE): every top-level _scaffold range alias
//     must have a matching top-level WHERE ColumnRef named "org_id";
//     DDL/utility rejected unless bypass is present. This catches WHERE
//     predicates while correctly rejecting org_id appearing only in the
//     projection list, ORDER BY, JOIN ON condition, or nested subquery WHERE.
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
		switch sv := stmt.Node.(type) {
		case *pg_query.Node_SelectStmt:
			var whereClause *pg_query.Node
			var tenantAliases []string
			whereClause = sv.SelectStmt.WhereClause
			tenantAliases = tenantAliasesFromNodes(sv.SelectStmt.FromClause)
			if !whereSatisfiesTenantAliases(whereClause, tenantAliases) {
				return ErrSQLMissingOrgFilter
			}
		case *pg_query.Node_UpdateStmt:
			whereClause := sv.UpdateStmt.WhereClause
			tenantAliases := tenantAliasesFromRangeVar(sv.UpdateStmt.Relation)
			tenantAliases = appendTenantAliasesFromNodes(tenantAliases, sv.UpdateStmt.FromClause)
			if !whereSatisfiesTenantAliases(whereClause, tenantAliases) {
				return ErrSQLMissingOrgFilter
			}
		case *pg_query.Node_DeleteStmt:
			whereClause := sv.DeleteStmt.WhereClause
			tenantAliases := tenantAliasesFromRangeVar(sv.DeleteStmt.Relation)
			tenantAliases = appendTenantAliasesFromNodes(tenantAliases, sv.DeleteStmt.UsingClause)
			if !whereSatisfiesTenantAliases(whereClause, tenantAliases) {
				return ErrSQLMissingOrgFilter
			}
		case *pg_query.Node_InsertStmt:
			ins := sv.InsertStmt
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

// orgIDColumnQualifier returns the table qualifier for a ColumnRef whose final
// field is "org_id". For "org_id" it returns ("", true); for "a.org_id" it
// returns ("a", true). Multi-part names use the field immediately before the
// column as the relation/alias qualifier.
func orgIDColumnQualifier(cr *pg_query.Node_ColumnRef) (string, bool) {
	fields := cr.ColumnRef.GetFields()
	if len(fields) == 0 {
		return "", false
	}
	last := fields[len(fields)-1]
	s, ok := last.Node.(*pg_query.Node_String_)
	if !ok || s.String_.GetSval() != "org_id" {
		return "", false
	}
	if len(fields) == 1 {
		return "", true
	}
	qualifier, ok := fields[len(fields)-2].Node.(*pg_query.Node_String_)
	if !ok {
		return "", true
	}
	return qualifier.String_.GetSval(), true
}

// whereSatisfiesTenantAliases returns true when every top-level _scaffold
// range alias has a matching org_id ColumnRef in the same statement's WHERE.
// A single _scaffold range may use an unqualified org_id; multi-range queries
// must qualify each alias so one scoped side of a join cannot satisfy another.
func whereSatisfiesTenantAliases(where *pg_query.Node, tenantAliases []string) bool {
	if len(tenantAliases) == 0 {
		return false
	}
	qualified := make(map[string]struct{})
	var hasUnqualified bool
	collectTopLevelOrgIDRefs(where, qualified, &hasUnqualified)

	if len(tenantAliases) == 1 && hasUnqualified {
		return true
	}
	for _, alias := range tenantAliases {
		if _, ok := qualified[alias]; !ok {
			return false
		}
	}
	return true
}

// collectTopLevelOrgIDRefs recursively walks a WHERE-clause node tree and
// records ColumnRefs named "org_id" without crossing into SubLink subqueries.
// Handles:
//   - ColumnRef  — leaf: check the field name
//   - BoolExpr   — AND/OR/NOT: recurse into Args
//   - A_Expr     — comparison (=, >, IN, etc.): recurse into Lexpr and Rexpr
//
// All other node types are ignored (no org_id ColumnRef can hide inside them
// in any well-formed SQL the application would emit).
func collectTopLevelOrgIDRefs(node *pg_query.Node, qualified map[string]struct{}, hasUnqualified *bool) {
	if node == nil {
		return
	}
	switch n := node.Node.(type) {
	case *pg_query.Node_ColumnRef:
		qualifier, ok := orgIDColumnQualifier(n)
		if !ok {
			return
		}
		if qualifier == "" {
			*hasUnqualified = true
			return
		}
		qualified[qualifier] = struct{}{}
	case *pg_query.Node_BoolExpr:
		for _, arg := range n.BoolExpr.GetArgs() {
			collectTopLevelOrgIDRefs(arg, qualified, hasUnqualified)
		}
	case *pg_query.Node_AExpr:
		collectTopLevelOrgIDRefs(n.AExpr.Lexpr, qualified, hasUnqualified)
		collectTopLevelOrgIDRefs(n.AExpr.Rexpr, qualified, hasUnqualified)
	}
}

func tenantAliasesFromNodes(nodes []*pg_query.Node) []string {
	return appendTenantAliasesFromNodes(nil, nodes)
}

func appendTenantAliasesFromNodes(aliases []string, nodes []*pg_query.Node) []string {
	seen := make(map[string]struct{}, len(aliases)+len(nodes))
	for _, alias := range aliases {
		seen[alias] = struct{}{}
	}
	for _, node := range nodes {
		aliases = appendTenantAliasesFromNode(aliases, seen, node)
	}
	return aliases
}

func appendTenantAliasesFromNode(aliases []string, seen map[string]struct{}, node *pg_query.Node) []string {
	if node == nil {
		return aliases
	}
	switch n := node.Node.(type) {
	case *pg_query.Node_RangeVar:
		return appendTenantAliasFromRangeVar(aliases, seen, n.RangeVar)
	case *pg_query.Node_JoinExpr:
		aliases = appendTenantAliasesFromNode(aliases, seen, n.JoinExpr.Larg)
		return appendTenantAliasesFromNode(aliases, seen, n.JoinExpr.Rarg)
	}
	return aliases
}

func tenantAliasesFromRangeVar(rv *pg_query.RangeVar) []string {
	return appendTenantAliasFromRangeVar(nil, make(map[string]struct{}, 1), rv)
}

func appendTenantAliasFromRangeVar(aliases []string, seen map[string]struct{}, rv *pg_query.RangeVar) []string {
	if rv == nil || rv.GetRelname() != "_scaffold" {
		return aliases
	}
	alias := rv.GetRelname()
	if aliasName := rv.GetAlias().GetAliasname(); aliasName != "" {
		alias = aliasName
	}
	if _, ok := seen[alias]; ok {
		return aliases
	}
	seen[alias] = struct{}{}
	return append(aliases, alias)
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
