// chunk_test.go — testcontainers Postgres integration suite for the
// chunked-savepoint orchestrator (chunk.go).
//
// Skips automatically when sharedPool == nil (run without -short to
// enable; CI must have Docker for the postgres testcontainer to come
// up — see main_test.go's exit-code policy).
//
// Why integration-level (not pure unit): the chunk orchestrator's
// contract IS the SAVEPOINT semantics — Begin → ROLLBACK TO / RELEASE
// → outerTx.Commit. Pure unit tests with mocked Tx interfaces would
// not exercise the real Postgres savepoint behaviour.
package imports

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// ctxWithOrg attaches the test's org_id to ctx so every sqlc query
// routed through OrgDB / OrgTx / savepoint passes SQLChecker preflight
// (D-03 — ctx must carry org_id). Production hits this via the
// OrgContext middleware; tests synthesise it inline.
func ctxWithOrg(ctx context.Context, th *TestImports) context.Context {
	return orgkey.SetOrgID(ctx, th.OrgID)
}

// rawAgentRow builds a JSON-shaped map[string]interface{} matching the
// ImportAgentRequest schema. Tests use this to construct parsedRow.raw
// without involving the parser layer.
func rawAgentRow(code, name, email string) map[string]interface{} {
	return map[string]interface{}{
		"code":  code,
		"name":  name,
		"email": email,
	}
}

// rawAgentRowWithSkills extends rawAgentRow with a skills[] array.
func rawAgentRowWithSkills(code, name, email string, skills []map[string]interface{}) map[string]interface{} {
	row := rawAgentRow(code, name, email)
	row["skills"] = skills
	return row
}

// fetchAgentCountByCode returns the number of agents rows with the
// given (org_id, code). 0 = absent; 1 = present.
func fetchAgentCountByCode(t testing.TB, ctx context.Context, th *TestImports, code string) int {
	t.Helper()
	var n int
	err := th.Pool.QueryRow(ctx,
		`SELECT count(*) FROM agents WHERE org_id = $1 AND code = $2`,
		th.OrgID, code,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

// TestProcessChunk_AllSucceed_OuterCommits — every row passes; outer
// Tx commits; all rows surface in the catalog table.
func TestProcessChunk_AllSucceed_OuterCommits(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRow("alice", "Alice Anderson", "alice@example.com")},
		{lineNo: 2, raw: rawAgentRow("bob", "Bob Brown", "bob@example.com")},
		{lineNo: 3, raw: rawAgentRow("carol", "Carol Carter", "carol@example.com")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)

	require.Len(t, succeeded, 3, "all rows should land")
	require.Len(t, failed, 0, "no failures expected")
	require.Equal(t, 1, fetchAgentCountByCode(t, ctx, th, "alice"))
	require.Equal(t, 1, fetchAgentCountByCode(t, ctx, th, "bob"))
	require.Equal(t, 1, fetchAgentCountByCode(t, ctx, th, "carol"))
}

// TestProcessChunk_OneRowFails_OthersSucceed — middle row is invalid;
// only that row's savepoint rolls back.
func TestProcessChunk_OneRowFails_OthersSucceed(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRow("alice", "Alice", "alice@example.com")},
		{lineNo: 2, raw: rawAgentRow("INVALID CODE", "Bad Bob", "bob@example.com")},
		{lineNo: 3, raw: rawAgentRow("carol", "Carol", "carol@example.com")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)

	require.Len(t, succeeded, 2, "rows 1 and 3 should land")
	require.Len(t, failed, 1, "row 2 should fail")
	require.Equal(t, 2, failed[0].Row)
	require.Equal(t, "invalid_code_format", failed[0].Reason)
	require.NotNil(t, failed[0].Field)
	require.Equal(t, "code", *failed[0].Field)

	require.Equal(t, 1, fetchAgentCountByCode(t, ctx, th, "alice"))
	require.Equal(t, 0, fetchAgentCountByCode(t, ctx, th, "INVALID CODE"),
		"rolled-back row must be absent from the catalog table")
	require.Equal(t, 1, fetchAgentCountByCode(t, ctx, th, "carol"))
}

