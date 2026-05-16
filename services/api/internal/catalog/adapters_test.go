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

// ---------------------------------------------------------------------------
// Per-test fixtures + helpers
// ---------------------------------------------------------------------------

func adapterPath(orgID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/adapters"
}

func adapterDetailPath(orgID, adapterID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/adapters/" + adapterID.String()
}

func postAdapter(t testing.TB, th *TestHandlers, body api.CreateAdapterRequest) api.Adapter {
	t.Helper()
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, adapterPath(th.OrgID), body)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"postAdapter: want 201, got %d body=%s", resp.StatusCode, string(raw))
	var a api.Adapter
	require.NoError(t, json.Unmarshal(raw, &a), "postAdapter: unmarshal Adapter")
	return a
}

func makeAdapterBody(name, adapterType string, cfg map[string]any) api.CreateAdapterRequest {
	body := api.CreateAdapterRequest{
		Name:        name,
		AdapterType: adapterType,
	}
	if cfg != nil {
		body.Config = &cfg
	}
	return body
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestAdapters_CreateThenGet — POST 201 + Adapter body; GET 200 + cache key
// populated; subsequent GET still 200.
func TestAdapters_CreateThenGet(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeAdapterBody("FreeSWITCH Edge", "freeswitch", map[string]any{
		"endpoint": "sip:edge.example.test:5060",
		"timeout":  float64(30),
	})
	a := postAdapter(t, th, body)
	require.Equal(t, 1, a.Version, "fresh adapter version must be 1")
	require.True(t, a.Enabled, "fresh adapter must default to enabled=true")
	require.Equal(t, "FreeSWITCH Edge", a.Name)
	require.Equal(t, "freeswitch", a.AdapterType)
	require.Equal(t, uuid.Version(7), uuid.UUID(a.Id).Version())
	require.Equal(t, th.OrgID, uuid.UUID(a.OrgId))
	require.NotNil(t, a.Config)
	require.Equal(t, "sip:edge.example.test:5060", (*a.Config)["endpoint"])

	resp, raw := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "first GET status, body=%s", string(raw))
	var got api.Adapter
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, a.Id, got.Id)

	cacheKey := cache.Key(th.OrgID, "adapters", uuid.UUID(a.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET (D-49)")

	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var got2 api.Adapter
	require.NoError(t, json.Unmarshal(raw2, &got2))
	require.Equal(t, got.Id, got2.Id)
	require.True(t, th.Miniredis.Exists(cacheKey))
}

// TestAdapters_GetMissing — GET an id that never existed → 404.
func TestAdapters_GetMissing(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	unknown := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, unknown), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e), "body=%s", string(raw))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestAdapters_VersionConflict — PATCH with stale version → 409 + current;
// PATCH with correct version → 200 + bumped version.
func TestAdapters_VersionConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAdapter(t, th, makeAdapterBody("LiveKit Gateway", "livekit", nil))
	newName := "LiveKit Gateway v2"

	bad := api.UpdateAdapterRequest{Version: 99, Name: &newName}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), bad)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))

	var body struct {
		Current api.Adapter `json:"current"`
		Error   string      `json:"error"`
		Reason  string      `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "version_conflict", body.Error)
	require.Equal(t, "version_mismatch", body.Reason)
	require.Equal(t, a.Id, body.Current.Id)
	require.Equal(t, 1, body.Current.Version)

	good := api.UpdateAdapterRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), good)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var updated api.Adapter
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, 2, updated.Version)
	require.Equal(t, newName, updated.Name)
}

// TestAdapters_SoftDelete — DELETE → 204; GET → 404; second DELETE → 404.
func TestAdapters_SoftDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAdapter(t, th, makeAdapterBody("Twilio Bridge", "twilio", nil))

	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp2, _ := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	resp3, _ := httpDELETE(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)))
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)

	resp4, raw4 := httpGET(t, th.HTTP, th.OrgID, adapterPath(th.OrgID), nil)
	require.Equal(t, http.StatusOK, resp4.StatusCode)
	var list api.ListAdapters200JSONResponse
	require.NoError(t, json.Unmarshal(raw4, &list))
	for _, it := range list.Items {
		require.NotEqual(t, a.Id, it.Id, "list default must NOT contain soft-deleted adapter")
	}
}

// TestAdapters_SoftDelete_IncludeDisabled — ?include_disabled=true MUST
// surface the soft-deleted row with enabled=false.
func TestAdapters_SoftDelete_IncludeDisabled(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAdapter(t, th, makeAdapterBody("Vonage VBC", "vonage", nil))
	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	q := url.Values{"include_disabled": {"true"}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, adapterPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "body=%s", string(raw2))
	var list api.ListAdapters200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &list))

	found := false
	for _, it := range list.Items {
		if it.Id == a.Id {
			found = true
			require.False(t, it.Enabled)
		}
	}
	require.True(t, found, "?include_disabled=true must include the soft-deleted adapter")
}

// TestAdapters_Cursor — POST 60 adapters; verify pagination across 3 pages.
func TestAdapters_Cursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	const total = 60
	ids := make(map[uuid.UUID]struct{}, total)
	for i := 0; i < total; i++ {
		body := makeAdapterBody(fmt.Sprintf("Adapter%03d", i), "freeswitch", nil)
		a := postAdapter(t, th, body)
		ids[uuid.UUID(a.Id)] = struct{}{}
		time.Sleep(1 * time.Millisecond)
	}

	seen := make(map[uuid.UUID]struct{}, total)

	resp, raw := httpGET(t, th.HTTP, th.OrgID, adapterPath(th.OrgID), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "page 1, body=%s", string(raw))
	var page1 api.ListAdapters200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &page1))
	require.Equal(t, 25, len(page1.Items))
	require.True(t, page1.HasMore)
	require.NotNil(t, page1.NextCursor)
	for _, it := range page1.Items {
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q2 := url.Values{"cursor": {*page1.NextCursor}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, adapterPath(th.OrgID), q2)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "page 2, body=%s", string(raw2))
	var page2 api.ListAdapters200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &page2))
	require.Equal(t, 25, len(page2.Items))
	require.True(t, page2.HasMore)
	for _, it := range page2.Items {
		_, dup := seen[uuid.UUID(it.Id)]
		require.False(t, dup)
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q3 := url.Values{"cursor": {*page2.NextCursor}}
	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, adapterPath(th.OrgID), q3)
	require.Equalf(t, http.StatusOK, resp3.StatusCode, "page 3, body=%s", string(raw3))
	var page3 api.ListAdapters200JSONResponse
	require.NoError(t, json.Unmarshal(raw3, &page3))
	require.Equal(t, 10, len(page3.Items))
	require.False(t, page3.HasMore)
	require.Nil(t, page3.NextCursor)
	for _, it := range page3.Items {
		_, dup := seen[uuid.UUID(it.Id)]
		require.False(t, dup)
		seen[uuid.UUID(it.Id)] = struct{}{}
	}
	require.Equal(t, total, len(seen))
}

// TestAdapters_NameSearch — case-insensitive substring search on name.
func TestAdapters_NameSearch(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	postAdapter(t, th, makeAdapterBody("FreeSWITCH-East", "freeswitch", nil))
	postAdapter(t, th, makeAdapterBody("FreeSWITCH-West", "freeswitch", nil))
	postAdapter(t, th, makeAdapterBody("LiveKit-Edge", "livekit", nil))

	q := url.Values{"name": {"freeswitch"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, adapterPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var list api.ListAdapters200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &list))
	require.Equal(t, 2, len(list.Items))
	for _, it := range list.Items {
		require.Containsf(t, []string{"FreeSWITCH-East", "FreeSWITCH-West"}, it.Name,
			"unexpected name: %s", it.Name)
	}
}

// TestAdapters_CacheInvalidationOnUpdate — POST → GET (warm) → PATCH →
// cache MUST be DELed; next GET shows fresh state.
func TestAdapters_CacheInvalidationOnUpdate(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAdapter(t, th, makeAdapterBody("Cache-Adapter", "freeswitch", nil))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "adapters", uuid.UUID(a.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	newName := "Cache-Adapter v2"
	body := api.UpdateAdapterRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))

	require.False(t, th.Miniredis.Exists(cacheKey), "cache MUST be DELed after PATCH (D-55)")

	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	var got api.Adapter
	require.NoError(t, json.Unmarshal(raw3, &got))
	require.Equal(t, newName, got.Name)
	require.Equal(t, 2, got.Version)
}

// TestAdapters_CacheInvalidationOnDelete — DELETE wipes cache; next GET 404.
func TestAdapters_CacheInvalidationOnDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAdapter(t, th, makeAdapterBody("Cache-Del-Adapter", "freeswitch", nil))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "adapters", uuid.UUID(a.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	resp2, _ := httpDELETE(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)))
	require.Equal(t, http.StatusNoContent, resp2.StatusCode)
	require.False(t, th.Miniredis.Exists(cacheKey))

	resp3, _ := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)
}

// TestAdapters_ConfigJSONBRoundTrip — POST with deeply-nested config object;
// GET returns the same nested map (Pitfall 9). JSON numbers decode as
// float64 — the test fixture uses float-shaped values so deep equality
// holds without coercion gymnastics.
func TestAdapters_ConfigJSONBRoundTrip(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	cfg := map[string]any{
		"endpoint": "https://x.example.test/api",
		"timeout":  float64(30),
		"retries":  float64(3),
		"nested": map[string]any{
			"a": map[string]any{
				"b": map[string]any{
					"c": float64(1),
					"d": "deep",
				},
			},
		},
		"tags": []any{"east", "primary", float64(42)},
	}
	body := makeAdapterBody("Round-Trip", "custom", cfg)
	a := postAdapter(t, th, body)
	require.NotNil(t, a.Config)
	require.Equal(t, cfg, *a.Config, "POST response config matches input")

	resp, raw := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var got api.Adapter
	require.NoError(t, json.Unmarshal(raw, &got))
	require.NotNil(t, got.Config)
	require.Equal(t, cfg, *got.Config, "GET response config matches POST input deeply")
}

// TestAdapters_EmptyConfig — POST {config: {}}; GET returns {} (NOT null).
// Matches the spec additionalProperties:true semantics: empty config is
// "{}", never absent.
func TestAdapters_EmptyConfig(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeAdapterBody("Empty-Cfg", "custom", map[string]any{})
	a := postAdapter(t, th, body)
	require.NotNil(t, a.Config)
	require.Equal(t, map[string]any{}, *a.Config)

	resp, raw := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var got api.Adapter
	require.NoError(t, json.Unmarshal(raw, &got))
	require.NotNil(t, got.Config)
	require.Equal(t, map[string]any{}, *got.Config)
}

// TestAdapters_PatchConfigPreserved — POST with config A; PATCH without
// config (only name change); GET returns ORIGINAL config A. The sqlc
// UPDATE uses COALESCE($3::jsonb, config), so passing nil bytes preserves.
func TestAdapters_PatchConfigPreserved(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	original := map[string]any{
		"endpoint": "https://preserve.example.test",
		"port":     float64(8080),
	}
	a := postAdapter(t, th, makeAdapterBody("Preserve-Adapter", "freeswitch", original))

	newName := "Preserve-Adapter v2"
	body := api.UpdateAdapterRequest{Version: 1, Name: &newName}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "PATCH, body=%s", string(raw))

	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var got api.Adapter
	require.NoError(t, json.Unmarshal(raw2, &got))
	require.NotNil(t, got.Config)
	require.Equal(t, original, *got.Config, "PATCH without config MUST preserve existing config")
	require.Equal(t, newName, got.Name)
}

// TestAdapters_PatchConfigReplaced — POST config A; PATCH config B; GET
// returns config B.
func TestAdapters_PatchConfigReplaced(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	original := map[string]any{"k": "v_old"}
	a := postAdapter(t, th, makeAdapterBody("Replace-Adapter", "freeswitch", original))

	replacement := map[string]any{"k": "v_new", "extra": float64(99)}
	body := api.UpdateAdapterRequest{Version: 1, Config: &replacement}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), body)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "PATCH, body=%s", string(raw))

	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, adapterDetailPath(th.OrgID, uuid.UUID(a.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var got api.Adapter
	require.NoError(t, json.Unmarshal(raw2, &got))
	require.NotNil(t, got.Config)
	require.Equal(t, replacement, *got.Config)
}

// TestAdapters_CrossOrgGet404 — POST in orgA; GET via orgB → 404
// (FOUND-08 isolation canary; full suite in Plan 03-10).
func TestAdapters_CrossOrgGet404(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	a := postAdapter(t, th, makeAdapterBody("Cross-Org", "freeswitch", nil))

	orgB := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, orgB, adapterDetailPath(orgB, uuid.UUID(a.Id)), nil)
	require.Equalf(t, http.StatusNotFound, resp.StatusCode,
		"cross-org GET MUST return 404 (FOUND-08), body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestAdapters_LimitOutOfRange — ?limit=0 / ?limit=101 → 400.
func TestAdapters_LimitOutOfRange(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	cases := []string{"0", "101"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			q := url.Values{"limit": {c}}
			resp, raw := httpGET(t, th.HTTP, th.OrgID, adapterPath(th.OrgID), q)
			require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "limit=%s body=%s", c, string(raw))
			var e api.ErrorResponse
			require.NoError(t, json.Unmarshal(raw, &e))
			require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
			require.Contains(t, e.Reason, "limit_out_of_range")
		})
	}
}
