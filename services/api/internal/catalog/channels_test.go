// channels_test.go — end-to-end channels handler tests (CAT-05). The
// D-76 cross-row FK probe gets its own block of tests (Plan 03-08 must-
// have) — these are the canonical proof that channels.default_queue_id
// is validated against queues in the SAME org BEFORE the INSERT/UPDATE.
//
// The cross-org test (orgB seeds a queue → orgA tries to reference it)
// is the FOUND-08 enforcement proof for the FK pattern. A successful
// 422 here means the probe produces the SAME response shape whether the
// queue is missing or simply invisible from the caller's org.
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

// channelPath / channelDetailPath — URL helpers.
func channelPath(orgID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/channels"
}

func channelDetailPath(orgID, channelID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/channels/" + channelID.String()
}

// postChannel issues a Create call and returns the parsed Channel on 201.
func postChannel(t testing.TB, th *TestHandlers, body api.CreateChannelRequest) api.Channel {
	t.Helper()
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"postChannel: want 201, got %d body=%s", resp.StatusCode, string(raw))
	var c api.Channel
	require.NoError(t, json.Unmarshal(raw, &c), "postChannel: unmarshal Channel")
	return c
}

// makeChannelBody — minimal default; default_queue_id omitted so it's
// nil unless the test overrides.
// Phase 04.1: signature gained leading `code string` arg (D04_1-03 required).
// externalID is now stored as *string (column nullable post-04.1).
func makeChannelBody(code, externalID, name, channelType string) api.CreateChannelRequest {
	body := api.CreateChannelRequest{
		Code:        code,
		Name:        name,
		ChannelType: api.ChannelType(channelType),
	}
	if externalID != "" {
		body.ExternalId = strPtr(externalID)
	}
	return body
}

