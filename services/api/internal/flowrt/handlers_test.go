package flowrt

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

type fixture struct {
	e     *Endpoints
	orgID uuid.UUID
	ctx   context.Context
	q     *generated.Queries
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if sharedPool == nil {
		t.Skip("flowrt: sharedPool nil — Docker testcontainer unavailable (run without -short)")
		return nil
	}
	ctx := context.Background()
	_, err := sharedPool.Exec(ctx, `TRUNCATE TABLE flow_entry_bindings, flow_versions, flows, skills, queues, channels, adapters CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	orgDB := db.NewOrgDB(sharedPool, db.NewSQLChecker(), db.ValidationPanic)
	orgID := uuid.Must(uuid.NewV7())
	return &fixture{
		e:     New(Deps{OrgDB: orgDB, Logger: logger}),
		orgID: orgID,
		ctx:   orgkey.SetOrgID(ctx, orgID),
		q:     generated.New(sharedPool),
	}
}

func cfg(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return b
}

func (f *fixture) seedFlow(t *testing.T, code string, g runtime.Graph) uuid.UUID {
	t.Helper()
	graphBytes, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal graph: %v", err)
	}
	id := uuid.Must(uuid.NewV7())
	_, err = f.q.InsertFlow(f.ctx, generated.InsertFlowParams{
		ID: pgUUID(id), OrgID: pgUUID(f.orgID), Code: code, Name: code, Graph: graphBytes, Enabled: true,
	})
	if err != nil {
		t.Fatalf("seed flow: %v", err)
	}
	return id
}

func (f *fixture) seedQueue(t *testing.T, code string) {
	t.Helper()
	_, err := f.q.InsertQueue(f.ctx, generated.InsertQueueParams{
		ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(f.orgID), Code: code, Name: code,
		ChannelTypes: []string{"voice"}, Priority: 0, AcwSec: 0, Enabled: true,
	})
	if err != nil {
		t.Fatalf("seed queue: %v", err)
	}
}

func (f *fixture) seedSkill(t *testing.T, code string) {
	t.Helper()
	_, err := f.q.InsertSkill(f.ctx, generated.InsertSkillParams{
		ID: pgUUID(uuid.Must(uuid.NewV7())), OrgID: pgUUID(f.orgID), Code: code, Name: code,
		SkillType: "binary", Enabled: true,
	})
	if err != nil {
		t.Fatalf("seed skill: %v", err)
	}
}

func (f *fixture) activeBindingCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := sharedPool.QueryRow(f.ctx,
		`SELECT count(*) FROM flow_entry_bindings WHERE org_id=$1 AND active=TRUE`, f.orgID).Scan(&n); err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	return n
}

// validGraph: trigger -> route_queue(queue_vip) -> match_skill(skill_es) -> end.
func validGraph(t *testing.T) runtime.Graph {
	return runtime.Graph{
		Nodes: []runtime.GraphNode{
			{ID: "t", Kind: runtime.NodeTrigger},
			{ID: "q", Kind: runtime.NodeRouteQueue, Config: cfg(t, map[string]string{"queue": "queue_vip"})},
			{ID: "s", Kind: runtime.NodeMatchSkill, Config: cfg(t, map[string]string{"skill": "skill_es"})},
			{ID: "end", Kind: runtime.NodeEnd},
		},
		Edges: []runtime.GraphEdge{
			{ID: "e1", From: "t", To: "q"},
			{ID: "e2", From: "q", To: "s"},
			{ID: "e3", From: "s", To: "end"},
		},
	}
}

func (f *fixture) seedCatalog(t *testing.T) {
	f.seedQueue(t, "queue_vip")
	f.seedSkill(t, "skill_es")
}

func TestValidateFlow_Valid(t *testing.T) {
	f := newFixture(t)
	f.seedCatalog(t)
	id := f.seedFlow(t, "flow_ok", validGraph(t))

	resp, err := f.e.ValidateFlow(f.ctx, api.ValidateFlowRequestObject{Id: api.EntityIdPath(id)})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	r, ok := resp.(api.ValidateFlow200JSONResponse)
	if !ok {
		t.Fatalf("want 200, got %T", resp)
	}
	if !r.Valid || len(r.Issues) != 0 {
		t.Fatalf("want valid+no issues, got valid=%v issues=%+v", r.Valid, r.Issues)
	}
}

func TestValidateFlow_InvalidMissingCatalogRef(t *testing.T) {
	f := newFixture(t)
	// No catalog seeded: queue + skill refs dangle.
	id := f.seedFlow(t, "flow_bad", validGraph(t))

	resp, _ := f.e.ValidateFlow(f.ctx, api.ValidateFlowRequestObject{Id: api.EntityIdPath(id)})
	r := resp.(api.ValidateFlow200JSONResponse)
	if r.Valid {
		t.Fatal("expected invalid")
	}
	got := 0
	for _, is := range r.Issues {
		if is.Code == runtime.IssueMissingCatalog {
			got++
		}
	}
	if got != 2 {
		t.Fatalf("want two missing_catalog_reference, got %+v", r.Issues)
	}
}

func TestValidateFlow_NotFound(t *testing.T) {
	f := newFixture(t)
	resp, _ := f.e.ValidateFlow(f.ctx, api.ValidateFlowRequestObject{Id: api.EntityIdPath(uuid.Must(uuid.NewV7()))})
	if _, ok := resp.(api.ValidateFlow404JSONResponse); !ok {
		t.Fatalf("want 404, got %T", resp)
	}
}

func TestPublishFlow_SuccessThenRepublishKeepsOneActiveBinding(t *testing.T) {
	f := newFixture(t)
	f.seedCatalog(t)
	id := f.seedFlow(t, "flow_pub", validGraph(t))

	resp, err := f.e.PublishFlow(f.ctx, api.PublishFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	pub, ok := resp.(api.PublishFlow201JSONResponse)
	if !ok {
		t.Fatalf("want 201, got %T", resp)
	}
	if pub.Version.VersionNumber != 1 || !pub.Binding.Active {
		t.Fatalf("unexpected publish result: %+v", pub)
	}

	// Republish (same draft version 1) → version 2, still exactly one active binding.
	resp2, err := f.e.PublishFlow(f.ctx, api.PublishFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	pub2 := resp2.(api.PublishFlow201JSONResponse)
	if pub2.Version.VersionNumber != 2 {
		t.Fatalf("want version 2, got %d", pub2.Version.VersionNumber)
	}
	if n := f.activeBindingCount(t); n != 1 {
		t.Fatalf("want exactly one active binding, got %d", n)
	}
	if pub2.Binding.FlowVersionId != pub2.Version.Id {
		t.Fatal("active binding should point at the newest version")
	}
}

func TestPublishFlow_InvalidGraph422(t *testing.T) {
	f := newFixture(t)
	// Graph with no trigger/end and a missing-required-field if_else.
	g := runtime.Graph{Nodes: []runtime.GraphNode{
		{ID: "i", Kind: runtime.NodeIfElse, Config: cfg(t, map[string]string{"expr": ""})},
	}}
	id := f.seedFlow(t, "flow_inv", g)

	resp, _ := f.e.PublishFlow(f.ctx, api.PublishFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 1},
	})
	r, ok := resp.(api.PublishFlow422JSONResponse)
	if !ok {
		t.Fatalf("want 422, got %T", resp)
	}
	if r.Valid || len(r.Issues) == 0 {
		t.Fatalf("want invalid with issues, got %+v", r)
	}
	if f.activeBindingCount(t) != 0 {
		t.Fatal("invalid publish must not create a binding")
	}
}

func TestPublishFlow_DraftVersionConflict409(t *testing.T) {
	f := newFixture(t)
	f.seedCatalog(t)
	id := f.seedFlow(t, "flow_vc", validGraph(t))

	resp, _ := f.e.PublishFlow(f.ctx, api.PublishFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: 99}, // wrong
	})
	if _, ok := resp.(api.PublishFlow409JSONResponse); !ok {
		t.Fatalf("want 409, got %T", resp)
	}
}

func TestRollbackFlow_ReactivatesPriorVersion(t *testing.T) {
	f := newFixture(t)
	f.seedCatalog(t)
	id := f.seedFlow(t, "flow_rb", validGraph(t))

	// Publish v1, then v2.
	mustPublish(t, f, id, 1)
	v2 := mustPublish(t, f, id, 1)

	// Rollback to version 1.
	resp, err := f.e.RollbackFlow(f.ctx, api.RollbackFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.RollbackFlowRequest{Channel: "voice", EntryCode: "main", ToVersionNumber: 1},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rb, ok := resp.(api.RollbackFlow200JSONResponse)
	if !ok {
		t.Fatalf("want 200, got %T", resp)
	}
	if rb.Version.VersionNumber != 1 {
		t.Fatalf("want version 1 active, got %d", rb.Version.VersionNumber)
	}
	if rb.Binding.FlowVersionId == v2.Version.Id {
		t.Fatal("binding should no longer point at v2")
	}
	if f.activeBindingCount(t) != 1 {
		t.Fatal("rollback must leave exactly one active binding")
	}
}

func TestRollbackFlow_VersionNotFound404(t *testing.T) {
	f := newFixture(t)
	f.seedCatalog(t)
	id := f.seedFlow(t, "flow_rb2", validGraph(t))
	mustPublish(t, f, id, 1)

	resp, _ := f.e.RollbackFlow(f.ctx, api.RollbackFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.RollbackFlowRequest{Channel: "voice", EntryCode: "main", ToVersionNumber: 7},
	})
	if _, ok := resp.(api.RollbackFlow404JSONResponse); !ok {
		t.Fatalf("want 404, got %T", resp)
	}
}

func TestPublishFlow_ExpectedBindingMismatch409(t *testing.T) {
	f := newFixture(t)
	f.seedCatalog(t)
	id := f.seedFlow(t, "flow_exp", validGraph(t))
	mustPublish(t, f, id, 1)

	wrong := api.UUIDv7(uuid.Must(uuid.NewV7()))
	resp, _ := f.e.PublishFlow(f.ctx, api.PublishFlowRequestObject{
		Id: api.EntityIdPath(id),
		Body: &api.PublishFlowRequest{
			Channel: "voice", EntryCode: "main", Version: 1,
			ExpectedCurrentFlowVersionId: &wrong,
		},
	})
	if _, ok := resp.(api.PublishFlow409JSONResponse); !ok {
		t.Fatalf("want 409 on expected-binding mismatch, got %T", resp)
	}
}

func TestListAndGetFlowVersions(t *testing.T) {
	f := newFixture(t)
	f.seedCatalog(t)
	id := f.seedFlow(t, "flow_lv", validGraph(t))
	mustPublish(t, f, id, 1)
	p2 := mustPublish(t, f, id, 1)

	lresp, _ := f.e.ListFlowVersions(f.ctx, api.ListFlowVersionsRequestObject{Id: api.EntityIdPath(id)})
	l, ok := lresp.(api.ListFlowVersions200JSONResponse)
	if !ok || len(l.Items) != 2 {
		t.Fatalf("want 2 versions, got %T %+v", lresp, l)
	}
	// DESC order: newest first.
	if l.Items[0].VersionNumber != 2 {
		t.Fatalf("want newest-first, got %d", l.Items[0].VersionNumber)
	}

	gresp, _ := f.e.GetFlowVersion(f.ctx, api.GetFlowVersionRequestObject{Id: api.EntityIdPath(uuid.UUID(p2.Version.Id))})
	if _, ok := gresp.(api.GetFlowVersion200JSONResponse); !ok {
		t.Fatalf("want 200, got %T", gresp)
	}
}

func mustPublish(t *testing.T, f *fixture, id uuid.UUID, draftVersion int) api.PublishFlow201JSONResponse {
	t.Helper()
	resp, err := f.e.PublishFlow(f.ctx, api.PublishFlowRequestObject{
		Id:   api.EntityIdPath(id),
		Body: &api.PublishFlowRequest{Channel: "voice", EntryCode: "main", Version: draftVersion},
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	pub, ok := resp.(api.PublishFlow201JSONResponse)
	if !ok {
		t.Fatalf("publish want 201, got %T", resp)
	}
	return pub
}
