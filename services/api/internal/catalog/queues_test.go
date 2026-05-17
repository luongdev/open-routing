// queues_test.go — end-to-end queues handler tests (CAT-04). Mirrors the
// agents_test.go canonical pattern; queues-specific cases cover the
// channel_types[] round-trip + int round-trip.
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
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
)

// queuePath / queueDetailPath — URL helpers for the queues collection.
func queuePath(orgID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/queues"
}

func queueDetailPath(orgID, queueID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/queues/" + queueID.String()
}

// postQueue issues a Create call and returns the parsed Queue on 201.
func postQueue(t testing.TB, th *TestHandlers, body api.CreateQueueRequest) api.Queue {
	t.Helper()
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, queuePath(th.OrgID), body)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"postQueue: want 201, got %d body=%s", resp.StatusCode, string(raw))
	var q api.Queue
	require.NoError(t, json.Unmarshal(raw, &q), "postQueue: unmarshal Queue")
	return q
}

// makeQueueBody — sensible default body. Single channel_type voice for
// the cases that don't care; the channel_types round-trip test overrides.
func makeQueueBody(externalID, name string) api.CreateQueueRequest {
	return api.CreateQueueRequest{
		ExternalId:   externalID,
		Name:         name,
		ChannelTypes: []api.ChannelType{"voice"},
		Priority:     1,
		AcwSec:       0,
	}
}