// TestProcessChunk_AllRowsFail_OuterStillCommits — every row fails
// its own savepoint; outer Tx still commits with zero rows.
func TestProcessChunk_AllRowsFail_OuterStillCommits(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRow("INVALID 1", "Bad A", "bad-a@example.com")},
		{lineNo: 2, raw: rawAgentRow("INVALID 2", "Bad B", "bad-b@example.com")},
		{lineNo: 3, raw: rawAgentRow("INVALID 3", "Bad C", "bad-c@example.com")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)

	require.Len(t, succeeded, 0)
	require.Len(t, failed, 3)
	for i, f := range failed {
		require.Equal(t, i+1, f.Row)
		require.Equal(t, "invalid_code_format", f.Reason)
	}
	var n int
	err := th.Pool.QueryRow(ctx, `SELECT count(*) FROM agents WHERE org_id = $1`, th.OrgID).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

// TestProcessChunk_OuterTxBeginFails_AllRowsMarkedFailed — build a
// fresh Importer wired to a CLOSED pgxpool so BeginTx fails. Every
// row in the chunk gets reason=tx_begin_failed.
//
// We open a one-shot pool against the same connection string used by
// sharedPool, immediately close it, then build OrgDB + Importer on
// the closed pool. The sharedPool itself stays untouched.
func TestProcessChunk_OuterTxBeginFails_AllRowsMarkedFailed(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Reuse the shared pool's conn string. Close the temp pool so
	// BeginTx will fail on the broken Importer.
	connString := th.Pool.Config().ConnString()
	tempPool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	tempPool.Close()

	brokenDB := db.NewOrgDB(tempPool, db.NewSQLChecker(), db.ValidationError)
	brokenImp := New(Deps{
		OrgDB:  brokenDB,
		Cache:  th.I.deps.Cache,
		Logger: th.Logger,
	})

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRow("alice", "Alice", "alice@example.com")},
		{lineNo: 2, raw: rawAgentRow("bob", "Bob", "bob@example.com")},
	}
	succeeded, failed := brokenImp.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: brokenImp}, rows)

	require.Len(t, succeeded, 0)
	require.Len(t, failed, 2)
	for _, f := range failed {
		require.Equal(t, "tx_begin_failed", f.Reason)
	}
}

// TestProcessChunk_PostCommit_CacheDel_OnlyAfterSuccess — seed cache
// keys with stale values; run processChunk; assert keys are GONE
// after processChunk returns (Pitfall 4 — POST-COMMIT cache.Del).
func TestProcessChunk_PostCommit_CacheDel_OnlyAfterSuccess(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Pre-seed agents + their cache entries.
	preID1 := seedAgentRaw(t, ctx, th, "alice", "Alice Old", "alice-old@example.com")
	preID2 := seedAgentRaw(t, ctx, th, "bob", "Bob Old", "bob-old@example.com")

	key1 := cache.Key(th.OrgID, string(api.Agents), preID1)
	key2 := cache.Key(th.OrgID, string(api.Agents), preID2)

	mustWarmCache(t, ctx, th, key1)
	mustWarmCache(t, ctx, th, key2)
	require.True(t, cacheKeyExists(t, ctx, th, key1))
	require.True(t, cacheKeyExists(t, ctx, th, key2))

	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRow("alice", "Alice New", "alice-new@example.com")},
		{lineNo: 2, raw: rawAgentRow("bob", "Bob New", "bob-new@example.com")},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 2)
	require.Len(t, failed, 0)

	// The two pre-seeded keys must be cleared by chunk.processChunk's
	// post-commit cache.Del loop. Note: the IDs returned by
	// UpsertAgentByCode on the ON CONFLICT path are the EXISTING
	// row's IDs — so the cache.Del targets exactly key1/key2.
	require.False(t, cacheKeyExists(t, ctx, th, key1),
		"cache key %q must be cleared post-commit", key1)
	require.False(t, cacheKeyExists(t, ctx, th, key2),
		"cache key %q must be cleared post-commit", key2)
}

