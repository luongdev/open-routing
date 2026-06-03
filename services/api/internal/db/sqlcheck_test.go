package db

import (
	"testing"
)

// TestSQLChecker_MustContainOrgFilter is a table-driven proof that the
// hardened validator accepts only DML that binds every top-level _scaffold
// range alias to org_id in the WHERE clause (or INSERT column list) while
// rejecting DML with org_id in other positions (projection, ORDER BY, JOIN ON,
// subquery WHERE, string literal) and rejecting DDL outright. DDL must always
// flow through WithBypass (D-11); DDL reaching the inspector means bypass is
// absent and the check fails safe.
func TestSQLChecker_MustContainOrgFilter(t *testing.T) {
	t.Parallel()
	c := NewSQLChecker()

	accept := []struct {
		name string
		sql  string
	}{
		{"InsertScaffold", `INSERT INTO _scaffold (id, org_id, external_id, name) VALUES ($1, $2, $3, $4) RETURNING id, org_id, external_id, name, created_at`},
		{"GetScaffoldByID", `SELECT id, org_id, external_id, name, created_at FROM _scaffold WHERE id = $1 AND org_id = $2`},
		{"ListScaffolds", `SELECT id, org_id, external_id, name, created_at FROM _scaffold WHERE org_id = $1 ORDER BY created_at DESC LIMIT 100`},
		{"UpdateScaffold", `UPDATE _scaffold SET name = $2 WHERE id = $1 AND org_id = $3`},
		{"DeleteScaffold", `DELETE FROM _scaffold WHERE id = $1 AND org_id = $2`},
		{"AliasedScaffold", `SELECT a.id FROM _scaffold a WHERE a.org_id = $1 AND a.id = $2`},
		{"JoinWithAllAliasesScoped", `SELECT a.id FROM _scaffold a JOIN _scaffold b ON a.id = b.id WHERE a.org_id = $1 AND b.org_id = $1`},
		{"OuterFilterWithSubquery", `SELECT id, org_id FROM _scaffold WHERE org_id = $1 AND EXISTS (SELECT 1 FROM _scaffold s2 WHERE s2.org_id = $1)`},
		// Cross-AI review 2026-06-02: recursive validation accepts scoped nested queries.
		{"InsertSelectScopedSource", `INSERT INTO _scaffold (id, org_id, name) SELECT $1, org_id, name FROM _scaffold WHERE org_id = $2 AND id = $3`},
		{"CteScoped", `WITH x AS (SELECT id FROM _scaffold WHERE org_id = $1) SELECT id, org_id FROM _scaffold WHERE org_id = $1`},
	}
	for _, tc := range accept {
		tc := tc
		t.Run("accept_"+tc.name, func(t *testing.T) {
			t.Parallel()
			if err := c.MustContainOrgFilter(tc.sql); err != nil {
				t.Fatalf("expected nil, got %v for %q", err, tc.sql)
			}
		})
	}

	reject := []struct {
		name string
		sql  string
	}{
		{"BareSelectWithoutOrgID", `SELECT id FROM _scaffold`},
		{"SelectAll", `SELECT * FROM _scaffold`},
		{"InsertWithoutOrgIDColumn", `INSERT INTO _scaffold (id, external_id, name) VALUES ($1, $2, $3)`},
		{"DeleteWithoutWhere", `DELETE FROM _scaffold`},
		{"UpdateNoOrgFilter", `UPDATE _scaffold SET name = $1 WHERE id = $2`},
		// org_id appears only in JOIN ON condition, not in WHERE predicate.
		{"JoinWithoutWhereOrgID", `SELECT a.id FROM _scaffold a JOIN _scaffold b ON a.org_id = b.org_id WHERE a.id = $1`},
		// org_id scopes only one top-level _scaffold alias; b remains unscoped.
		{"JoinWithPartialWhereOrgID", `SELECT a.id, b.id FROM _scaffold a JOIN _scaffold b ON a.org_id = b.org_id WHERE a.org_id = $1 AND a.id = $2`},
		// org_id appears only in the SubLink WHERE, not the outer query WHERE.
		{"SubqueryOnlyOrgID", `SELECT id, org_id FROM _scaffold WHERE EXISTS (SELECT 1 FROM _scaffold s2 WHERE s2.org_id = $1)`},
		// org_id in SELECT projection list, not WHERE predicate — actual cross-org leak vector.
		{"ProjectionOnlyOrgID", `SELECT id, org_id FROM _scaffold`},
		// org_id in ORDER BY clause only, not WHERE predicate.
		{"OrderByOrgID", `SELECT id FROM _scaffold ORDER BY org_id`},
		// org_id as a string literal (A_Const.sval), not a ColumnRef — must not pass.
		{"LiteralOrgIDString", `SELECT 'org_id' AS alias FROM _scaffold`},
		// DDL without bypass must be rejected — bypass short-circuits before inspector.
		{"DropTable", `DROP TABLE _scaffold`},
		{"TruncateTable", `TRUNCATE _scaffold`},
		// Cross-AI review 2026-06-02: nested-query isolation gaps now fail closed.
		{"InsertSelectUnscopedSource", `INSERT INTO _scaffold (id, org_id, name) SELECT $1, org_id, name FROM _scaffold`},
		{"SubqueryInWhereUnscoped", `SELECT id FROM _scaffold WHERE org_id = $1 AND id IN (SELECT id FROM _scaffold s2)`},
		{"CteUnscoped", `WITH x AS (SELECT id FROM _scaffold) SELECT id, org_id FROM _scaffold WHERE org_id = $1`},
	}
	for _, tc := range reject {
		tc := tc
		t.Run("reject_"+tc.name, func(t *testing.T) {
			t.Parallel()
			if err := c.MustContainOrgFilter(tc.sql); err == nil {
				t.Fatalf("expected ErrSQLMissingOrgFilter, got nil for %q", tc.sql)
			}
		})
	}
}

