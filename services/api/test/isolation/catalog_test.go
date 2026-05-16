// catalog_test.go ships the per-entity FOUND-08 cross-org probe suite.
// Every catalog entity (CAT-01..CAT-06) gets its own canonical test:
// create in orgA, GET from orgA succeeds, GET from orgB returns 404 —
// orgB MUST NOT be able to observe orgA's rows (FOUND-08).
//
// Two FK cross-org probes cover D-76 (channels.default_queue_id and
// agent_skills.skill_id): the create body in orgA references a row owned
// by orgB; the handler probe returns 422 invalid_reference. Both shapes
// (single-entity 404 + FK 422) are FOUND-08-compliant — clients cannot
// distinguish "row in another org" from "row does not exist".
//
// One list-isolation probe asserts that List in orgA returns only orgA
// rows (the canonical FOUND-08 happy path from VALIDATION.md).
package isolation_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// postEntity sends POST /v1/orgs/{orgID}/{entityPath} with the supplied
// body and X-Org-Id header. Returns the status code and (on 201) the
// minted id. On non-201 responses the test continues — the caller can
// assert the code shape, and id is zero.
//
// X-Org-Id is set from the orgID arg (NOT a struct field) so individual
// tests can hit the same httptest server with a different org for
// cross-org isolation probes (FOUND-08).
func postEntity(t *testing.T, urlBase, entityPath string, orgID uuid.UUID, body any) (int, uuid.UUID) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v1/orgs/%s/%s", urlBase, orgID, entityPath),
		bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var out struct {
		Id string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	var id uuid.UUID
	if out.Id != "" {
		id = uuid.MustParse(out.Id)
	}
	return resp.StatusCode, id
}

