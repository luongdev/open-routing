// agents_test.go — end-to-end integration tests for the agents handler
// established by Plan 03-06. This is the TEMPLATE Plans 03-07/08/09 copy
// for skills/queues/channels/adapters/break_reasons.
//
// Coverage matrix (one Test per must_have truth in 03-06-PLAN.md):
//
//	TestAgents_CreateThenGet                   — CAT-01 + CAT-11 (cache fill)
//	TestAgents_GetMissing                      — CAT-01 404
//	TestAgents_VersionConflict                 — CAT-08 (D-66 disambiguation)
//	TestAgents_SoftDelete                      — CAT-09 (D-65 idempotent-disabled)
//	TestAgents_SoftDelete_IncludeDisabled      — CAT-09 (include_disabled toggle)
//	TestAgents_Cursor                          — CAT-10 (pagination + N+1)
//	TestAgents_NameSearch                      — CAT-10 (ILIKE %name%)
//	TestAgents_CacheInvalidationOnUpdate       — CAT-11 + D-55
//	TestAgents_CacheInvalidationOnDelete       — CAT-11 + D-55
//	TestAgents_SkillsReplace                   — CAT-03 PUT-semantics
//	TestAgents_SkillsReplace_UnknownSkill      — CAT-03 + D-76 (Codex C1/C2)
//	TestAgents_OutOfRangeProficiency_422       — Codex C1 (UPDATE 422 invalid_value)
//	TestAgents_SkillsReplace_OutOfRangeProficiency_422 — Codex C1 (alias for grep)
//	TestAgents_Create_OutOfRangeProficiency_422 — Codex C2 (CREATE 422 invalid_value)
//	TestAgents_Create_UnknownSkillId_422       — Codex C2 (CREATE 422 invalid_reference + atomic rollback)
//	TestAgents_LimitOutOfRange                 — handler-level 400 invalid_body
//	TestAgents_BadCursor                       — DecodeCursor → 400 bad_cursor
//	TestAgents_CrossOrgGet404                  — FOUND-08 inline isolation probe
//
// Tests use `package catalog` so they can call unexported helpers (D-73 +
// Codex C7). The newTestHandlers helper builds the fixture per test;
// cleanCatalogTables resets state before each scenario.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// ---------------------------------------------------------------------------
// Per-test fixtures + helpers
// ---------------------------------------------------------------------------

// seedSkill inserts a skill directly via the generated.Queries (bypassing
// the HTTP handler, which isn't implemented until Plan 03-07). Returns
// the new skill_id (UUIDv7) so tests can reference it from agent skill
// assignments.
func seedSkill(t testing.TB, th *TestHandlers, ctx context.Context, name string) uuid.UUID {
	t.Helper()
	q := generated.New(th.Pool) // bypass OrgDB for the test seed — the seed
	// statement still includes org_id in the WHERE/INSERT, but uses the
	// bare pool so we don't have to thread ctx through OrgContext for a
	// trivial seed.
	id := uuid.Must(uuid.NewV7())
	_, err := q.InsertSkill(ctx, generated.InsertSkillParams{
		ID:         pgUUID(id),
		OrgID:      pgUUID(th.OrgID),
		ExternalID: "ext-" + name,
		Name:       name,
		SkillType:  "language",
		Enabled:    true,
	})
	require.NoError(t, err, "seedSkill: InsertSkill")
	return id
}

// agentPath returns the URL path for the agents collection.
func agentPath(orgID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/agents"
}

// agentDetailPath returns the URL path for a specific agent.
func agentDetailPath(orgID, agentID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/agents/" + agentID.String()
}

// postAgent issues a typical Create call with the given external_id,
// name, and optional skills. Returns the parsed Agent on 201; t.Fatalf
// on any other status.
func postAgent(t testing.TB, th *TestHandlers, body api.CreateAgentRequest) api.Agent {
	t.Helper()
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, agentPath(th.OrgID), body)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"postAgent: want 201, got %d body=%s", resp.StatusCode, string(raw))
	var a api.Agent
	require.NoError(t, json.Unmarshal(raw, &a), "postAgent: unmarshal Agent")
	return a
}