// TestChannels_CreateThenGet — happy path + cache fill on first GET.
func TestChannels_CreateThenGet(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeChannelBody("ch_001", "ext-001", "Voice", "voice")
	c := postChannel(t, th, body)
	require.Equal(t, 1, c.Version, "fresh channel version must be 1")
	require.True(t, c.Enabled)
	require.Equal(t, "Voice", c.Name)
	require.Equal(t, api.ChannelType("voice"), c.ChannelType)
	require.Nil(t, c.DefaultQueueId, "default_queue_id omitted in body → nil in response")
	require.Equal(t, uuid.Version(7), uuid.UUID(c.Id).Version())

	resp, raw := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "first GET, body=%s", string(raw))
	var got api.Channel
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, c.Id, got.Id)
	require.Nil(t, got.DefaultQueueId)

	cacheKey := cache.Key(th.OrgID, "channels", uuid.UUID(c.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET (D-49)")

	resp2, _ := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	require.True(t, th.Miniredis.Exists(cacheKey))
}

// TestChannels_GetMissing — unknown id → 404.
func TestChannels_GetMissing(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	unknown := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, unknown), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e), "body=%s", string(raw))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestChannels_VersionConflict — stale version → 409 with current.
func TestChannels_VersionConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_vc", "ext-vc", "Chat", "chat"))
	newName := "Chat v2"

	bad := api.UpdateChannelRequest{Version: 99, Name: &newName}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), bad)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))

	var body struct {
		Current api.Channel `json:"current"`
		Error   string      `json:"error"`
		Reason  string      `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "version_conflict", body.Error)
	require.Equal(t, "version_mismatch", body.Reason)
	require.Equal(t, c.Id, body.Current.Id)
	require.Equal(t, 1, body.Current.Version)

	good := api.UpdateChannelRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), good)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var updated api.Channel
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, 2, updated.Version)
	require.Equal(t, "Chat v2", updated.Name)
}

// TestChannels_SoftDelete — DELETE → 204; GET → 404; second DELETE → 404.
func TestChannels_SoftDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_del", "ext-del", "Email", "email"))

	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp2, _ := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	resp3, _ := httpDELETE(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)))
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)

	resp4, raw4 := httpGET(t, th.HTTP, th.OrgID, channelPath(th.OrgID), nil)
	require.Equal(t, http.StatusOK, resp4.StatusCode)
	var list api.ListChannels200JSONResponse
	require.NoError(t, json.Unmarshal(raw4, &list))
	for _, it := range list.Items {
		require.NotEqual(t, c.Id, it.Id, "list default must NOT contain soft-deleted channel")
	}
}

// TestChannels_SoftDelete_IncludeDisabled — ?include_disabled returns
// the soft-deleted channel with enabled=false.
func TestChannels_SoftDelete_IncludeDisabled(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_del2", "ext-del2", "Gone", "voice"))
	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	q := url.Values{"include_disabled": {"true"}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, channelPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "body=%s", string(raw2))
	var list api.ListChannels200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &list))

	found := false
	for _, it := range list.Items {
		if it.Id == c.Id {
			found = true
			require.False(t, it.Enabled)
		}
	}
	require.True(t, found, "?include_disabled=true must include the soft-deleted channel")
}

// TestChannels_Cursor — 60 channels paginated 25/25/10 with no overlap.
func TestChannels_Cursor(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	const total = 60
	ids := make(map[uuid.UUID]struct{}, total)
	for i := 0; i < total; i++ {
		body := makeChannelBody(fmt.Sprintf("ch_%03d", i), fmt.Sprintf("ext-%03d", i), fmt.Sprintf("Channel%03d", i), "voice")
		c := postChannel(t, th, body)
		ids[uuid.UUID(c.Id)] = struct{}{}
		time.Sleep(1 * time.Millisecond)
	}

	seen := make(map[uuid.UUID]struct{}, total)

	resp, raw := httpGET(t, th.HTTP, th.OrgID, channelPath(th.OrgID), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "page 1, body=%s", string(raw))
	var page1 api.ListChannels200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &page1))
	require.Equal(t, 25, len(page1.Items))
	require.True(t, page1.HasMore)
	require.NotNil(t, page1.NextCursor)
	for _, it := range page1.Items {
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q2 := url.Values{"cursor": {*page1.NextCursor}}
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, channelPath(th.OrgID), q2)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "page 2, body=%s", string(raw2))
	var page2 api.ListChannels200JSONResponse
	require.NoError(t, json.Unmarshal(raw2, &page2))
	require.Equal(t, 25, len(page2.Items))
	for _, it := range page2.Items {
		_, dup := seen[uuid.UUID(it.Id)]
		require.False(t, dup)
		seen[uuid.UUID(it.Id)] = struct{}{}
	}

	q3 := url.Values{"cursor": {*page2.NextCursor}}
	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, channelPath(th.OrgID), q3)
	require.Equalf(t, http.StatusOK, resp3.StatusCode, "page 3, body=%s", string(raw3))
	var page3 api.ListChannels200JSONResponse
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

// TestChannels_NameSearch — case-insensitive substring match.
func TestChannels_NameSearch(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	postChannel(t, th, makeChannelBody("ch_1", "ext-1", "Alpha", "voice"))
	postChannel(t, th, makeChannelBody("ch_2", "ext-2", "Alphabet", "chat"))
	postChannel(t, th, makeChannelBody("ch_3", "ext-3", "Beta", "email"))

	q := url.Values{"name": {"al"}}
	resp, raw := httpGET(t, th.HTTP, th.OrgID, channelPath(th.OrgID), q)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var list api.ListChannels200JSONResponse
	require.NoError(t, json.Unmarshal(raw, &list))
	require.Equal(t, 2, len(list.Items))
	for _, it := range list.Items {
		require.Containsf(t, []string{"Alpha", "Alphabet"}, it.Name,
			"unexpected name: %s", it.Name)
	}
}

// TestChannels_CacheInvalidationOnUpdate — D-55 DEL after PATCH.
func TestChannels_CacheInvalidationOnUpdate(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_cache", "ext-cache", "CC", "voice"))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "channels", uuid.UUID(c.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	newName := "CC v2"
	body := api.UpdateChannelRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), body)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))

	require.False(t, th.Miniredis.Exists(cacheKey), "D-55 cache DEL")

	resp3, raw3 := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	var got api.Channel
	require.NoError(t, json.Unmarshal(raw3, &got))
	require.Equal(t, "CC v2", got.Name)
	require.Equal(t, 2, got.Version)
}

// TestChannels_CacheInvalidationOnDelete — DEL fires so 404 isn't masked.
func TestChannels_CacheInvalidationOnDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_cd", "ext-cd", "CD", "voice"))
	resp, _ := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	cacheKey := cache.Key(th.OrgID, "channels", uuid.UUID(c.Id))
	require.True(t, th.Miniredis.Exists(cacheKey))

	resp2, _ := httpDELETE(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)))
	require.Equal(t, http.StatusNoContent, resp2.StatusCode)
	require.False(t, th.Miniredis.Exists(cacheKey))

	resp3, _ := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)
}

// TestChannels_DefaultQueueId_HappyPath — POST queue in orgA + POST
// channel referencing that queue → 201 with default_queue_id set;
// GET preserves the link.
func TestChannels_DefaultQueueId_HappyPath(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	queueID := seedQueueForOrg(t, th, ctx, th.OrgID, "primary")
	qUUID := api.UUIDv7(queueID)
	body := api.CreateChannelRequest{
		Code:           "ch_link",
		ExternalId:     strPtr("ext-link"),
		Name:           "Linked",
		ChannelType:    "voice",
		DefaultQueueId: &qUUID,
	}
	c := postChannel(t, th, body)
	require.NotNil(t, c.DefaultQueueId, "default_queue_id must round-trip")
	require.Equal(t, queueID, uuid.UUID(*c.DefaultQueueId))

	resp, raw := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var got api.Channel
	require.NoError(t, json.Unmarshal(raw, &got))
	require.NotNil(t, got.DefaultQueueId)
	require.Equal(t, queueID, uuid.UUID(*got.DefaultQueueId))
}

// TestChannels_DefaultQueueId_NotFound — random UUID → 422
// invalid_reference. The handler probe runs BEFORE the INSERT (D-76).
func TestChannels_DefaultQueueId_NotFound(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	bogusUUID := api.UUIDv7(uuid.Must(uuid.NewV7()))
	body := api.CreateChannelRequest{
		Code:           "ch_bogus",
		ExternalId:     strPtr("ext-bogus"),
		Name:           "Bogus",
		ChannelType:    "voice",
		DefaultQueueId: &bogusUUID,
	}
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"want 422 invalid_reference, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
	require.Equal(t, "default_queue_id_not_found_or_disabled", e.Reason)
}

// TestChannels_Create_UnknownQueueId_422 — plan-required alias for
// the not-found 422 case. Same scenario, different name to satisfy the
// must-have grep gate.
func TestChannels_Create_UnknownQueueId_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	bogusUUID := api.UUIDv7(uuid.Must(uuid.NewV7()))
	body := api.CreateChannelRequest{
		Code:           "ch_unk",
		ExternalId:     strPtr("ext-unk"),
		Name:           "Unknown",
		ChannelType:    "chat",
		DefaultQueueId: &bogusUUID,
	}
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
}

// TestChannels_DefaultQueueId_Disabled — POST queue, soft-delete the
// queue, POST channel pointing at it → 422. The probe filters on
// enabled=TRUE so the disabled queue is invisible to the probe.
func TestChannels_DefaultQueueId_Disabled(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	queueID := seedQueueForOrg(t, th, ctx, th.OrgID, "willdelete")
	// Soft-delete the queue via the queues handler so the test goes
	// through the production code path (CAT-09 idempotent-disabled).
	respDel, _ := httpDELETE(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, queueID))
	require.Equal(t, http.StatusNoContent, respDel.StatusCode)

	qUUID := api.UUIDv7(queueID)
	body := api.CreateChannelRequest{
		Code:           "ch_dis",
		ExternalId:     strPtr("ext-dis"),
		Name:           "PointingDisabled",
		ChannelType:    "voice",
		DefaultQueueId: &qUUID,
	}
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"disabled queue must surface as 422, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
	require.Equal(t, "default_queue_id_not_found_or_disabled", e.Reason)
}

// TestChannels_DefaultQueueId_CrossOrg — POST queue in orgB; POST
// channel in orgA referencing that queue → 422 (FOUND-08 — orgA cannot
// see orgB's rows; the probe ErrNoRows surfaces as invalid_reference).
func TestChannels_DefaultQueueId_CrossOrg(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	orgB := uuid.Must(uuid.NewV7())
	queueB := seedQueueForOrg(t, th, ctx, orgB, "in-orgB")

	qUUID := api.UUIDv7(queueB)
	body := api.CreateChannelRequest{
		Code:           "ch_cross",
		ExternalId:     strPtr("ext-cross"),
		Name:           "CrossOrg",
		ChannelType:    "voice",
		DefaultQueueId: &qUUID,
	}
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"cross-org queue MUST surface as 422 (FOUND-08), body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
	require.Equal(t, "default_queue_id_not_found_or_disabled", e.Reason,
		"response shape MUST be identical to the not-found case — no existence oracle")
}

// TestChannels_CrossOrgQueueReject_422 — plan-required alias for the
// cross-org probe test.
func TestChannels_CrossOrgQueueReject_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	orgB := uuid.Must(uuid.NewV7())
	queueB := seedQueueForOrg(t, th, ctx, orgB, "alias-orgB")

	qUUID := api.UUIDv7(queueB)
	body := api.CreateChannelRequest{
		Code:           "ch_alias",
		ExternalId:     strPtr("ext-alias"),
		Name:           "AliasCross",
		ChannelType:    "voice",
		DefaultQueueId: &qUUID,
	}
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
}

// TestChannels_UpdateChannelInvalidReference — PATCH a channel with a
// random default_queue_id → 422. The channel itself is unchanged.
func TestChannels_UpdateChannelInvalidReference(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_upd", "ext-upd", "Plain", "voice"))
	bogus := api.UUIDv7(uuid.Must(uuid.NewV7()))
	body := api.UpdateChannelRequest{Version: 1, DefaultQueueId: &bogus}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"want 422, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
	require.Equal(t, "default_queue_id_not_found_or_disabled", e.Reason)

	// Channel must be untouched — version still 1, default still nil.
	resp2, raw2 := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var after api.Channel
	require.NoError(t, json.Unmarshal(raw2, &after))
	require.Equal(t, 1, after.Version, "rejected PATCH must NOT bump version")
	require.Nil(t, after.DefaultQueueId)
}

// TestChannels_Update_QueueDisabledInSameOrg_422 — PATCH a channel
// pointing at a queue that was soft-deleted in the SAME org → 422.
// Confirms the UpdateChannel probe runs on the same enabled=TRUE filter
// as CreateChannel.
func TestChannels_Update_QueueDisabledInSameOrg_422(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_upd_dis", "ext-upd-dis", "Updish", "voice"))
	queueID := seedQueueForOrg(t, th, ctx, th.OrgID, "to-soft-delete")
	respDel, _ := httpDELETE(t, th.HTTP, th.OrgID, queueDetailPath(th.OrgID, queueID))
	require.Equal(t, http.StatusNoContent, respDel.StatusCode)

	qUUID := api.UUIDv7(queueID)
	body := api.UpdateChannelRequest{Version: 1, DefaultQueueId: &qUUID}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"PATCH to disabled queue must be 422, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
	require.Equal(t, "default_queue_id_not_found_or_disabled", e.Reason)
}

// TestChannels_CrossOrgGet404 — FOUND-08 inline canary at the channel
// handler level.
func TestChannels_CrossOrgGet404(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_cog", "ext-cog", "InOrgA", "voice"))

	orgB := uuid.Must(uuid.NewV7())
	resp, raw := httpGET(t, th.HTTP, orgB, channelDetailPath(orgB, uuid.UUID(c.Id)), nil)
	require.Equalf(t, http.StatusNotFound, resp.StatusCode,
		"cross-org GET MUST return 404 (FOUND-08), body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeNotFound, e.Error)
}

// TestChannels_ChannelTypeRoundTrip — channel_type "voice" round-trips
// through POST → GET → PATCH "chat" → GET.
func TestChannels_ChannelTypeRoundTrip(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	c := postChannel(t, th, makeChannelBody("ch_ct", "ext-ct", "Rotates", "voice"))
	require.Equal(t, api.ChannelType("voice"), c.ChannelType)

	resp, raw := httpGET(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var got api.Channel
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, api.ChannelType("voice"), got.ChannelType)

	newType := api.ChannelType("chat")
	patch := api.UpdateChannelRequest{Version: 1, ChannelType: &newType}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(c.Id)), patch)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "PATCH, body=%s", string(raw2))
	var updated api.Channel
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, api.ChannelType("chat"), updated.ChannelType)
}

// TestChannels_LimitOutOfRange — handler-side 400 invalid_body on bad
// ?limit boundaries.
func TestChannels_LimitOutOfRange(t *testing.T) {
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
			resp, raw := httpGET(t, th.HTTP, th.OrgID, channelPath(th.OrgID), q)
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

// TestChannels_InvalidReference — plan-required must_have alias for the
// CreateChannel 422 path (`contains: "TestChannels_InvalidReference"`).
func TestChannels_InvalidReference(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	bogusUUID := api.UUIDv7(uuid.Must(uuid.NewV7()))
	body := api.CreateChannelRequest{
		Code:           "ch_ir",
		ExternalId:     strPtr("ext-ir"),
		Name:           "AliasIR",
		ChannelType:    "voice",
		DefaultQueueId: &bogusUUID,
	}
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidReference, e.Error)
}

// ---------------------------------------------------------------------------
// Phase 04.1 — 7 standard tests (D04_1-23 + VALIDATION IDENT-02).
// ---------------------------------------------------------------------------

// TestChannels_MissingCode_Returns400 — Layer 1 rejects empty code.
// Channels.go has Layer 1 BEFORE the D-76 default_queue_id FK probe, so
// the regex fires regardless of any FK column state.
func TestChannels_MissingCode_Returns400(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeChannelBody("", "", "Voice", "voice") // empty code
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body)
	require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
	require.Equal(t, "invalid_code_format", e.Reason)
}

// TestChannels_DuplicateCode_Returns409_DuplicateCode — composite UNIQUE on
// (org_id, code) fires; MapPgError returns duplicate_code.
func TestChannels_DuplicateCode_Returns409_DuplicateCode(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeChannelBody("channel_voice", "ext-ch-001", "Voice", "voice")
	_ = postChannel(t, th, body)

	body2 := makeChannelBody("channel_voice", "ext-ch-002", "Voice2", "voice")
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body2)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeDuplicateCode, e.Error)
	require.Equal(t, "duplicate_code", e.Reason)
}

// TestChannels_DuplicateExternalId_Returns409_DuplicateExternalId — partial
// UNIQUE on (org_id, external_id) fires; MapPgError returns duplicate_external_id.
func TestChannels_DuplicateExternalId_Returns409_DuplicateExternalId(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeChannelBody("channel_dx_001", "ext-ch-001", "First", "voice")
	_ = postChannel(t, th, body)

	body2 := makeChannelBody("channel_dx_002", "ext-ch-001", "Second", "chat")
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body2)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeDuplicateExternalId, e.Error)
	require.Equal(t, "duplicate_external_id", e.Reason)
}

// TestChannels_PatchSameCode_Returns200 — PATCH with same code is a no-op.
func TestChannels_PatchSameCode_Returns200(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeChannelBody("channel_same_001", "ext-ch-same", "Plain", "voice")
	created := postChannel(t, th, body)

	sameCode := "channel_same_001"
	newName := "Plain v2"
	patchBody := api.UpdateChannelRequest{
		Version: created.Version,
		Code:    &sameCode,
		Name:    &newName,
	}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(created.Id)), patchBody)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "body=%s", string(raw))
	var updated api.Channel
	require.NoError(t, json.Unmarshal(raw, &updated))
	require.Equal(t, sameCode, updated.Code)
	require.Equal(t, newName, updated.Name)
}

// TestChannels_PatchDifferentCode_Returns422_ImmutableField — Layer 2 rejects.
func TestChannels_PatchDifferentCode_Returns422_ImmutableField(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeChannelBody("channel_diff_001", "ext-ch-diff", "Email1", "email")
	created := postChannel(t, th, body)

	newCode := "channel_diff_002"
	patchBody := api.UpdateChannelRequest{
		Version: created.Version,
		Code:    &newCode,
	}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(created.Id)), patchBody)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeImmutableField, e.Error)
	require.Equal(t, "code", e.Reason)
}

// TestChannels_PatchInvalidCodeFormat_Returns400 — Layer 1 fires first.
func TestChannels_PatchInvalidCodeFormat_Returns400(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeChannelBody("channel_bad_001", "ext-ch-bad", "Bad", "voice")
	created := postChannel(t, th, body)

	badCode := "has spaces" // space → fails regex
	patchBody := api.UpdateChannelRequest{
		Version: created.Version,
		Code:    &badCode,
	}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, channelDetailPath(th.OrgID, uuid.UUID(created.Id)), patchBody)
	require.Equalf(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeInvalidBody, e.Error)
	require.Equal(t, "invalid_code_format", e.Reason)
}

// TestChannels_TwoNullExternalIds_NoConflict — IDENT-02.
func TestChannels_TwoNullExternalIds_NoConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	body := makeChannelBody("channel_null_001", "", "First", "voice")
	_ = postChannel(t, th, body)

	body2 := makeChannelBody("channel_null_002", "", "Second", "chat")
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, channelPath(th.OrgID), body2)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"body=%s — two NULL external_ids must coexist", string(raw))
}
