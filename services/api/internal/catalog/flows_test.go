// flows_test.go — end-to-end flow draft handler tests (FLOW, v0.2 Stage 1).
// Mirrors queues_test.go; flow-specific cases cover the graph JSONB round-trip
// and code immutability.
package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
)

func flowPath(orgID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/flows"
}

func flowDetailPath(orgID, flowID uuid.UUID) string {
	return "/v1/orgs/" + orgID.String() + "/flows/" + flowID.String()
}

func postFlow(t testing.TB, th *TestHandlers, body api.CreateFlowRequest) api.Flow {
	t.Helper()
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, flowPath(th.OrgID), body)
	require.Equalf(t, http.StatusCreated, resp.StatusCode,
		"postFlow: want 201, got %d body=%s", resp.StatusCode, string(raw))
	var f api.Flow
	require.NoError(t, json.Unmarshal(raw, &f), "postFlow: unmarshal Flow")
	return f
}

func makeFlowBody(code, name string) api.CreateFlowRequest {
	return api.CreateFlowRequest{Code: code, Name: name}
}

// TestFlows_CreateThenGet — POST → 201 Flow (version=1, enabled, UUIDv7 id,
// empty graph default); GET caches on first read.
func TestFlows_CreateThenGet(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	f := postFlow(t, th, makeFlowBody("flow_inbound", "Inbound Voice"))
	require.Equal(t, 1, f.Version, "fresh flow version must be 1")
	require.True(t, f.Enabled)
	require.Equal(t, "Inbound Voice", f.Name)
	require.Equal(t, uuid.Version(7), uuid.UUID(f.Id).Version())
	require.Equal(t, th.OrgID, uuid.UUID(f.OrgId))
	require.NotNil(t, f.Graph, "omitted graph must default to an empty object, not null")

	resp, raw := httpGET(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)), nil)
	require.Equalf(t, http.StatusOK, resp.StatusCode, "first GET, body=%s", string(raw))
	var got api.Flow
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, f.Id, got.Id)

	cacheKey := cache.Key(th.OrgID, "flows", uuid.UUID(f.Id))
	require.True(t, th.Miniredis.Exists(cacheKey), "cache key must exist after first GET (D-49)")
}

// TestFlows_GraphRoundTrip — a non-trivial graph survives create + GET
// unchanged, and PATCH replaces it.
func TestFlows_GraphRoundTrip(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	graph := map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"id": "trigger_1", "type": "trigger"},
			map[string]interface{}{"id": "end_1", "type": "end"},
		},
		"edges": []interface{}{
			map[string]interface{}{"from": "trigger_1", "to": "end_1"},
		},
	}
	body := api.CreateFlowRequest{Code: "flow_graph", Name: "Graph", Graph: &graph}
	f := postFlow(t, th, body)

	wantJSON, _ := json.Marshal(graph)
	gotJSON, _ := json.Marshal(f.Graph)
	require.JSONEq(t, string(wantJSON), string(gotJSON), "graph must round-trip through create")

	// GET returns the same graph (from cache miss → DB load).
	resp, raw := httpGET(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got api.Flow
	require.NoError(t, json.Unmarshal(raw, &got))
	gotJSON2, _ := json.Marshal(got.Graph)
	require.JSONEq(t, string(wantJSON), string(gotJSON2), "graph must round-trip through GET")

	// PATCH replaces the graph.
	newGraph := map[string]interface{}{"nodes": []interface{}{}, "edges": []interface{}{}}
	patch := api.UpdateFlowRequest{Version: 1, Graph: &newGraph}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)), patch)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var updated api.Flow
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, 2, updated.Version)
	newJSON, _ := json.Marshal(newGraph)
	updJSON, _ := json.Marshal(updated.Graph)
	require.JSONEq(t, string(newJSON), string(updJSON), "PATCH must replace the graph")
}

