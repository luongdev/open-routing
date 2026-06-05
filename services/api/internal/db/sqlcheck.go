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

// tenantTables enumerates every table whose rows carry org_id and therefore
// MUST be filtered by org_id in DML (D-02, H3). Phase 1 began with the
// single _scaffold table; Phase 3 Wave 1 introduces the catalog v0.1 schema
// (agents, skills, queues, channels, adapters, break_reasons + the
// agent_skills join — D-61, D-62, D-72). Adding a row here is the canonical
// way to extend the validator's coverage to new org-scoped tables.
//
// Tables NOT in this set are treated as non-tenant (lookup tables, reference
// data, etc.). A future hardening pass may invert the policy so that any
// unrecognized table fails closed (Pitfall: a typo in a future tenant table
// name would silently bypass the check). For v0.1 we accept the explicit
// allowlist semantics.
var tenantTables = map[string]struct{}{
	"_scaffold":            {}, // legacy Phase 1; removed once scaffold queries drop out
	"agents":               {}, // CAT-01
	"skills":               {}, // CAT-02
	"queues":               {}, // CAT-04
	"channels":             {}, // CAT-05
	"adapters":             {}, // CAT-06
	"break_reasons":        {}, // CAT-07
	"agent_skills":         {}, // CAT-03 (junction; carries denormalized org_id per D-72)
	"agent_states":         {}, // STATE-01 (Phase 4; denormalized org_id per D-78 — sweeper bypasses ctx-org and relies on this column)
	"import_jobs":          {}, // IMP-06 (Phase 5; denormalized org_id per RESEARCH §Pattern 8 — sweeper bypasses ctx-org and relies on this column)
	"flows":                {}, // FLOW (v0.2 — flow drafts)
	"flow_versions":        {}, // FLOW (v0.2 — immutable published versions)
	"flow_entry_bindings":  {}, // FLOW (v0.2 — route entry → active published version)
	"route_requests":       {}, // RT (v0.2 — interaction spine)
	"reservations":         {}, // RT (v0.2 — reservation lifecycle)
	"continuations":        {}, // RT (v0.2 — durable delayed work, SKIP LOCKED)
	"runtime_events":       {}, // RT (v0.2 — canonical event envelope, outbox-first)
	"traces":               {}, // RT (v0.2 — trace read records)
	"agent_outbox":         {}, // WS (v0.3 — durable outbound, per-agent server_seq)
	"ws_command_dedupe":    {}, // WS (v0.3 — at-most-once inbound commands)
	"agent_sessions":       {}, // WS (v0.3 — session inventory / revocation)
	"agent_capacity_slots": {}, // CAP (v0.3 W3 — per-(agent,channel) capacity slots)
	"route_decisions":      {}, // MATCH (v0.3 W4 — matcher decision audit)
	"agent_routing_state":  {}, // MATCH (v0.3 W4 — RONA routing state)
	"delivery_commands":    {}, // DELIVERY (v0.4 W1 — durable delivery outbox)
}

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
//   - DML (SELECT / UPDATE / DELETE): every top-level tenant-table range
//     alias (see tenantTables) must have a matching top-level WHERE
//     ColumnRef named "org_id"; DDL/utility rejected unless bypass is
//     present. This catches WHERE predicates while correctly rejecting
//     org_id appearing only in the projection list, ORDER BY, JOIN ON
//     condition, or nested subquery WHERE.
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
		if raw.Stmt == nil {
			return ErrSQLMissingOrgFilter
		}
		// Only the four DML statements may reach orgDB; DDL/utility must go
		// through WithBypass (D-11), so a non-DML top-level statement fails closed.
		switch raw.Stmt.Node.(type) {
		case *pg_query.Node_SelectStmt, *pg_query.Node_InsertStmt,
			*pg_query.Node_UpdateStmt, *pg_query.Node_DeleteStmt:
			if err := validateNode(raw.Stmt, true); err != nil {
				return err
			}
		default:
			return ErrSQLMissingOrgFilter
		}
	}
	return nil
}

