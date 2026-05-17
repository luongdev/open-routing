// break_reasons_test.go — end-to-end integration tests for the
// break_reasons handler (Plan 03-07). Mirrors skills_test.go's coverage
// with the description-nullable test swapped for routable + display_order
// + name-uniqueness probes.
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

func breakReasonPath(orgID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/break-reasons"
}

func breakReasonDetailPath(orgID, brID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/break-reasons/" + brID.String()
}

func postBreakReason(t testing.TB, th *TestHandlers, body api.CreateBreakReasonRequest) api.BreakReason {
	t.Helper()
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), body)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"postBreakReason: want 201, got %d body=%s", resp.StatusCode, string(raw))
	var br api.BreakReason
	require.NoError(t, json.Unmarshal(raw, &br), "postBreakReason: unmarshal BreakReason")
	return br
}

// Phase 04.1: signature gained leading `code string` arg (D04_1-03 — required).
// break_reasons pre-04.1 had no external_id column; post-04.1 it gains both
// `code` (required) and `external_id` (optional, nullable). The legacy helper
// did not take a code; callers now derive a unique code from the name via
// sanitizeForCode to keep the per-test fixture concise.
func makeBreakReasonBody(name string, routable bool, displayOrder int) api.CreateBreakReasonRequest {
	return api.CreateBreakReasonRequest{
		Code:         "break_" + sanitizeForCode(name),
		Name:         name,
		Routable:     routable,
		DisplayOrder: displayOrder,
	}
}