// TestSQLChecker_MustContainOrgFilter_CatalogTables exercises the catalog
// v0.1 schema (Phase 3 Wave 1, D-62) against the validator. Each entity
// in the tenantTables map (agents, skills, queues, channels, adapters,
// break_reasons, agent_skills) MUST be accepted when the query carries
// org_id in WHERE and rejected when it does not. This test guards the
// allowlist policy in tenantTables — adding a new tenant table to the map
// without extending this test would silently weaken coverage.
func TestSQLChecker_MustContainOrgFilter_CatalogTables(t *testing.T) {
	t.Parallel()
	c := NewSQLChecker()

	accept := []struct {
		name string
		sql  string
	}{
		// Each catalog entity: SELECT happy path.
		{"SelectAgents", `SELECT id, name FROM agents WHERE org_id = $1 AND id = $2`},
		{"SelectSkills", `SELECT id, name FROM skills WHERE org_id = $1 AND id = $2`},
		{"SelectQueues", `SELECT id, name FROM queues WHERE org_id = $1 AND id = $2`},
		{"SelectChannels", `SELECT id, name FROM channels WHERE org_id = $1 AND id = $2`},
		{"SelectAdapters", `SELECT id, name FROM adapters WHERE org_id = $1 AND id = $2`},
		{"SelectBreakReasons", `SELECT id, name FROM break_reasons WHERE org_id = $1 AND id = $2`},
		{"SelectAgentSkills", `SELECT agent_id, skill_id FROM agent_skills WHERE org_id = $1 AND agent_id = $2`},
		// UPDATE + DELETE happy paths for one entity (covers UPDATE/DELETE classify branches).
		{"UpdateAgentVersioned", `UPDATE agents SET name = $2, version = version + 1 WHERE id = $1 AND org_id = $3 AND version = $4`},
		{"DeleteAgentSkills", `DELETE FROM agent_skills WHERE agent_id = $1 AND org_id = $2`},
		// INSERT happy paths.
		{"InsertAgent", `INSERT INTO agents (id, org_id, external_id, name, email, enabled) VALUES ($1, $2, $3, $4, $5, $6)`},
		{"InsertAdapter", `INSERT INTO adapters (id, org_id, name, adapter_type, config, enabled) VALUES ($1, $2, $3, $4, $5, $6)`},
		// ILIKE pagination shape (D-63/D-64) — name filter via cursor and lower(name).
		{"ListAgentsCursor", `SELECT id FROM agents WHERE org_id = $1 AND enabled = TRUE AND (created_at, id) < ($2, $3) ORDER BY created_at DESC LIMIT $4`},
		// Joined query: ListSkillsForAgent — both tenant tables scoped via JOIN ON, WHERE binds aliased org_id for both.
		{"ListSkillsForAgentJoin", `SELECT s.id FROM agent_skills ag_s JOIN skills s ON s.id = ag_s.skill_id AND s.org_id = ag_s.org_id WHERE ag_s.agent_id = $1 AND ag_s.org_id = $2 AND s.org_id = $2`},
		// FK probe (D-76): QueueExistsAndEnabledInOrg returns one row if a
		// queue with the given id is enabled in the org, otherwise zero rows.
		// Handler maps pgx.ErrNoRows to 422 invalid_reference.
		{"QueueExistsProbe", `SELECT 1 AS exists_in_org FROM queues WHERE id = $1 AND org_id = $2 AND enabled = TRUE LIMIT 1`},
		// SkillsPresentInOrg pattern — array membership via ANY().
		{"SkillsPresentInOrg", `SELECT id FROM skills WHERE id = ANY($1::uuid[]) AND org_id = $2 AND enabled = TRUE`},
	}
	for _, tc := range accept {
		tc := tc
		t.Run("accept_"+tc.name, func(t *testing.T) {
			t.Parallel()
			if err := c.MustContainOrgFilter(tc.sql); err != nil {
				t.Fatalf("expected nil, got %v for %q", err, tc.sql)
			}
		})
	}

	reject := []struct {
		name string
		sql  string
	}{
		{"SelectAgentsNoOrg", `SELECT id, name FROM agents WHERE id = $1`},
		{"UpdateChannelsNoOrg", `UPDATE channels SET name = $1 WHERE id = $2`},
		{"DeleteAgentSkillsNoOrg", `DELETE FROM agent_skills WHERE agent_id = $1`},
		{"InsertAgentsNoOrgColumn", `INSERT INTO agents (id, external_id, name, email, enabled) VALUES ($1, $2, $3, $4, $5)`},
		// Skill_id IN subquery where outer FROM is non-tenant unnest(...) — rejected
		// (validator requires tenant table at top-level FROM).
		{"UnnestExceptPattern", `SELECT input_id FROM unnest($1::uuid[]) AS t(input_id) WHERE input_id NOT IN (SELECT id FROM skills WHERE org_id = $2)`},
	}
	for _, tc := range reject {
		tc := tc
		t.Run("reject_"+tc.name, func(t *testing.T) {
			t.Parallel()
			if err := c.MustContainOrgFilter(tc.sql); err == nil {
				t.Fatalf("expected ErrSQLMissingOrgFilter, got nil for %q", tc.sql)
			}
		})
	}
}

// TestSQLChecker_CacheMemoizes proves the SHA-256 cache is consulted on
// repeated calls (D-02). The second call returns the same verdict as the
// first; inspecting the internal map confirms a single cache entry per
// unique SQL string.
func TestSQLChecker_CacheMemoizes(t *testing.T) {
	t.Parallel()
	c := NewSQLChecker()
	const sql = `SELECT id FROM _scaffold WHERE org_id = $1`

	if err := c.MustContainOrgFilter(sql); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := c.MustContainOrgFilter(sql); err != nil {
		t.Fatalf("second call: %v", err)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.cache[hashSQL(sql)]; !ok {
		t.Fatal("expected cache entry for sql after MustContainOrgFilter")
	}
}