// validateNode validates `node` if it is a query statement — every tenant-table
// reference must carry a matching org_id filter at the level that introduces it —
// then recurses into every child that can nest another query: CTEs, FROM/WHERE
// subqueries, set-operation arms, and INSERT...SELECT sources. This closes the
// subquery/CTE isolation gap: a nested SELECT from a tenant table without an
// org_id filter is rejected even when the outer query is scoped (cross-AI review
// 2026-06-02, Codex/agy CRITICAL).
//
// requireTenant is true only for a top-level statement: it must touch a tenant
// table, so a bare `SELECT 1` issued through orgDB fails closed. Nested queries
// may legitimately touch no tenant table at their own level; any tenant tables
// they DO reference are still checked.
func validateNode(node *pg_query.Node, requireTenant bool) error {
	if node == nil {
		return nil
	}
	switch n := node.Node.(type) {
	case *pg_query.Node_SelectStmt:
		return validateSelectStmt(n.SelectStmt, requireTenant)
	case *pg_query.Node_InsertStmt:
		ins := n.InsertStmt
		if !insertHasOrgIDColumn(ins) {
			return ErrSQLMissingOrgFilter
		}
		// INSERT ... SELECT: the source query must be scoped too (a VALUES list
		// has no FROM, so validateNode is a no-op for it).
		if err := validateNode(ins.SelectStmt, false); err != nil {
			return err
		}
		return validateChildren(withCtes(ins.WithClause))
	case *pg_query.Node_UpdateStmt:
		u := n.UpdateStmt
		aliases := tenantAliasesFromRangeVar(u.Relation)
		aliases = appendTenantAliasesFromNodes(aliases, u.FromClause)
		if !whereSatisfiesTenantAliases(u.WhereClause, aliases) {
			return ErrSQLMissingOrgFilter
		}
		return validateChildren(u.FromClause, u.TargetList, u.ReturningList,
			[]*pg_query.Node{u.WhereClause}, withCtes(u.WithClause))
	case *pg_query.Node_DeleteStmt:
		d := n.DeleteStmt
		aliases := tenantAliasesFromRangeVar(d.Relation)
		aliases = appendTenantAliasesFromNodes(aliases, d.UsingClause)
		if !whereSatisfiesTenantAliases(d.WhereClause, aliases) {
			return ErrSQLMissingOrgFilter
		}
		return validateChildren(d.UsingClause, d.ReturningList,
			[]*pg_query.Node{d.WhereClause}, withCtes(d.WithClause))
	case *pg_query.Node_List:
		return validateChildren(n.List.Items)
	case *pg_query.Node_JoinExpr:
		return validateChildren([]*pg_query.Node{n.JoinExpr.Larg, n.JoinExpr.Rarg, n.JoinExpr.Quals})
	case *pg_query.Node_RangeSubselect:
		return validateNode(n.RangeSubselect.Subquery, false)
	case *pg_query.Node_SubLink:
		return validateNode(n.SubLink.Subselect, false)
	case *pg_query.Node_BoolExpr:
		return validateChildren(n.BoolExpr.Args)
	case *pg_query.Node_AExpr:
		return validateChildren([]*pg_query.Node{n.AExpr.Lexpr, n.AExpr.Rexpr})
	case *pg_query.Node_ResTarget:
		return validateNode(n.ResTarget.Val, false)
	case *pg_query.Node_CommonTableExpr:
		return validateNode(n.CommonTableExpr.Ctequery, false)
	default:
		return nil
	}
}

// validateSelectStmt enforces tenant scoping for one SELECT level, then recurses
// into its sub-queries. Set-operation arms (UNION etc.) are themselves SELECTs.
func validateSelectStmt(s *pg_query.SelectStmt, requireTenant bool) error {
	if s == nil {
		return nil
	}
	aliases := tenantAliasesFromNodes(s.FromClause)
	if len(aliases) > 0 {
		if !whereSatisfiesTenantAliases(s.WhereClause, aliases) {
			return ErrSQLMissingOrgFilter
		}
	} else if requireTenant && len(s.ValuesLists) == 0 {
		return ErrSQLMissingOrgFilter
	}
	if err := validateSelectStmt(s.Larg, false); err != nil {
		return err
	}
	if err := validateSelectStmt(s.Rarg, false); err != nil {
		return err
	}
	return validateChildren(s.FromClause, s.TargetList, s.GroupClause,
		s.SortClause, s.ValuesLists, []*pg_query.Node{s.WhereClause},
		[]*pg_query.Node{s.HavingClause}, withCtes(s.WithClause))
}

// validateChildren recurses validateNode over every node in the given lists.
// Nested nodes never carry the top-level tenant requirement.
func validateChildren(lists ...[]*pg_query.Node) error {
	for _, list := range lists {
		for _, child := range list {
			if err := validateNode(child, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func withCtes(w *pg_query.WithClause) []*pg_query.Node {
	if w == nil {
		return nil
	}
	return w.GetCtes()
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

// whereSatisfiesTenantAliases returns true when every top-level tenant-table
// range alias has a matching org_id ColumnRef in the same statement's WHERE.
// A single tenant range may use an unqualified org_id; multi-range queries
// must qualify each alias so one scoped side of a join cannot satisfy another.
//
// Empty tenant alias list means the statement's top-level FROM does not
// touch any known tenant table — for example a SELECT from `unnest(...)`
// or `SELECT 1`. Such a query is rejected here as a defense-in-depth measure
// (the validator cannot reason about whether a non-tenant query is safe).
// Queries that need to probe an input array against a tenant table should
// put the tenant table in the outer FROM and use the array via `WHERE id =
// ANY($N::uuid[])` (Phase 3 SkillsPresentInOrg in agent_skills.sql is the
// canonical example).
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
	if rv == nil {
		return aliases
	}
	rel := rv.GetRelname()
	if _, ok := tenantTables[rel]; !ok {
		return aliases
	}
	alias := rel
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