// makeAgentBody is a one-line constructor for a sensible default request body.
func makeAgentBody(externalID, name string, skills *[]api.AgentSkillAssignment) api.CreateAgentRequest {
	return api.CreateAgentRequest{
		ExternalId: externalID,
		Name:       name,
		Email:      openapi_types.Email(name + "@example.test"),
		Skills:     skills,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestAgents_CreateThenGet — POST returns 201 + Agent body with version=1,
// enabled=true, server-minted UUIDv7 id; GET returns 200 + same Agent; the
// SECOND GET in the 60s TTL window is served from cache (miniredis
// Exists(cache.Key) == true).
func TestAgents_CreateThenGet(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	// POST → 201
	body := makeAgentBody("ext-001", "Alice", nil)
	a := postAgent(t, th, body)
	require.Equal(t, 1, a.Version, "fresh agent version must be 1")
	require.True(t, a.Enabled, "fresh agent must default to enabled=true")
	require.Equal(t, "Alice", a.Name)
	require.Equal(t, openapi_types.Email("Alice@example.test"), a.Email)
	require.Equal(t, uuid.Version(7), uuid.UUID(a.Id).Version(), "id must be UUIDv7")
	require.Equal(t, th.OrgID, uuid.UUID(a.OrgId), "org_id must match ctx orgID")

	// First GET → 200 + cache miss → cache populated.
	resp, raw := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "first GET status, body=%s", string(raw))
	var got api.Agent
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, a.Id, got.Id, "id preserved across POST→GET")

	// Cache assertion — miniredis MUST contain the key after the GET.
	cacheKey := cache.Key(th.OrgID, "agents", uuid.UUID(a.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET (D-49)")

	// Second GET → still 200, still serves the same body. The cache
	// path is exercised; miniredis still has the key.
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var got2 api.Agent
	require.NoError(t, json.Unmarshal(raw2, &got2))
	require.Equal(t, got.Id, got2.Id, "id consistent across cached reads")
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key still present on second GET")
}

func TestAgents_CreateDuplicateExternalID_IncludesRequestID(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeAgentBody("ext-duplicate", "Dupe", nil)
	_ = postAgent(t, th, body)

	resp, raw := httpPOST(t, th.HTTP, th.OrgID, agentPath(th.OrgID), body)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))
	require.NotEmpty(t, resp.Header.Get("X-Request-Id"))

	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeVersionConflict, e.Error)
	require.Equal(t, "external_id_collision", e.Reason)
	require.NotNil(t, e.RequestId, "409 body must include request_id")
	require.Equal(t, resp.Header.Get("X-Request-Id"), e.RequestId.String())
}