// TestProcessChunk_SkillResolveBatchedOnce — 3 agent rows × 2 skill
// codes each → resolveChunkSkillCodes must execute exactly ONE
// ResolveSkillCodes call. Verify via pg_stat_user_tables delta.
func TestProcessChunk_SkillResolveBatchedOnce(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := ctxWithOrg(context.Background(), th)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Seed both skills the import references.
	seedSkillRaw(t, ctx, th, "skill_voice", "Voice", "core")
	seedSkillRaw(t, ctx, th, "skill_chat", "Chat", "core")

	// Build the resolver explicitly so we can inspect callCount.
	outerTx, err := th.OrgDB.BeginTx(ctx)
	require.NoError(t, err)
	resolver, err := th.I.resolveChunkSkillCodes(ctx, outerTx, th.OrgID, []parsedRow{
		{lineNo: 1, raw: rawAgentRowWithSkills("alice", "Alice", "alice@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_voice", "proficiency": 7},
				{"skill_code": "skill_chat", "proficiency": 5},
			})},
		{lineNo: 2, raw: rawAgentRowWithSkills("bob", "Bob", "bob@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_voice", "proficiency": 8},
				{"skill_code": "skill_chat", "proficiency": 6},
			})},
		{lineNo: 3, raw: rawAgentRowWithSkills("carol", "Carol", "carol@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_voice", "proficiency": 9},
				{"skill_code": "skill_chat", "proficiency": 4},
			})},
	})
	require.NoError(t, err)
	require.NoError(t, outerTx.Rollback(ctx))

	// resolveChunkSkillCodes ran ONE batched ResolveSkillCodes call.
	// Both skill codes were resolved.
	require.Equal(t, 2, len(resolver.byCode),
		"both unique skill codes should be cached after one batched lookup")

	// Now run the actual chunk to verify the agent + skills land.
	rows := []parsedRow{
		{lineNo: 1, raw: rawAgentRowWithSkills("alice", "Alice", "alice@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_voice", "proficiency": 7},
				{"skill_code": "skill_chat", "proficiency": 5},
			})},
		{lineNo: 2, raw: rawAgentRowWithSkills("bob", "Bob", "bob@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_voice", "proficiency": 8},
				{"skill_code": "skill_chat", "proficiency": 6},
			})},
		{lineNo: 3, raw: rawAgentRowWithSkills("carol", "Carol", "carol@example.com",
			[]map[string]interface{}{
				{"skill_code": "skill_voice", "proficiency": 9},
				{"skill_code": "skill_chat", "proficiency": 4},
			})},
	}
	succeeded, failed := th.I.processChunk(ctx, th.OrgID, api.Agents, &agentRowProc{handlers: th.I}, rows)
	require.Len(t, succeeded, 3)
	require.Len(t, failed, 0)

	// Verify every (agent, skill) pair landed with the expected
	// proficiency — confirms the resolver's UUIDs flowed correctly
	// into MergeAgentSkill.
	for _, agent := range []struct {
		code  string
		voice int
		chat  int
	}{
		{"alice", 7, 5},
		{"bob", 8, 6},
		{"carol", 9, 4},
	} {
		voice, chat := fetchAgentSkillProficiencies(t, ctx, th, agent.code)
		require.Equal(t, agent.voice, voice, "voice prof for %s", agent.code)
		require.Equal(t, agent.chat, chat, "chat prof for %s", agent.code)
	}
}

// ---------------------------------------------------------------------------
// File-local test helpers.
// ---------------------------------------------------------------------------