// getEntityStatus returns the status code from
// GET {urlBase}/v1/orgs/{orgID}/{entityPath}/{id}. Used by the canonical
// cross-org probe pattern: GET in orgA = 200, GET in orgB = 404.
func getEntityStatus(t *testing.T, urlBase, entityPath string, orgID, id uuid.UUID) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/v1/orgs/%s/%s/%s", urlBase, orgID, entityPath, id),
		nil)
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// listEntityIDs returns the array of `id` fields parsed from a List
// response. Used by the list-isolation probe to assert orgA's list
// contains exactly the orgA rows.
func listEntityIDs(t *testing.T, urlBase, entityPath string, orgID uuid.UUID) []uuid.UUID {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/v1/orgs/%s/%s", urlBase, orgID, entityPath), nil)
	require.NoError(t, err)
	req.Header.Set("X-Org-Id", orgID.String())
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "list must return 200 for entity %q in org %s", entityPath, orgID)
	var page struct {
		Items []struct {
			ID    uuid.UUID `json:"id"`
			OrgID uuid.UUID `json:"org_id"`
		} `json:"items"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	ids := make([]uuid.UUID, 0, len(page.Items))
	for _, item := range page.Items {
		require.Equal(t, orgID, item.OrgID,
			"FOUND-08: list in org %s returned a row from org %s — leak", orgID, item.OrgID)
		ids = append(ids, item.ID)
	}
	return ids
}

// TestCatalog_AgentsCrossOrg — CAT-01. Create in orgA; orgA GET 200;
// orgB GET 404. The orgDB SQLChecker enforces the org_id filter at the
// query layer; the handler maps pgx.ErrNoRows → 404.
func TestCatalog_AgentsCrossOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	code, idA := postEntity(t, baseURL(), "agents", orgA, map[string]any{
		"external_id": "ext-iso-a",
		"name":        "Alice",
		"email":       "alice@example.com",
	})
	require.Equal(t, http.StatusCreated, code, "create in orgA must succeed")
	require.Equal(t, http.StatusOK, getEntityStatus(t, baseURL(), "agents", orgA, idA))
	require.Equal(t, http.StatusNotFound, getEntityStatus(t, baseURL(), "agents", orgB, idA),
		"orgB MUST get 404 on orgA's agent (FOUND-08)")
}

// TestCatalog_SkillsCrossOrg — CAT-02.
func TestCatalog_SkillsCrossOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	code, idA := postEntity(t, baseURL(), "skills", orgA, map[string]any{
		"external_id": "ext-iso-s",
		"name":        "Vietnamese",
		"skill_type":  "language",
	})
	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, http.StatusOK, getEntityStatus(t, baseURL(), "skills", orgA, idA))
	require.Equal(t, http.StatusNotFound, getEntityStatus(t, baseURL(), "skills", orgB, idA))
}

// TestCatalog_QueuesCrossOrg — CAT-03.
func TestCatalog_QueuesCrossOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	code, idA := postEntity(t, baseURL(), "queues", orgA, map[string]any{
		"external_id":   "ext-iso-q",
		"name":          "VIP",
		"channel_types": []string{"voice"},
		"priority":      1,
		"acw_sec":       10,
	})
	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, http.StatusOK, getEntityStatus(t, baseURL(), "queues", orgA, idA))
	require.Equal(t, http.StatusNotFound, getEntityStatus(t, baseURL(), "queues", orgB, idA))
}

// TestCatalog_ChannelsCrossOrg — CAT-04.
func TestCatalog_ChannelsCrossOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	code, idA := postEntity(t, baseURL(), "channels", orgA, map[string]any{
		"external_id":  "ext-iso-c",
		"name":         "Inbound Voice",
		"channel_type": "voice",
	})
	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, http.StatusOK, getEntityStatus(t, baseURL(), "channels", orgA, idA))
	require.Equal(t, http.StatusNotFound, getEntityStatus(t, baseURL(), "channels", orgB, idA))
}

// TestCatalog_AdaptersCrossOrg — CAT-05.
func TestCatalog_AdaptersCrossOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	code, idA := postEntity(t, baseURL(), "adapters", orgA, map[string]any{
		"name":         "FreeSWITCH Edge",
		"adapter_type": "freeswitch",
		"config":       map[string]any{"sid": "AC123"},
	})
	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, http.StatusOK, getEntityStatus(t, baseURL(), "adapters", orgA, idA))
	require.Equal(t, http.StatusNotFound, getEntityStatus(t, baseURL(), "adapters", orgB, idA))
}

// TestCatalog_BreakReasonsCrossOrg — CAT-06. URL uses kebab-case
// `break-reasons` per the spec; the underlying cache slug stays
// underscore (`break_reasons` per D-58).
func TestCatalog_BreakReasonsCrossOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	code, idA := postEntity(t, baseURL(), "break-reasons", orgA, map[string]any{
		"name":          "Lunch",
		"routable":      false,
		"display_order": 10,
	})
	require.Equal(t, http.StatusCreated, code)
	require.Equal(t, http.StatusOK, getEntityStatus(t, baseURL(), "break-reasons", orgA, idA))
	require.Equal(t, http.StatusNotFound, getEntityStatus(t, baseURL(), "break-reasons", orgB, idA))
}

// TestCatalog_ChannelsDefaultQueueId_CrossOrg — D-76 FK probe. Channel
// in orgA references a queue that lives in orgB — the probe MUST return
// 422 invalid_reference, NOT 201. FOUND-08 holds because orgDB-scoped
// queries cannot see orgB's queue when the caller's org context is orgA.
func TestCatalog_ChannelsDefaultQueueId_CrossOrg(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	qCode, qB := postEntity(t, baseURL(), "queues", orgB, map[string]any{
		"external_id":   "ext-iso-cqB",
		"name":          "OtherOrgQueue",
		"channel_types": []string{"voice"},
		"priority":      1,
		"acw_sec":       10,
	})
	require.Equal(t, http.StatusCreated, qCode, "queue must be created in orgB")

	cCode, _ := postEntity(t, baseURL(), "channels", orgA, map[string]any{
		"external_id":      "ext-iso-cx",
		"name":             "InboundVoice",
		"channel_type":     "voice",
		"default_queue_id": qB.String(),
	})
	require.Equal(t, http.StatusUnprocessableEntity, cCode,
		"channel in orgA referencing queue in orgB MUST return 422 (D-76)")
}

// TestCatalog_AgentSkills_CrossOrgSkillId — D-76 FK probe for the
// agent_skills join. Agent in orgA references a skill that lives in
// orgB; the create handler's SkillsPresentInOrg probe returns ErrNoRows
// and the handler maps it to 422 invalid_reference.
func TestCatalog_AgentSkills_CrossOrgSkillId(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)
	sCode, skillB := postEntity(t, baseURL(), "skills", orgB, map[string]any{
		"external_id": "ext-iso-sx",
		"name":        "OtherOrgSkill",
		"skill_type":  "language",
	})
	require.Equal(t, http.StatusCreated, sCode)

	aCode, _ := postEntity(t, baseURL(), "agents", orgA, map[string]any{
		"external_id": "ext-iso-ax",
		"name":        "AliceX",
		"email":       "alicex@example.com",
		"skills": []map[string]any{
			{"skill_id": skillB.String(), "proficiency": 5},
		},
	})
	require.Equal(t, http.StatusUnprocessableEntity, aCode,
		"agent in orgA referencing skill in orgB MUST return 422 (D-76)")
}

// TestCatalog_AgentsListIsolation — FOUND-08 happy path. orgA seeds 3
// agents, orgB seeds 2; List in orgA must return exactly the 3 orgA ids
// (no leak); List in orgB must return exactly the 2 orgB ids. The
// listEntityIDs helper asserts every returned row carries the caller's
// org_id, so a SQL-layer leak would fail the per-item check too.
func TestCatalog_AgentsListIsolation(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	orgA, orgB := freshOrg(t), freshOrg(t)

	idsA := make(map[uuid.UUID]struct{})
	for i := 1; i <= 3; i++ {
		code, id := postEntity(t, baseURL(), "agents", orgA, map[string]any{
			"external_id": fmt.Sprintf("ext-A-%d", i),
			"name":        fmt.Sprintf("A-%d", i),
			"email":       fmt.Sprintf("a%d@example.com", i),
		})
		require.Equal(t, http.StatusCreated, code)
		idsA[id] = struct{}{}
	}
	idsB := make(map[uuid.UUID]struct{})
	for i := 1; i <= 2; i++ {
		code, id := postEntity(t, baseURL(), "agents", orgB, map[string]any{
			"external_id": fmt.Sprintf("ext-B-%d", i),
			"name":        fmt.Sprintf("B-%d", i),
			"email":       fmt.Sprintf("b%d@example.com", i),
		})
		require.Equal(t, http.StatusCreated, code)
		idsB[id] = struct{}{}
	}

	gotA := listEntityIDs(t, baseURL(), "agents", orgA)
	require.GreaterOrEqual(t, len(gotA), 3, "orgA list must include the 3 seeded rows")
	for id := range idsA {
		require.Contains(t, gotA, id, "orgA list must contain seeded id %s", id)
	}
	for id := range idsB {
		require.NotContains(t, gotA, id, "orgA list MUST NOT contain orgB id %s (FOUND-08)", id)
	}

	gotB := listEntityIDs(t, baseURL(), "agents", orgB)
	require.GreaterOrEqual(t, len(gotB), 2, "orgB list must include the 2 seeded rows")
	for id := range idsB {
		require.Contains(t, gotB, id, "orgB list must contain seeded id %s", id)
	}
	for id := range idsA {
		require.NotContains(t, gotB, id, "orgB list MUST NOT contain orgA id %s (FOUND-08)", id)
	}
}

// FOUND-06 HTTP-layer proof — two consecutive POSTs with the same
// (org_id, external_id) must collide. Wave 6 review: this test was
// dropped with the scaffold suite but FOUND-06 still applies to every
// catalog entity. Agents is the canonical probe; the same constraint
// holds for skills/queues/channels/adapters.
func TestCatalog_AgentsUniqueOrgExternalId(t *testing.T) {
	requireContainer(t)
	t.Parallel()
	org := freshOrg(t)

	code1, _ := postEntity(t, baseURL(), "agents", org, map[string]any{
		"external_id": "ext-dup-iso",
		"name":        "First",
		"email":       "first@example.com",
	})
	require.Equal(t, http.StatusCreated, code1, "first POST must succeed")

	code2, _ := postEntity(t, baseURL(), "agents", org, map[string]any{
		"external_id": "ext-dup-iso",
		"name":        "Second",
		"email":       "second@example.com",
	})
	require.Equal(t, http.StatusConflict, code2,
		"second POST with same (org_id, external_id) MUST return 409 (FOUND-06)")
}