// TestAgents_GetMissing — GET an id that never existed → 404 with
// error="not_found".
func TestAgents_GetMissing(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	unknown := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, unknown), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e), "unmarshal ErrorResponse, body=%s", string(raw))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestAgents_VersionConflict — POST → PATCH with stale version → 409 with
// {error: "version_conflict", reason: "version_mismatch", current: Agent};
// PATCH with correct version → 200 + bumped version.
func TestAgents_VersionConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAgent(t, th, makeAgentBody("ext-002", "Bob", nil))
	newName := "Bob v2"

	// First PATCH with wrong version → 409.
	bad := api.UpdateAgentRequest{Version: 99, Name: &newName}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), bad)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))

	// The 409 body has fields {current, error, reason, request_id?}.
	var body struct {
		Current api.Agent `json:"current"`
		Error   string    `json:"error"`
		Reason  string    `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "version_conflict", body.Error)
	require.Equal(t, "version_mismatch", body.Reason)
	require.Equal(t, a.Id, body.Current.Id, "409 body must echo the current row")
	require.Equal(t, 1, body.Current.Version, "current version unchanged after failed PATCH")

	// Correct version → 200 + version bumped.
	good := api.UpdateAgentRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), good)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var updated api.Agent
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, 2, updated.Version, "version must bump to 2")
	require.Equal(t, "Bob v2", updated.Name)
}

// TestAgents_SoftDelete — POST → DELETE → 204; GET → 404; second DELETE
// → 404 (idempotent-disabled, D-65); list default does NOT include.
func TestAgents_SoftDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAgent(t, th, makeAgentBody("ext-del", "Carol", nil))

	// DELETE → 204
	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// GET → 404
	resp2, _ := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	// Second DELETE → 404 (idempotent-disabled).
	resp3, _ := httpDELETE(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)))
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)

	// List default excludes the disabled row.
	resp4, raw4 := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), nil)
	require.Equal(t, http.StatusOK, resp4.StatusCode)
	var list api.ListAgents200JSONResponse
	require.NoError(t, json.Unmarshal(raw4, &list))
	for _, it := range list.Items {
		require.NotEqual(t, a.Id, it.Id, "list default must NOT contain soft-deleted agent")
	}
}

// TestAgents_SoftDelete_IncludeDisabled — same setup as SoftDelete, then
// ?include_disabled=true MUST return the row with enabled=false.
func TestAgents_SoftDelete_IncludeDisabled(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAgent(t, th, makeAgentBody("ext-del2", "Dave", nil))
	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	q := url.Values{"include_disabled": {"true"}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var list api.ListAgents200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &list))

	found := false
	for _, it := range list.Items {
		if it.Id == a.Id {
			found = true
			require.False(t, it.Enabled, "soft-deleted agent must surface with enabled=false")
		}
	}
	require.True(t, found, "?include_disabled=true must include the soft-deleted agent")
}

// TestAgents_Cursor — POST 60 agents; first list returns 25 + has_more +
// next_cursor; second page returns next 25; third page returns 10 +
// has_more=false. Cross-page id sets do not overlap.
func TestAgents_Cursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	const total = 60
	ids := make(map[uuid.UUID]struct{}, total)
	for i := 0; i < total; i++ {
		body := makeAgentBody(fmt.Sprintf("ext-%03d", i), fmt.Sprintf("Agent%03d", i), nil)
		a := postAgent(t, th, body)
		ids[uuid.UUID(a.Id)] = struct{}{}
		// Tiny pause so UUIDv7 timestamps differ by at least a microsecond
		// — keeps the (created_at, id) cursor strictly monotonic, avoiding
		// flaky ties under fast inserts.
		time.Sleep(1 * time.Millisecond)
	}

	seen := make(map[uuid.UUID]struct{}, total)

	// Page 1 — default 25
	resp, raw := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "page 1, body=%s", string(raw))
	var page1 api.ListAgents200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &page1))
	require.Equal(t, 25, len(page1.Items), "page 1 size")
	require.True(t, page1.HasMore, "page 1 has_more")
	require.NotNil(t, page1.NextCursor, "page 1 next_cursor")
	for _, it := range page1.Items {
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	// Page 2 — cursor from page 1, 25 more
	q2 := url.Values{"cursor": {*page1.NextCursor}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), q2)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "page 2, body=%s", string(raw2))
	var page2 api.ListAgents200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &page2))
	require.Equal(t, 25, len(page2.Items), "page 2 size")
	require.True(t, page2.HasMore, "page 2 has_more")
	require.NotNil(t, page2.NextCursor, "page 2 next_cursor")
	for _, it := range page2.Items {
		_, dup := seen[uuid.UUID(it.Id)]
		require.False(t, dup, "page 2 must not repeat page 1 ids")
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	// Page 3 — final 10
	q3 := url.Values{"cursor": {*page2.NextCursor}}
	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), q3)
	require.Equalf(t, http.StatusOK, resp3.StatusCode, "page 3, body=%s", string(raw3))
	var page3 api.ListAgents200JSONResponse
	require.NoError(t, json.Unmarshal(raw3, &page3))
	require.Equal(t, 10, len(page3.Items), "page 3 size")
	require.False(t, page3.HasMore, "page 3 has_more=false")
	require.Nil(t, page3.NextCursor, "page 3 next_cursor=nil")
	for _, it := range page3.Items {
		_, dup := seen[uuid.UUID(it.Id)]
		require.False(t, dup, "page 3 must not repeat earlier ids")
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	require.Equal(t, total, len(seen), "all 60 ids returned across the three pages")
}

// TestAgents_NameSearch — POST "Alice", "Alpine", "Bob"; ?name=al returns
// 2 (Alice + Alpine) case-insensitive.
func TestAgents_NameSearch(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	postAgent(t, th, makeAgentBody("ext-alice", "Alice", nil))
	postAgent(t, th, makeAgentBody("ext-alpine", "Alpine", nil))
	postAgent(t, th, makeAgentBody("ext-bob", "Bob", nil))

	q := url.Values{"name": {"al"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var list api.ListAgents200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &list))
	require.Equal(t, 2, len(list.Items), "?name=al must match 2 case-insensitive: Alice + Alpine")
	for _, it := range list.Items {
		require.Containsf(t, []string{"Alice", "Alpine"}, it.Name,
			"unexpected name in name-search results: %s", it.Name)
	}
}

// TestAgents_CacheInvalidationOnUpdate — POST → GET (cache warmed) →
// PATCH → cache MUST be DELed (D-55) so the next GET returns fresh data.
func TestAgents_CacheInvalidationOnUpdate(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAgent(t, th, makeAgentBody("ext-cache", "Eve", nil))
	// Warm the cache.
	resp, _ := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "agents", uuid.UUID(a.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache MUST be warm after GET")

	// PATCH bumps the row.
	newName := "Eve v2"
	body := api.UpdateAgentRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))

	// D-55 — cache key MUST be DELed after the commit.
	require.False(t, th.Miniredis.Exists(cacheKey), "cache MUST be DELed after PATCH (D-55)")

	// Next GET returns the UPDATED row (not a stale cached version).
	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	var got api.Agent
	require.NoError(t, json.Unmarshal(raw3, &got))
	require.Equal(t, "Eve v2", got.Name, "GET after PATCH must return fresh data")
	require.Equal(t, 2, got.Version, "GET after PATCH must show bumped version")
}

// TestAgents_CacheInvalidationOnDelete — POST → GET (warmed) → DELETE →
// next GET returns 404 (cache DEL must fire so the 404 isn't masked).
func TestAgents_CacheInvalidationOnDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAgent(t, th, makeAgentBody("ext-cache-del", "Frank", nil))
	// Warm cache.
	resp, _ := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "agents", uuid.UUID(a.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	// DELETE.
	resp2, _ := httpDELETE(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)))
	require.Equal(t, http.StatusNoContent, resp2.StatusCode)
	require.False(t, th.Miniredis.Exists(cacheKey), "cache DEL must fire after DELETE")

	// GET now returns 404 — would falsely return cached value if DEL skipped.
	resp3, _ := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)
}

// TestAgents_SkillsReplace — POST 2 skills; CREATE agent with
// skills=[s1@5,s2@7]; GET returns Agent with BOTH skills in skills[];
// PATCH agent with skills=[s2@9] only; GET returns Agent with only s2@9.
func TestAgents_SkillsReplace(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "english")
	s2 := seedSkill(t, th, ctx, "french")

	// CREATE with both skills.
	initial := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 5},
		{SkillId: api.UUIDv7(s2), Proficiency: 7},
	}
	a := postAgent(t, th, makeAgentBody("ext-skill", "Skiller", &initial))

	// GET → both skills present.
	resp, raw := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var got api.Agent
	require.NoError(t, json.Unmarshal(raw, &got))
	require.NotNil(t, got.Skills, "GET must include skills for detail")
	require.Equal(t, 2, len(*got.Skills), "must have both skills initially")

	// PATCH replaces with just s2@9.
	replace := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s2), Proficiency: 9},
	}
	body := api.UpdateAgentRequest{Version: 1, Skills: &replace}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))

	// GET after PATCH → only s2@9.
	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	var after api.Agent
	require.NoError(t, json.Unmarshal(raw3, &after))
	require.NotNil(t, after.Skills, "GET must include skills")
	require.Equal(t, 1, len(*after.Skills), "PATCH must replace, not merge")
	require.Equal(t, s2, uuid.UUID((*after.Skills)[0].SkillId), "remaining skill is s2")
	require.Equal(t, 9, (*after.Skills)[0].Proficiency, "proficiency updated to 9")
}

// TestAgents_SkillsReplace_UnknownSkill — POST agent; PATCH agent with
// skills=[random_uuid@5] → 422 invalid_reference. Agent unchanged.
func TestAgents_SkillsReplace_UnknownSkill(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAgent(t, th, makeAgentBody("ext-unknown", "Hank", nil))
	unknown := uuid.Must(uuid.NewV7())
	replace := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(unknown), Proficiency: 5},
	}
	body := api.UpdateAgentRequest{Version: 1, Skills: &replace}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"want 422 invalid_reference, body=%s", string(raw))

	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
	require.Contains(t, e.Reason, "unknown_skill_id:", "Reason includes the missing id sentinel")
	require.Contains(t, e.Reason, unknown.String(), "Reason mentions the actual missing id")

	// Agent version unchanged (tx rolled back).
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var after api.Agent
	require.NoError(t, json.Unmarshal(raw2, &after))
	require.Equal(t, 1, after.Version, "tx rolled back — version stays 1")
}

// TestAgents_OutOfRangeProficiency_422 — PATCH agent with skills=[s1@11]
// → 422 invalid_value (Codex C1: NOT 400). Reason includes
// "skills[0].proficiency" so client UI can point at the bad field.
func TestAgents_OutOfRangeProficiency_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "spanish")
	a := postAgent(t, th, makeAgentBody("ext-proficiency", "Iris", nil))

	bad := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 11}, // out of [1..10] range
	}
	body := api.UpdateAgentRequest{Version: 1, Skills: &bad}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"Codex C1 — out-of-range proficiency MUST be 422 invalid_value (NOT 400), body=%s", string(raw))

	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidValue, e.Error, "ErrorCode must be invalid_value")
	require.Contains(t, e.Reason, "skills[0].proficiency", "Reason must point at the bad slot")
}

// TestAgents_SkillsReplace_OutOfRangeProficiency_422 — grep-friendly
// alias for the PATCH proficiency-out-of-range test (Codex C1 acceptance
// criterion in 03-06-PLAN.md requires this exact name).
func TestAgents_SkillsReplace_OutOfRangeProficiency_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "italian")
	a := postAgent(t, th, makeAgentBody("ext-proficiency-2", "Jack", nil))

	bad := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 0}, // below range
	}
	body := api.UpdateAgentRequest{Version: 1, Skills: &bad}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, agentDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"proficiency=0 must be 422 invalid_value, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidValue, e.Error)
}

// TestAgents_Create_OutOfRangeProficiency_422 — Codex C2 iter 3. POST
// CreateAgent with skills=[s1@11] → 422 invalid_value via
// CreateAgent422JSONResponse (same shape as Update422). Agent NOT
// created — verified by GETting the list and finding no row with the
// external_id.
func TestAgents_Create_OutOfRangeProficiency_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s1 := seedSkill(t, th, ctx, "german")
	bad := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 11},
	}
	body := makeAgentBody("ext-create-422", "Kira", &bad)

	resp, raw := httpPOST(t, th.HTTP, th.OrgID, agentPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"Codex C2 — Create with proficiency=11 MUST be 422 invalid_value, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidValue, e.Error)
	require.Contains(t, e.Reason, "skills[0].proficiency",
		"Reason must point at the bad slot (Codex C1)")
}

// TestAgents_Create_UnknownSkillId_422 — Codex C2 iter 3 + Codex C4. POST
// CreateAgent with skills=[random_uuid@5] → 422 invalid_reference; the
// agent row MUST NOT have been created (tx rolled back atomically per
// Codex C4 — both the InsertAgent AND the skills replace run in one
// BeginTx).
func TestAgents_Create_UnknownSkillId_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	unknown := uuid.Must(uuid.NewV7())
	bad := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(unknown), Proficiency: 5},
	}
	const externalID = "ext-create-422-unknown"
	body := makeAgentBody(externalID, "Liam", &bad)

	resp, raw := httpPOST(t, th.HTTP, th.OrgID, agentPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"want 422 invalid_reference, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
	require.Contains(t, e.Reason, "unknown_skill_id:")
	require.Contains(t, e.Reason, unknown.String())

	// Codex C4 atomicity — the agent row must NOT be in the DB. Probe
	// via List and assert no row with our external_id exists. Use
	// ?include_disabled=true so a tx-rollback can't be hidden by an
	// enabled=false flip.
	q := url.Values{"include_disabled": {"true"}, "limit": {"100"}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), q)
	require.Equal(t, http.StatusOK, resp2.StatusCode, "list status")
	var list api.ListAgents200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &list))
	for _, it := range list.Items {
		require.NotEqualf(t, externalID, it.ExternalId,
			"Codex C4 — CreateAgent tx MUST have rolled back; found agent with external_id=%s", externalID)
	}
}

// Wave 4 review: duplicate skill_id in the request body must surface as
// 422 invalid_value, not the 409 the DB UNIQUE constraint would yield
// if the request reached InsertAgentSkill.
func TestAgents_Create_DuplicateSkillId_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)
	s1 := seedSkill(t, th, ctx, "SKILL-DUP")

	dup := []api.AgentSkillAssignment{
		{SkillId: api.UUIDv7(s1), Proficiency: 5},
		{SkillId: api.UUIDv7(s1), Proficiency: 7},
	}
	body := makeAgentBody("ext-create-dup", "Mia", &dup)
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, agentPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"want 422 invalid_value, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidValue, e.Error)
	require.Contains(t, e.Reason, "duplicate")
}

// TestAgents_LimitOutOfRange — ?limit=0 → 400 invalid_body;
// ?limit=101 → 400 invalid_body. Layer 1 in the OpenAPI spec does NOT
// enforce min/max on LimitQuery (the spec says "Defaults to 25.
// Maximum 100." in the description but no minimum/maximum keywords),
// so the handler is the source of the 400.
func TestAgents_LimitOutOfRange(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	cases := []struct {
		name  string
		limit string
	}{
		{"zero", "0"},
		{"too_high", "101"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := url.Values{"limit": {c.limit}}
			resp, raw := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), q)
			require.Equalf(t, http.StatusBadRequest, resp.StatusCode,
				"limit=%s must be 400, body=%s", c.limit, string(raw))
			var e api.ErrorResponse
			require.NoError(t, json.Unmarshal(raw, &e))
			require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
			require.Contains(t, e.Reason, "limit_out_of_range")
			n, parseErr := strconv.Atoi(c.limit)
			require.NoError(t, parseErr)
			require.Contains(t, e.Reason, fmt.Sprintf(":%d", n),
				"Reason must include the offending value")
		})
	}
}

// TestAgents_BadCursor — ?cursor=not-base64!!! → 400 with reason=bad_cursor.
func TestAgents_BadCursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := url.Values{"cursor": {"not-base64!!!"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, agentPath(th.OrgID), q)
	require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
	require.Equal(t, "bad_cursor", e.Reason)
}

// TestAgents_CrossOrgGet404 — POST agent in orgA; GET same id via orgB
// header → 404 (FOUND-08 isolation). Plan 03-10 isolation suite has a
// full cross-org probe; this is the inline canary so a regression at the
// handler level is caught immediately rather than in the late isolation
// pass.
func TestAgents_CrossOrgGet404(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	// Agent created in orgA (th.OrgID).
	a := postAgent(t, th, makeAgentBody("ext-cross", "Mara", nil))

	// orgB — fresh UUIDv7 — issues a GET with the SAME id.
	orgB := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, orgB, agentDetailPath(orgB, uuid.UUID(a.Id)), nil)
	require.Equalf(t, http.StatusNotFound, resp.StatusCode,
		"cross-org GET MUST return 404 (FOUND-08), body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}