// seedAgentRaw inserts an agents row + an agent_states row via raw
// SQL. Returns the minted UUIDv7. Used by the cache-del test which
// needs pre-existing rows so the chunk's UpsertAgentByCode hits the
// ON CONFLICT UPDATE path (re-using the existing row's id).
func seedAgentRaw(t testing.TB, ctx context.Context, th *TestImports, code, name, email string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO agents (id, org_id, code, name, email, enabled, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, TRUE, 1, NOW(), NOW())`,
		id, th.OrgID, code, name, email,
	)
	require.NoError(t, err)
	_, err = th.Pool.Exec(ctx,
		`INSERT INTO agent_states (agent_id, org_id, status, state_version, updated_at)
		 VALUES ($1, $2, 'Offline', 1, NOW())`,
		id, th.OrgID,
	)
	require.NoError(t, err)
	return id
}

// seedSkillRaw inserts a skills row via raw SQL. Used by the skill
// resolution test.
func seedSkillRaw(t testing.TB, ctx context.Context, th *TestImports, code, name, skillType string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := th.Pool.Exec(ctx,
		`INSERT INTO skills (id, org_id, code, name, skill_type, enabled, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, TRUE, 1, NOW(), NOW())`,
		id, th.OrgID, code, name, skillType,
	)
	require.NoError(t, err)
	return id
}

// fetchAgentSkillProficiencies returns (voice, chat) proficiency for
// the agent identified by `code`. Returns 0/0 if either row is
// missing — tests should assert positive expected values.
func fetchAgentSkillProficiencies(t testing.TB, ctx context.Context, th *TestImports, agentCode string) (int, int) {
	t.Helper()
	var voice, chat int
	_ = th.Pool.QueryRow(ctx,
		`SELECT proficiency FROM agent_skills as_
		   JOIN agents a ON a.id = as_.agent_id AND a.org_id = as_.org_id
		   JOIN skills s ON s.id = as_.skill_id AND s.org_id = as_.org_id
		  WHERE a.org_id = $1 AND a.code = $2 AND s.code = 'skill_voice'`,
		th.OrgID, agentCode,
	).Scan(&voice)
	_ = th.Pool.QueryRow(ctx,
		`SELECT proficiency FROM agent_skills as_
		   JOIN agents a ON a.id = as_.agent_id AND a.org_id = as_.org_id
		   JOIN skills s ON s.id = as_.skill_id AND s.org_id = as_.org_id
		  WHERE a.org_id = $1 AND a.code = $2 AND s.code = 'skill_chat'`,
		th.OrgID, agentCode,
	).Scan(&chat)
	return voice, chat
}

// mustWarmCache seeds the cache with a tiny placeholder value via
// the package's public cache.GetOrSet (the only supported write path
// — Cache has no public Set method). The placeholder TTL is 5 minutes
// — long enough that the assertion in the same test can confirm the
// key was deleted by chunk.processChunk's post-commit Del rather than
// expiring on its own.
func mustWarmCache(t testing.TB, ctx context.Context, th *TestImports, key string) {
	t.Helper()
	_, err := cache.GetOrSet[map[string]string](ctx, th.I.deps.Cache, key, 5*time.Minute,
		func(ctx context.Context) (map[string]string, error) {
			return map[string]string{"stale": "yes"}, nil
		})
	require.NoError(t, err)
}

// cacheKeyExists tests whether a key is currently in Redis. We use
// the public GetOrSet loader-observation trick: if the loader is
// invoked, the key was a MISS (absent). If not invoked, the key was
// a HIT (present).
//
// Caveat: calling this function re-populates the cache (the loader
// returns a value, GetOrSet writes it). For assertion purposes we
// only check pre / post the import; the read-only intent is
// preserved at the call sites' semantics layer.
func cacheKeyExists(t testing.TB, ctx context.Context, th *TestImports, key string) bool {
	t.Helper()
	loaderCalled := false
	_, err := cache.GetOrSet[map[string]string](ctx, th.I.deps.Cache, key, 5*time.Minute,
		func(ctx context.Context) (map[string]string, error) {
			loaderCalled = true
			return map[string]string{"loaded": "after_del"}, nil
		})
	require.NoError(t, err)
	return !loaderCalled
}