// TestBreakReasons_CreateThenGet — POST → 201; GET → 200; cache filled.
func TestBreakReasons_CreateThenGet(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br := postBreakReason(t, th, makeBreakReasonBody("Lunch", false, 10))
	require.Equal(t, 1, br.Version, "fresh break_reason version must be 1")
	require.True(t, br.Enabled, "fresh break_reason must default to enabled=true")
	require.Equal(t, "Lunch", br.Name)
	require.False(t, br.Routable, "Routable false must round-trip")
	require.Equal(t, 10, br.DisplayOrder)
	require.Equal(t, uuid.Version(7), uuid.UUID(br.Id).Version(), "id must be UUIDv7")

	resp, raw := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "first GET status, body=%s", string(raw))
	var got api.BreakReason
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, br.Id, got.Id, "id preserved across POST→GET")

	cacheKey := cache.Key(th.OrgID, "break_reasons", uuid.UUID(br.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET (D-49)")

	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var got2 api.BreakReason
	require.NoError(t, json.Unmarshal(raw2, &got2))
	require.Equal(t, got.Id, got2.Id)
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key still present on second GET")
}

// TestBreakReasons_GetMissing — unknown id → 404.
func TestBreakReasons_GetMissing(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	unknown := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, unknown), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e), "unmarshal ErrorResponse, body=%s", string(raw))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestBreakReasons_VersionConflict — D-66 disambiguation (CAT-08).
func TestBreakReasons_VersionConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br := postBreakReason(t, th, makeBreakReasonBody("Coffee", true, 20))
	newName := "Coffee v2"

	bad := api.UpdateBreakReasonRequest{Version: 99, Name: &newName}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), bad)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))

	var body struct {
		Current api.BreakReason `json:"current"`
		Error   string          `json:"error"`
		Reason  string          `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "version_conflict", body.Error)
	require.Equal(t, "version_mismatch", body.Reason)
	require.Equal(t, br.Id, body.Current.Id, "409 body must echo the current row")
	require.Equal(t, 1, body.Current.Version, "current version unchanged after failed PATCH")

	good := api.UpdateBreakReasonRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), good)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var updated api.BreakReason
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, 2, updated.Version, "version must bump to 2")
	require.Equal(t, "Coffee v2", updated.Name)
}

// TestBreakReasons_SoftDelete — POST → DELETE → 204; GET → 404; idempotent.
func TestBreakReasons_SoftDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br := postBreakReason(t, th, makeBreakReasonBody("Training", false, 30))

	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp2, _ := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	resp3, _ := httpDELETE(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)))
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)

	resp4, raw4 := httpGET(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), nil)
	require.Equal(t, http.StatusOK, resp4.StatusCode)
	var list api.ListBreakReasons200JSONResponse
	require.NoError(t, json.Unmarshal(raw4, &list))
	for _, it := range list.Items {
		require.NotEqual(t, br.Id, it.Id, "list default must NOT contain soft-deleted break_reason")
	}
}

// TestBreakReasons_SoftDelete_IncludeDisabled — ?include_disabled=true.
func TestBreakReasons_SoftDelete_IncludeDisabled(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br := postBreakReason(t, th, makeBreakReasonBody("AwayBio", false, 40))
	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	q := url.Values{"include_disabled": {"true"}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var list api.ListBreakReasons200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &list))

	found := false
	for _, it := range list.Items {
		if it.Id == br.Id {
			found = true
			require.False(t, it.Enabled, "soft-deleted break_reason must surface with enabled=false")
		}
	}
	require.True(t, found, "?include_disabled=true must include the soft-deleted break_reason")
}

// TestBreakReasons_Cursor — POST 60; 25/25/10 across three pages.
func TestBreakReasons_Cursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	const total = 60
	ids := make(map[uuid.UUID]struct{}, total)
	for i := 0; i < total; i++ {
		body := makeBreakReasonBody(fmt.Sprintf("BR%03d", i), false, i)
		br := postBreakReason(t, th, body)
		ids[uuid.UUID(br.Id)] = struct{}{}
		time.Sleep(1 * time.Millisecond)
	}

	seen := make(map[uuid.UUID]struct{}, total)

	resp, raw := httpGET(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "page 1, body=%s", string(raw))
	var page1 api.ListBreakReasons200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &page1))
	require.Equal(t, 25, len(page1.Items), "page 1 size")
	require.True(t, page1.HasMore)
	require.NotNil(t, page1.NextCursor)
	for _, it := range page1.Items {
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q2 := url.Values{"cursor": {*page1.NextCursor}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), q2)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "page 2, body=%s", string(raw2))
	var page2 api.ListBreakReasons200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &page2))
	require.Equal(t, 25, len(page2.Items))
	require.True(t, page2.HasMore)
	require.NotNil(t, page2.NextCursor)
	for _, it := range page2.Items {
		_, dup := seen[uuid.UUID(it.Id)]
		require.False(t, dup, "page 2 must not repeat page 1 ids")
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q3 := url.Values{"cursor": {*page2.NextCursor}}
	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), q3)
	require.Equalf(t, http.StatusOK, resp3.StatusCode, "page 3, body=%s", string(raw3))
	var page3 api.ListBreakReasons200JSONResponse
	require.NoError(t, json.Unmarshal(raw3, &page3))
	require.Equal(t, 10, len(page3.Items))
	require.False(t, page3.HasMore)
	require.Nil(t, page3.NextCursor)
	for _, it := range page3.Items {
		_, dup := seen[uuid.UUID(it.Id)]
		require.False(t, dup, "page 3 must not repeat earlier ids")
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	require.Equal(t, total, len(seen), "all 60 ids returned across the three pages")
}

// TestBreakReasons_NameSearch — case-insensitive ILIKE on name.
func TestBreakReasons_NameSearch(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	postBreakReason(t, th, makeBreakReasonBody("Lunch", false, 10))
	postBreakReason(t, th, makeBreakReasonBody("Late Lunch", false, 11))
	postBreakReason(t, th, makeBreakReasonBody("Meeting", false, 20))

	q := url.Values{"name": {"lunch"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var list api.ListBreakReasons200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &list))
	require.Equal(t, 2, len(list.Items), "?name=lunch must match 2: Lunch + Late Lunch")
	for _, it := range list.Items {
		require.Containsf(t, []string{"Lunch", "Late Lunch"}, it.Name, "unexpected: %s", it.Name)
	}
}

// TestBreakReasons_CacheInvalidationOnUpdate — D-55 cache DEL post-commit.
func TestBreakReasons_CacheInvalidationOnUpdate(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br := postBreakReason(t, th, makeBreakReasonBody("Coffee", true, 10))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "break_reasons", uuid.UUID(br.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache MUST be warm after GET")

	newName := "Espresso"
	body := api.UpdateBreakReasonRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), body)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))

	require.False(t, th.Miniredis.Exists(cacheKey), "cache MUST be DELed after PATCH (D-55)")

	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	var got api.BreakReason
	require.NoError(t, json.Unmarshal(raw3, &got))
	require.Equal(t, "Espresso", got.Name)
	require.Equal(t, 2, got.Version)
}

// TestBreakReasons_CacheInvalidationOnDelete — DELETE invalidates cache.
func TestBreakReasons_CacheInvalidationOnDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br := postBreakReason(t, th, makeBreakReasonBody("AFK", false, 50))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "break_reasons", uuid.UUID(br.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	resp2, _ := httpDELETE(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)))
	require.Equal(t, http.StatusNoContent, resp2.StatusCode)
	require.False(t, th.Miniredis.Exists(cacheKey), "cache DEL must fire after DELETE")

	resp3, _ := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)
}

// TestBreakReasons_RoutableFlag — round-trip both routable=true and
// routable=false (CAT-07).
func TestBreakReasons_RoutableFlag(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br1 := postBreakReason(t, th, makeBreakReasonBody("ShortBreak", true, 5))
	require.True(t, br1.Routable, "POST routable=true must round-trip")

	br2 := postBreakReason(t, th, makeBreakReasonBody("Lunch", false, 10))
	require.False(t, br2.Routable, "POST routable=false must round-trip")

	resp, raw := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br1.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got1 api.BreakReason
	require.NoError(t, json.Unmarshal(raw, &got1))
	require.True(t, got1.Routable, "GET preserves routable=true")

	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br2.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var got2 api.BreakReason
	require.NoError(t, json.Unmarshal(raw2, &got2))
	require.False(t, got2.Routable, "GET preserves routable=false")

	// PATCH flips the routable flag.
	flip := false
	body := api.UpdateBreakReasonRequest{Version: 1, Routable: &flip}
	resp3, raw3 := httpPATCH(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br1.Id)), body)
	require.Equalf(t, http.StatusOK, resp3.StatusCode, "PATCH body=%s", string(raw3))
	var flipped api.BreakReason
	require.NoError(t, json.Unmarshal(raw3, &flipped))
	require.False(t, flipped.Routable, "PATCH must flip routable to false")
}

// TestBreakReasons_DisplayOrder — display_order round-trips on POST/PATCH;
// list ordering is by cursor pagination (created_at, id) DESC per CAT-10
// (NOT by display_order — that's the UI sort path, not the list-API path).
func TestBreakReasons_DisplayOrder(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br1 := postBreakReason(t, th, makeBreakReasonBody("Order10", false, 10))
	time.Sleep(1 * time.Millisecond)
	br2 := postBreakReason(t, th, makeBreakReasonBody("Order20", false, 20))
	time.Sleep(1 * time.Millisecond)
	br3 := postBreakReason(t, th, makeBreakReasonBody("Order05", false, 5))

	require.Equal(t, 10, br1.DisplayOrder)
	require.Equal(t, 20, br2.DisplayOrder)
	require.Equal(t, 5, br3.DisplayOrder)

	// PATCH br3 to bump its display_order; round-trip the new value.
	newOrder := 99
	body := api.UpdateBreakReasonRequest{Version: 1, DisplayOrder: &newOrder}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(br3.Id)), body)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "PATCH body=%s", string(raw))
	var updated api.BreakReason
	require.NoError(t, json.Unmarshal(raw, &updated))
	require.Equal(t, 99, updated.DisplayOrder, "PATCH display_order must round-trip")

	// List default ordering is by (created_at, id) DESC — newest first.
	// br3 was last-posted, so it appears first in the list response.
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), nil)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "list, body=%s", string(raw2))
	var list api.ListBreakReasons200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &list))
	require.Equal(t, 3, len(list.Items))
	require.Equal(t, br3.Id, list.Items[0].Id, "newest-posted (br3) appears first under (created_at, id) DESC")
	require.Equal(t, br2.Id, list.Items[1].Id)
	require.Equal(t, br1.Id, list.Items[2].Id)
}

// TestBreakReasons_DuplicateName_NoConflict — Phase 04.1 (IDENT-03 + D-CTX).
// Plan 01 dropped the legacy UNIQUE(org_id, name) constraint; the canonical
// identifier moved to `code`. Two break_reasons in the same org may share
// the same display name as long as their codes differ. Replaces the
// pre-04.1 TestBreakReasons_NameUniqueCollision (which expected 409
// name_collision). This is the 8th bonus test for break_reasons (D04_1-23
// + Plan 05 must_haves).
func TestBreakReasons_DuplicateName_NoConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeBreakReasonBodyWithCode("break_lunch", "", "Lunch", false, 10)
	_ = postBreakReason(t, th, body)

	// Same name "Lunch", different code → must succeed (IDENT-03 dropped
	// UNIQUE(org_id, name)).
	body2 := makeBreakReasonBodyWithCode("break_lunch_2", "", "Lunch", true, 20)
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), body2)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"body=%s — duplicate name with distinct code must succeed post Phase 04.1 (IDENT-03 dropped UNIQUE(org_id, name))", string(raw))
}

// TestBreakReasons_CrossOrgGet404 — FOUND-08 inline isolation canary.
func TestBreakReasons_CrossOrgGet404(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	br := postBreakReason(t, th, makeBreakReasonBody("Lunch", false, 10))

	orgB := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, orgB, breakReasonDetailPath(orgB, uuid.UUID(br.Id)), nil)
	require.Equalf(t, http.StatusNotFound, resp.StatusCode,
		"cross-org GET MUST return 404 (FOUND-08), body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestBreakReasons_BadCursor — ?cursor=garbage → 400.
func TestBreakReasons_BadCursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := url.Values{"cursor": {"not-base64!!!"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), q)
	require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
	require.Equal(t, "bad_cursor", e.Reason)
}

// ---------------------------------------------------------------------------
// Phase 04.1 — 7 standard tests (D04_1-23 + VALIDATION IDENT-02).
//
// break_reasons pre-04.1 had UNIQUE(org_id, name) and NO external_id column.
// Plan 01 dropped UNIQUE(name), added `code` (required) + `external_id`
// (nullable). The 8th bonus test (TestBreakReasons_DuplicateName_NoConflict)
// lives in place of the old TestBreakReasons_NameUniqueCollision above.
//
// makeBreakReasonBody derives Code from name via sanitizeForCode; the
// makeBreakReasonBodyWithCode variant fixes Code explicitly (Plan 04
// SUMMARY Hazard #3) so the tests can mutate it in isolation.
// ---------------------------------------------------------------------------

// TestBreakReasons_MissingCode_Returns400 — Layer 1 rejects empty code.
func TestBreakReasons_MissingCode_Returns400(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeBreakReasonBodyWithCode("", "", "EmptyCode", false, 10)
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), body)
	require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
	require.Equal(t, "invalid_code_format", e.Reason)
}

// TestBreakReasons_DuplicateCode_Returns409_DuplicateCode — composite UNIQUE on
// (org_id, code) fires; mapPgError returns duplicate_code.
func TestBreakReasons_DuplicateCode_Returns409_DuplicateCode(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeBreakReasonBodyWithCode("break_training", "ext-br-001", "Training", false, 30)
	_ = postBreakReason(t, th, body)

	body2 := makeBreakReasonBodyWithCode("break_training", "ext-br-002", "Training2", true, 31)
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), body2)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeDuplicateCode, e.Error)
	require.Equal(t, "duplicate_code", e.Reason)
}

// TestBreakReasons_DuplicateExternalId_Returns409_DuplicateExternalId — partial
// UNIQUE on (org_id, external_id) fires; external_id is NEW for break_reasons
// in Phase 04.1.
func TestBreakReasons_DuplicateExternalId_Returns409_DuplicateExternalId(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeBreakReasonBodyWithCode("break_dx_001", "ext-br-001", "First", false, 40)
	_ = postBreakReason(t, th, body)

	body2 := makeBreakReasonBodyWithCode("break_dx_002", "ext-br-001", "Second", false, 41)
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), body2)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeDuplicateExternalId, e.Error)
	require.Equal(t, "duplicate_external_id", e.Reason)
}

// TestBreakReasons_PatchSameCode_Returns200 — PATCH with same code is a no-op.
func TestBreakReasons_PatchSameCode_Returns200(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeBreakReasonBodyWithCode("break_same_001", "ext-br-same", "Lunch", false, 10)
	created := postBreakReason(t, th, body)

	sameCode := "break_same_001"
	newName := "Lunch v2"
	patchBody := api.UpdateBreakReasonRequest{
		Version: created.Version,
		Code:    &sameCode,
		Name:    &newName,
	}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(created.Id)), patchBody)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var updated api.BreakReason
	require.NoError(t, json.Unmarshal(raw, &updated))
	require.Equal(t, sameCode, updated.Code)
	require.Equal(t, newName, updated.Name)
}

// TestBreakReasons_PatchDifferentCode_Returns422_ImmutableField — Layer 2 rejects.
func TestBreakReasons_PatchDifferentCode_Returns422_ImmutableField(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeBreakReasonBodyWithCode("break_diff_001", "ext-br-diff", "Coffee", true, 20)
	created := postBreakReason(t, th, body)

	newCode := "break_diff_002"
	patchBody := api.UpdateBreakReasonRequest{
		Version: created.Version,
		Code:    &newCode,
	}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(created.Id)), patchBody)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeImmutableField, e.Error)
	require.Equal(t, "code", e.Reason)
}

// TestBreakReasons_PatchInvalidCodeFormat_Returns400 — Layer 1 fires first.
func TestBreakReasons_PatchInvalidCodeFormat_Returns400(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeBreakReasonBodyWithCode("break_bad_001", "ext-br-bad", "Bad", false, 50)
	created := postBreakReason(t, th, body)

	badCode := "Coffee.Break" // dot → fails regex (only a-z0-9_)
	patchBody := api.UpdateBreakReasonRequest{
		Version: created.Version,
		Code:    &badCode,
	}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, breakReasonDetailPath(th.OrgID, uuid.UUID(created.Id)), patchBody)
	require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
	require.Equal(t, "invalid_code_format", e.Reason)
}

// TestBreakReasons_TwoNullExternalIds_NoConflict — IDENT-02. external_id is
// NEW for break_reasons in Phase 04.1; the partial unique index allows two NULLs.
func TestBreakReasons_TwoNullExternalIds_NoConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeBreakReasonBodyWithCode("break_null_001", "", "First", false, 60)
	_ = postBreakReason(t, th, body)

	body2 := makeBreakReasonBodyWithCode("break_null_002", "", "Second", true, 61)
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, breakReasonPath(th.OrgID), body2)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"body=%s — two NULL external_ids must coexist", string(raw))
}
