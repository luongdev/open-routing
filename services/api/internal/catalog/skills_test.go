// skills_test.go — end-to-end integration tests for the skills handler
// (Plan 03-07). Mirrors agents_test.go's coverage matrix minus the
// SkillsReplace_* family (skills has no embedded join table). Adds a
// description-nullable round-trip test.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
)

func skillPath(orgID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/skills"
}

func skillDetailPath(orgID, skillID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/skills/" + skillID.String()
}

func postSkill(t testing.TB, th *TestHandlers, body api.CreateSkillRequest) api.Skill {
	t.Helper()
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, skillPath(th.OrgID), body)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"postSkill: want 201, got %d body=%s", resp.StatusCode, string(raw))
	var s api.Skill
	require.NoError(t, json.Unmarshal(raw, &s), "postSkill: unmarshal Skill")
	return s
}

func makeSkillBody(externalID, name string, description *string) api.CreateSkillRequest {
	return api.CreateSkillRequest{
		ExternalId:  externalID,
		Name:        name,
		Description: description,
		SkillType:   "language",
	}
}

// TestSkills_CreateThenGet — POST returns 201 + Skill body with version=1,
// enabled=true; GET returns 200 + same Skill; cache populated after first
// GET (CAT-11).
func TestSkills_CreateThenGet(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeSkillBody("skl-001", "English", nil)
	s := postSkill(t, th, body)
	require.Equal(t, 1, s.Version, "fresh skill version must be 1")
	require.True(t, s.Enabled, "fresh skill must default to enabled=true")
	require.Equal(t, "English", s.Name)
	require.Equal(t, "language", s.SkillType)
	require.Equal(t, uuid.Version(7), uuid.UUID(s.Id).Version(), "id must be UUIDv7")

	// First GET → 200 + cache miss → cache populated.
	resp, raw := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "first GET status, body=%s", string(raw))
	var got api.Skill
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, s.Id, got.Id, "id preserved across POST→GET")

	cacheKey := cache.Key(th.OrgID, "skills", uuid.UUID(s.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET (D-49)")

	// Second GET — cache path exercised.
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var got2 api.Skill
	require.NoError(t, json.Unmarshal(raw2, &got2))
	require.Equal(t, got.Id, got2.Id, "id consistent across cached reads")
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key still present on second GET")
}

// TestSkills_GetMissing — GET an unknown id → 404.
func TestSkills_GetMissing(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	unknown := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, unknown), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e), "unmarshal ErrorResponse, body=%s", string(raw))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestSkills_VersionConflict — PATCH with stale version → 409 (D-66
// disambiguation); correct version → 200 + bumped version (CAT-08).
func TestSkills_VersionConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s := postSkill(t, th, makeSkillBody("skl-002", "French", nil))
	newName := "French v2"

	bad := api.UpdateSkillRequest{Version: 99, Name: &newName}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), bad)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))

	var body struct {
		Current api.Skill `json:"current"`
		Error   string    `json:"error"`
		Reason  string    `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "version_conflict", body.Error)
	require.Equal(t, "version_mismatch", body.Reason)
	require.Equal(t, s.Id, body.Current.Id, "409 body must echo the current row")
	require.Equal(t, 1, body.Current.Version, "current version unchanged after failed PATCH")

	good := api.UpdateSkillRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), good)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var updated api.Skill
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, 2, updated.Version, "version must bump to 2")
	require.Equal(t, "French v2", updated.Name)
}

// TestSkills_SoftDelete — POST → DELETE → 204; GET → 404; second DELETE
// → 404 (D-65); list default does NOT include.
func TestSkills_SoftDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s := postSkill(t, th, makeSkillBody("skl-del", "Spanish", nil))

	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp2, _ := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	resp3, _ := httpDELETE(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)))
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)

	resp4, raw4 := httpGET(t, th.HTTP, th.OrgID, skillPath(th.OrgID), nil)
	require.Equal(t, http.StatusOK, resp4.StatusCode)
	var list api.ListSkills200JSONResponse
	require.NoError(t, json.Unmarshal(raw4, &list))
	for _, it := range list.Items {
		require.NotEqual(t, s.Id, it.Id, "list default must NOT contain soft-deleted skill")
	}
}

// TestSkills_SoftDelete_IncludeDisabled — ?include_disabled=true surfaces
// the soft-deleted row with enabled=false.
func TestSkills_SoftDelete_IncludeDisabled(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s := postSkill(t, th, makeSkillBody("skl-del2", "Italian", nil))
	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	q := url.Values{"include_disabled": {"true"}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, skillPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var list api.ListSkills200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &list))

	found := false
	for _, it := range list.Items {
		if it.Id == s.Id {
			found = true
			require.False(t, it.Enabled, "soft-deleted skill must surface with enabled=false")
		}
	}
	require.True(t, found, "?include_disabled=true must include the soft-deleted skill")
}

// TestSkills_Cursor — POST 60 skills; three pages of 25/25/10 (CAT-10).
func TestSkills_Cursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	const total = 60
	ids := make(map[uuid.UUID]struct{}, total)
	for i := 0; i < total; i++ {
		body := makeSkillBody(fmt.Sprintf("skl-%03d", i), fmt.Sprintf("Skill%03d", i), nil)
		s := postSkill(t, th, body)
		ids[uuid.UUID(s.Id)] = struct{}{}
		// Tiny pause so UUIDv7 timestamps differ — keeps (created_at, id)
		// cursor strictly monotonic.
		time.Sleep(1 * time.Millisecond)
	}

	seen := make(map[uuid.UUID]struct{}, total)

	resp, raw := httpGET(t, th.HTTP, th.OrgID, skillPath(th.OrgID), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "page 1, body=%s", string(raw))
	var page1 api.ListSkills200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &page1))
	require.Equal(t, 25, len(page1.Items), "page 1 size")
	require.True(t, page1.HasMore, "page 1 has_more")
	require.NotNil(t, page1.NextCursor, "page 1 next_cursor")
	for _, it := range page1.Items {
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q2 := url.Values{"cursor": {*page1.NextCursor}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, skillPath(th.OrgID), q2)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "page 2, body=%s", string(raw2))
	var page2 api.ListSkills200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &page2))
	require.Equal(t, 25, len(page2.Items), "page 2 size")
	require.True(t, page2.HasMore, "page 2 has_more")
	require.NotNil(t, page2.NextCursor, "page 2 next_cursor")
	for _, it := range page2.Items {
		_, dup := seen[uuid.UUID(it.Id)]
		require.False(t, dup, "page 2 must not repeat page 1 ids")
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q3 := url.Values{"cursor": {*page2.NextCursor}}
	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, skillPath(th.OrgID), q3)
	require.Equalf(t, http.StatusOK, resp3.StatusCode, "page 3, body=%s", string(raw3))
	var page3 api.ListSkills200JSONResponse
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

// TestSkills_NameSearch — ?name=al case-insensitive ILIKE.
func TestSkills_NameSearch(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	postSkill(t, th, makeSkillBody("skl-alice", "Algebra", nil))
	postSkill(t, th, makeSkillBody("skl-alpine", "Alpine", nil))
	postSkill(t, th, makeSkillBody("skl-bob", "Botany", nil))

	q := url.Values{"name": {"al"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, skillPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var list api.ListSkills200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &list))
	require.Equal(t, 2, len(list.Items), "?name=al must match 2: Algebra + Alpine")
	for _, it := range list.Items {
		require.Containsf(t, []string{"Algebra", "Alpine"}, it.Name,
			"unexpected name in name-search results: %s", it.Name)
	}
}

// TestSkills_CacheInvalidationOnUpdate — D-55 cache DEL after commit.
func TestSkills_CacheInvalidationOnUpdate(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s := postSkill(t, th, makeSkillBody("skl-cache", "German", nil))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "skills", uuid.UUID(s.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache MUST be warm after GET")

	newName := "German v2"
	body := api.UpdateSkillRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), body)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))

	require.False(t, th.Miniredis.Exists(cacheKey), "cache MUST be DELed after PATCH (D-55)")

	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	var got api.Skill
	require.NoError(t, json.Unmarshal(raw3, &got))
	require.Equal(t, "German v2", got.Name, "GET after PATCH must return fresh data")
	require.Equal(t, 2, got.Version, "GET after PATCH must show bumped version")
}

// TestSkills_CacheInvalidationOnDelete — DELETE invalidates cache.
func TestSkills_CacheInvalidationOnDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s := postSkill(t, th, makeSkillBody("skl-cache-del", "Hindi", nil))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "skills", uuid.UUID(s.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	resp2, _ := httpDELETE(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)))
	require.Equal(t, http.StatusNoContent, resp2.StatusCode)
	require.False(t, th.Miniredis.Exists(cacheKey), "cache DEL must fire after DELETE")

	resp3, _ := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)
}

// TestSkills_DescriptionNullable — POST without description; GET returns
// no description; PATCH adds description; GET returns it.
func TestSkills_DescriptionNullable(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s := postSkill(t, th, makeSkillBody("skl-desc", "Japanese", nil))
	require.Nil(t, s.Description, "POST without description must return nil/omitted")

	resp, raw := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got api.Skill
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Nil(t, got.Description, "GET after POST without description must be nil")

	desc := "Native or bilingual proficiency"
	body := api.UpdateSkillRequest{Version: 1, Description: &desc}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), body)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))

	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, skillDetailPath(th.OrgID, uuid.UUID(s.Id)), nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	var got2 api.Skill
	require.NoError(t, json.Unmarshal(raw3, &got2))
	require.NotNil(t, got2.Description, "GET after PATCH must return the description")
	require.Equal(t, desc, *got2.Description)
}

// TestSkills_LimitOutOfRange — limit=0 or 101 → 400 invalid_body.
func TestSkills_LimitOutOfRange(t *testing.T) {
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
			resp, raw := httpGET(t, th.HTTP, th.OrgID, skillPath(th.OrgID), q)
			require.Equalf(t, http.StatusBadRequest, resp.StatusCode,
				"limit=%s must be 400, body=%s", c.limit, string(raw))
			var e api.ErrorResponse
			require.NoError(t, json.Unmarshal(raw, &e))
			require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
			require.Contains(t, e.Reason, "limit_out_of_range")
		})
	}
}

// TestSkills_BadCursor — ?cursor=garbage → 400 with reason=bad_cursor.
func TestSkills_BadCursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := url.Values{"cursor": {"not-base64!!!"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, skillPath(th.OrgID), q)
	require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
	require.Equal(t, "bad_cursor", e.Reason)
}

// TestSkills_CrossOrgGet404 — FOUND-08 inline isolation canary.
func TestSkills_CrossOrgGet404(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	s := postSkill(t, th, makeSkillBody("skl-cross", "Korean", nil))

	orgB := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, orgB, skillDetailPath(orgB, uuid.UUID(s.Id)), nil)
	require.Equalf(t, http.StatusNotFound, resp.StatusCode,
		"cross-org GET MUST return 404 (FOUND-08), body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestSkills_ExternalIdCollision — POST two skills with the same external_id
// in the same org → 409 with reason=external_id_collision (CAT-02).
func TestSkills_ExternalIdCollision(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	_ = postSkill(t, th, makeSkillBody("skl-dup", "First", nil))

	resp, raw := httpPOST(t, th.HTTP, th.OrgID, skillPath(th.OrgID),
		makeSkillBody("skl-dup", "Second", nil))
	require.Equalf(t, http.StatusConflict, resp.StatusCode,
		"second POST with same external_id must be 409, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeVersionConflict, e.Error)
	require.Equal(t, "external_id_collision", e.Reason)
}