// TestQueues_CreateThenGet — POST returns 201 + Queue with version=1,
// enabled=true, UUIDv7 id; GET serves from cache on second call.
func TestQueues_CreateThenGet(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeQueueBody("ext-001", "Support")
	q := postQueue(t, th, body)
	require.Equal(t, 1, q.Version, "fresh queue version must be 1")
	require.True(t, q.Enabled)
	require.Equal(t, "Support", q.Name)
	require.Equal(t, uuid.Version(7), uuid.UUID(q.Id).Version())
	require.Equal(t, th.OrgID, uuid.UUID(q.OrgId))

	resp, raw := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "first GET, body=%s", string(raw))
	var got api.Queue
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, q.Id, got.Id)

	cacheKey := cache.Key(th.OrgID, "queues", uuid.UUID(q.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET (D-49)")

	resp2, _ := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	require.True(t, th.Miniredis.Exists(cacheKey))
}

// TestQueues_GetMissing — GET unknown id → 404.
func TestQueues_GetMissing(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	unknown := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, unknown), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e), "body=%s", string(raw))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestQueues_VersionConflict — PATCH with stale version → 409 + current;
// PATCH with right version → 200 + bumped.
func TestQueues_VersionConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := postQueue(t, th, makeQueueBody("ext-vc", "Sales"))
	newName := "Sales v2"

	bad := api.UpdateQueueRequest{Version: 99, Name: &newName}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), bad)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))

	var body struct {
		Current api.Queue `json:"current"`
		Error   string    `json:"error"`
		Reason  string    `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "version_conflict", body.Error)
	require.Equal(t, "version_mismatch", body.Reason)
	require.Equal(t, q.Id, body.Current.Id)
	require.Equal(t, 1, body.Current.Version)

	good := api.UpdateQueueRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), good)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var updated api.Queue
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, 2, updated.Version)
	require.Equal(t, "Sales v2", updated.Name)
}

// TestQueues_SoftDelete — DELETE → 204; GET → 404; second DELETE → 404
// (idempotent-disabled, D-65).
func TestQueues_SoftDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := postQueue(t, th, makeQueueBody("ext-del", "Closed"))

	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp2, _ := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	resp3, _ := httpDELETE(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)))
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)

	resp4, raw4 := httpGET(t, th.HTTP, th.OrgID, queuePath(th.OrgID), nil)
	require.Equal(t, http.StatusOK, resp4.StatusCode)
	var list api.ListQueues200JSONResponse
	require.NoError(t, json.Unmarshal(raw4, &list))
	for _, it := range list.Items {
		require.NotEqual(t, q.Id, it.Id, "list default must NOT contain soft-deleted queue")
	}
}

// TestQueues_SoftDelete_IncludeDisabled — ?include_disabled=true returns
// the soft-deleted queue with enabled=false.
func TestQueues_SoftDelete_IncludeDisabled(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := postQueue(t, th, makeQueueBody("ext-del2", "Old"))
	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	qry := url.Values{"include_disabled": {"true"}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, queuePath(th.OrgID), qry)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "body=%s", string(raw2))
	var list api.ListQueues200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &list))

	found := false
	for _, it := range list.Items {
		if it.Id == q.Id {
			found = true
			require.False(t, it.Enabled, "soft-deleted queue must surface with enabled=false")
		}
	}
	require.True(t, found, "?include_disabled=true must include the soft-deleted queue")
}

// TestQueues_Cursor — 60 queues paginated 25/25/10 with no overlap.
func TestQueues_Cursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	const total = 60
	ids := make(map[uuid.UUID]struct{}, total)
	for i := 0; i < total; i++ {
		body := makeQueueBody(fmt.Sprintf("ext-%03d", i), fmt.Sprintf("Queue%03d", i))
		q := postQueue(t, th, body)
		ids[uuid.UUID(q.Id)] = struct{}{}
		// UUIDv7 monotonicity hint — keeps the cursor strictly ordered.
		time.Sleep(1 * time.Millisecond)
	}

	seen := make(map[uuid.UUID]struct{}, total)

	resp, raw := httpGET(t, th.HTTP, th.OrgID, queuePath(th.OrgID), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "page 1, body=%s", string(raw))
	var page1 api.ListQueues200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &page1))
	require.Equal(t, 25, len(page1.Items))
	require.True(t, page1.HasMore)
	require.NotNil(t, page1.NextCursor)
	for _, it := range page1.Items {
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q2 := url.Values{"cursor": {*page1.NextCursor}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, queuePath(th.OrgID), q2)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "page 2, body=%s", string(raw2))
	var page2 api.ListQueues200JSONResponse
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
	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, queuePath(th.OrgID), q3)
	require.Equalf(t, http.StatusOK, resp3.StatusCode, "page 3, body=%s", string(raw3))
	var page3 api.ListQueues200JSONResponse
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

// TestQueues_NameSearch — case-insensitive substring match.
func TestQueues_NameSearch(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	postQueue(t, th, makeQueueBody("ext-1", "Alpha"))
	postQueue(t, th, makeQueueBody("ext-2", "Alphabet"))
	postQueue(t, th, makeQueueBody("ext-3", "Beta"))

	q := url.Values{"name": {"al"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, queuePath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var list api.ListQueues200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &list))
	require.Equal(t, 2, len(list.Items), "?name=al must match 2 case-insensitive")
	for _, it := range list.Items {
		require.Containsf(t, []string{"Alpha", "Alphabet"}, it.Name,
			"unexpected name in name-search results: %s", it.Name)
	}
}

// TestQueues_CacheInvalidationOnUpdate — D-55 cache DEL after PATCH so a
// stale cached value cannot mask the update.
func TestQueues_CacheInvalidationOnUpdate(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := postQueue(t, th, makeQueueBody("ext-cache", "QC"))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "queues", uuid.UUID(q.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	newName := "QC v2"
	body := api.UpdateQueueRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), body)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))

	require.False(t, th.Miniredis.Exists(cacheKey), "cache MUST be DELed after PATCH (D-55)")

	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	var got api.Queue
	require.NoError(t, json.Unmarshal(raw3, &got))
	require.Equal(t, "QC v2", got.Name)
	require.Equal(t, 2, got.Version)
}

// TestQueues_CacheInvalidationOnDelete — DEL fires so the next GET returns
// the 404 from the soft-delete (not a cached enabled=true).
func TestQueues_CacheInvalidationOnDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := postQueue(t, th, makeQueueBody("ext-cache-del", "QD"))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "queues", uuid.UUID(q.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	resp2, _ := httpDELETE(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)))
	require.Equal(t, http.StatusNoContent, resp2.StatusCode)
	require.False(t, th.Miniredis.Exists(cacheKey))

	resp3, _ := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)
}

// TestQueues_ChannelTypesRoundTrip — Queue-specific. POST with
// channel_types=["voice","chat"]; GET preserves both values; PATCH to
// ["email"] swaps the set atomically.
func TestQueues_ChannelTypesRoundTrip(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := api.CreateQueueRequest{
		ExternalId:   "ext-ct",
		Name:         "Multi",
		ChannelTypes: []api.ChannelType{"voice", "chat"},
		Priority:     5,
		AcwSec:       15,
	}
	q := postQueue(t, th, body)
	require.ElementsMatch(t, []api.ChannelType{"voice", "chat"}, q.ChannelTypes,
		"POST response must echo channel_types as submitted")

	resp, raw := httpGET(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var got api.Queue
	require.NoError(t, json.Unmarshal(raw, &got))
	require.ElementsMatch(t, []api.ChannelType{"voice", "chat"}, got.ChannelTypes,
		"GET must preserve channel_types round-trip")

	replace := []api.ChannelType{"email"}
	patch := api.UpdateQueueRequest{Version: 1, ChannelTypes: &replace}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), patch)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))
	var updated api.Queue
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, []api.ChannelType{"email"}, updated.ChannelTypes,
		"PATCH must replace (not merge) channel_types")
}

// TestQueues_PriorityAcwSec — priority + acw_sec integers round-trip
// straight through INSERT → GET → PATCH → GET.
func TestQueues_PriorityAcwSec(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := api.CreateQueueRequest{
		ExternalId:   "ext-pri",
		Name:         "Priority",
		ChannelTypes: []api.ChannelType{"voice"},
		Priority:     7,
		AcwSec:       45,
	}
	q := postQueue(t, th, body)
	require.Equal(t, 7, q.Priority)
	require.Equal(t, 45, q.AcwSec)

	newPriority := 3
	newAcw := 60
	patch := api.UpdateQueueRequest{Version: 1, Priority: &newPriority, AcwSec: &newAcw}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, uuid.UUID(q.Id)), patch)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "PATCH, body=%s", string(raw))
	var updated api.Queue
	require.NoError(t, json.Unmarshal(raw, &updated))
	require.Equal(t, 3, updated.Priority)
	require.Equal(t, 60, updated.AcwSec)
}

// TestQueues_CrossOrgGet404 — FOUND-08 isolation canary at the queue
// handler level; the full cross-org probe lives in Plan 03-10.
func TestQueues_CrossOrgGet404(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	q := postQueue(t, th, makeQueueBody("ext-cross", "Across"))

	orgB := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, orgB, queueDetailPath(orgB, uuid.UUID(q.Id)), nil)
	require.Equalf(t, http.StatusNotFound, resp.StatusCode,
		"cross-org GET MUST return 404 (FOUND-08), body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestQueues_LimitOutOfRange — handler-side 400 invalid_body on ?limit
// boundaries. The spec doesn't declare min/max on LimitQuery so the
// handler owns the 400 wire shape (matches agents).
func TestQueues_LimitOutOfRange(t *testing.T) {
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
			resp, raw := httpGET(t, th.HTTP, th.OrgID, queuePath(th.OrgID), q)
			require.Equalf(t, http.StatusBadRequest, resp.StatusCode,
				"limit=%s must be 400, body=%s", c.limit, string(raw))
			var e api.ErrorResponse
			require.NoError(t, json.Unmarshal(raw, &e))
			require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
			require.Contains(t, e.Reason, "limit_out_of_range")
			n, parseErr := strconv.Atoi(c.limit)
			require.NoError(t, parseErr)
			require.Contains(t, e.Reason, fmt.Sprintf(":%d", n))
		})
	}
}