// TestFlows_VersionConflict — stale version → 409 + current; right version → 200.
func TestFlows_VersionConflict(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	f := postFlow(t, th, makeFlowBody("flow_vc", "VC"))
	newName := "VC v2"

	bad := api.UpdateFlowRequest{Version: 99, Name: &newName}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)), bad)
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))

	var conflict struct {
		Current api.Flow `json:"current"`
		Error   string   `json:"error"`
		Reason  string   `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(raw, &conflict))
	require.Equal(t, "version_conflict", conflict.Error)
	require.Equal(t, "version_mismatch", conflict.Reason)
	require.Equal(t, f.Id, conflict.Current.Id)
	require.Equal(t, 1, conflict.Current.Version)

	good := api.UpdateFlowRequest{Version: 1, Name: &newName}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)), good)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "want 200, body=%s", string(raw2))
	var updated api.Flow
	require.NoError(t, json.Unmarshal(raw2, &updated))
	require.Equal(t, 2, updated.Version)
	require.Equal(t, "VC v2", updated.Name)
}

// TestFlows_CodeImmutable — PATCH that changes `code` → 422 immutable_field.
func TestFlows_CodeImmutable(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	f := postFlow(t, th, makeFlowBody("flow_imm", "Immutable"))
	other := "flow_renamed"
	patch := api.UpdateFlowRequest{Version: 1, Code: &other}
	resp, raw := httpPATCH(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)), patch)
	require.Equalf(t, http.StatusUnprocessableEntity, resp.StatusCode, "want 422, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeImmutableField, e.Error)

	// Same code is a no-op, not a 422.
	same := api.UpdateFlowRequest{Version: 1, Code: &f.Code, Name: strPtr("Renamed label")}
	resp2, raw2 := httpPATCH(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)), same)
	require.Equalf(t, http.StatusOK, resp2.StatusCode, "same code must succeed, body=%s", string(raw2))
}

// TestFlows_DuplicateCode — second create with the same code in the same org → 409.
func TestFlows_DuplicateCode(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	_ = postFlow(t, th, makeFlowBody("flow_dup", "First"))
	resp, raw := httpPOST(t, th.HTTP, th.OrgID, flowPath(th.OrgID), makeFlowBody("flow_dup", "Second"))
	require.Equalf(t, http.StatusConflict, resp.StatusCode, "want 409, body=%s", string(raw))
	var e api.ErrorResponse
	require.NoError(t, json.Unmarshal(raw, &e))
	require.Equal(t, api.ErrorCodeDuplicateCode, e.Error)
}

// TestFlows_SoftDelete — DELETE → 204; GET → 404; second DELETE → 404; list excludes.
func TestFlows_SoftDelete(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	f := postFlow(t, th, makeFlowBody("flow_del", "Closed"))

	resp, _ := httpDELETE(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)))
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp2, _ := httpGET(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)), nil)
	require.Equal(t, http.StatusNotFound, resp2.StatusCode)

	resp3, _ := httpDELETE(t, th.HTTP, th.OrgID, flowDetailPath(th.OrgID, uuid.UUID(f.Id)))
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)

	resp4, raw4 := httpGET(t, th.HTTP, th.OrgID, flowPath(th.OrgID), nil)
	require.Equal(t, http.StatusOK, resp4.StatusCode)
	var list api.ListFlows200JSONResponse
	require.NoError(t, json.Unmarshal(raw4, &list))
	for _, it := range list.Items {
		require.NotEqual(t, f.Id, it.Id, "list default must NOT contain soft-deleted flow")
	}
}

// TestFlows_CrossOrgIsolation — org B cannot GET or list org A's flow (FOUND-08).
func TestFlows_CrossOrgIsolation(t *testing.T) {
	th := newTestHandlers(t)
	ctx := context.Background()
	cleanCatalogTables(t, ctx)

	f := postFlow(t, th, makeFlowBody("flow_iso", "Org A only"))
	orgB := uuid.Must(uuid.NewV7())

	// Cross-org GET → 404 (not 200, not 403 — the row is invisible).
	resp, raw := httpGET(t, th.HTTP, orgB, flowDetailPath(orgB, uuid.UUID(f.Id)), nil)
	require.Equalf(t, http.StatusNotFound, resp.StatusCode, "cross-org GET must 404, body=%s", string(raw))

	// Org B's list is empty; org A still sees its flow.
	_, rawB := httpGET(t, th.HTTP, orgB, flowPath(orgB), nil)
	var listB api.ListFlows200JSONResponse
	require.NoError(t, json.Unmarshal(rawB, &listB))
	require.Empty(t, listB.Items, "org B must not see org A's flows")

	_, rawA := httpGET(t, th.HTTP, th.OrgID, flowPath(th.OrgID), nil)
	var listA api.ListFlows200JSONResponse
	require.NoError(t, json.Unmarshal(rawA, &listA))
	require.Len(t, listA.Items, 1)
	require.Equal(t, f.Id, listA.Items[0].Id)
}
