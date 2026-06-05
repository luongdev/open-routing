package flowrt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

// buildSnapshot captures the routable candidate pool under a read-only
// REPEATABLE READ transaction (a point-in-time view so the simulation is
// deterministic and replayable), then keys it by every queue the graph
// references. There is no agent↔queue membership table in v0.2 — a queue
// carries priority/channel semantics, and the pool is "Ready, enabled agents",
// narrowed downstream by match_skill/filter.
func (e *Endpoints) buildSnapshot(ctx context.Context, orgID pgtype.UUID, graph *runtime.Graph) (*runtime.Snapshot, error) {
	tx, err := e.deps.OrgDB.BeginTxWith(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }() // read-only: always rollback to release the snapshot

	q := generated.New(tx)
	rows, err := q.ListRoutableCandidates(ctx, orgID)
	if err != nil {
		return nil, err
	}
	pool, _ := poolFromRows(rows)
	qc, err := e.mapQueueCandidates(ctx, q, orgID, graph, pool)
	if err != nil {
		return nil, err
	}
	return &runtime.Snapshot{QueueCandidates: qc}, nil
}

// poolFromRows folds the per-(agent,skill) rows into a ranked candidate pool and
// returns the agent code→uuid map (the live snapshot needs the uuid for
// presence/capacity lookups; the sim path ignores it).
func poolFromRows(rows []generated.ListRoutableCandidatesRow) ([]runtime.Candidate, map[string]uuid.UUID) {
	byAgent := map[string]*runtime.Candidate{}
	idByCode := make(map[string]uuid.UUID, len(rows))
	order := make([]string, 0, len(rows))
	for _, r := range rows {
		c, ok := byAgent[r.AgentCode]
		if !ok {
			c = &runtime.Candidate{AgentID: r.AgentCode, Proficiency: map[string]int{}}
			if r.AvailableSince.Valid {
				c.AvailableSince = r.AvailableSince.Time
			}
			byAgent[r.AgentCode] = c
			idByCode[r.AgentCode] = apiUUID(r.AgentID)
			order = append(order, r.AgentCode)
		}
		if r.SkillCode != nil && r.Proficiency != nil {
			c.Proficiency[*r.SkillCode] = int(*r.Proficiency)
		}
	}
	pool := make([]runtime.Candidate, 0, len(order))
	for _, code := range order {
		pool = append(pool, *byAgent[code])
	}
	// Rank by longest-available so a direct route_queue → reservation (no
	// match_skill) still offers the longest-idle agent first (review M10).
	return runtime.RankCandidates(pool, nil), idByCode
}

// validQueueCodes returns the codes of every route_queue the graph references
// that still EXISTS and is ENABLED. A missing/disabled queue is omitted so
// route_queue yields missing_catalog_reference instead of routing to it (H7).
func (e *Endpoints) validQueueCodes(ctx context.Context, q *generated.Queries, orgID pgtype.UUID, graph *runtime.Graph) ([]string, error) {
	var codes []string
	seen := map[string]bool{}
	for _, n := range graph.Nodes {
		if n.Kind != runtime.NodeRouteQueue {
			continue
		}
		var cfg struct {
			Queue string `json:"queue"`
		}
		if json.Unmarshal(n.Config, &cfg) != nil || cfg.Queue == "" || seen[cfg.Queue] {
			continue
		}
		qrow, qerr := q.GetQueueByCode(ctx, generated.GetQueueByCodeParams{OrgID: orgID, Code: cfg.Queue})
		if errors.Is(qerr, pgx.ErrNoRows) || (qerr == nil && !qrow.Enabled) {
			continue
		}
		if qerr != nil {
			return nil, qerr // real DB error must not masquerade as a missing queue (re-review MED)
		}
		seen[cfg.Queue] = true
		codes = append(codes, cfg.Queue)
	}
	return codes, nil
}

// mapQueueCandidates keys the pool by every valid queue the graph references.
func (e *Endpoints) mapQueueCandidates(ctx context.Context, q *generated.Queries, orgID pgtype.UUID, graph *runtime.Graph, pool []runtime.Candidate) (map[string][]runtime.Candidate, error) {
	codes, err := e.validQueueCodes(ctx, q, orgID, graph)
	if err != nil {
		return nil, err
	}
	qc := make(map[string][]runtime.Candidate, len(codes))
	for _, c := range codes {
		qc[c] = pool
	}
	return qc, nil
}

// requiredSkillsFromTrace collects the skills the match_skill nodes that ACTUALLY
// RAN on this route's path filtered on — denormalized onto
// route_requests.required_skills so the W4 matcher finds eligible agents
// (required_skills <@ agent skills) without re-running the flow. Reading the
// executed trace (not the static graph) avoids over-constraining: a branching
// flow whose VIP arm needs skill_es and standard arm needs skill_fr must NOT
// demand both — only the branch the route took (cross-AI review HIGH).
func requiredSkillsFromTrace(tr runtime.Trace) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range tr.Steps {
		if s.Kind != runtime.NodeMatchSkill {
			continue
		}
		if sk, ok := s.Output["skill"].(string); ok && sk != "" && !seen[sk] {
			seen[sk] = true
			out = append(out, sk)
		}
	}
	return out
}

// routingSnapshot picks the candidate source: LIVE (presence+capacity filtered)
// when the gateway/presence deps are wired, else the simulation snapshot. There
// is NO silent live→sim fallback (review BLOCK): a real binary always wires
// presence (cmd/api), so reaching buildSnapshot means a genuine sim/test run.
func (e *Endpoints) routingSnapshot(ctx context.Context, orgID uuid.UUID, channel string, graph *runtime.Graph) (*runtime.Snapshot, error) {
	if e.deps.Presence != nil {
		return e.buildLiveSnapshot(ctx, orgID, channel, graph)
	}
	return e.buildSnapshot(ctx, pgUUID(orgID), graph)
}

// buildLiveSnapshot is the LIVE candidate source: the Ready+eligible pool
// post-filtered by a connection lease (presence) AND under-capacity
// (held < channel capacity). A presence-store error is returned as an infra
// error — the caller parks the route for retry — NEVER read as "disconnected"
// and NEVER fallen back to a DB snapshot (review HIGH). under-capacity uses the
// HELD count so it doesn't depend on slots being pre-provisioned; the offer tx
// provisions+acquires authoritatively.
func (e *Endpoints) buildLiveSnapshot(ctx context.Context, orgID uuid.UUID, channel string, graph *runtime.Graph) (*runtime.Snapshot, error) {
	// Phase 1 — all DB reads in ONE short read-only snapshot, then CLOSE it before
	// any Redis I/O so a slow presence store can't pin a pg connection / hold the
	// snapshot open (review HIGH). Bulk the held counts (one query, not N).
	pool, idByCode, heldByAgent, queueCodes, err := e.liveSnapshotReads(ctx, orgID, channel, graph)
	if err != nil {
		return nil, err
	}

	// Phase 2 — batch the lease check OUTSIDE the tx. An error parks the route
	// (infra), never read as "everyone disconnected" (review HIGH).
	agentIDs := make([]uuid.UUID, 0, len(idByCode))
	for _, id := range idByCode {
		agentIDs = append(agentIDs, id)
	}
	connected, err := e.deps.Presence.ConnectedMany(ctx, orgID, agentIDs)
	if err != nil {
		return nil, err
	}

	// Phase 3 — filter by lease + under-capacity (held < cap) and key by queue.
	cap := channelCapacity(channel)
	live := make([]runtime.Candidate, 0, len(pool))
	for _, c := range pool {
		id := idByCode[c.AgentID]
		if !connected[id] || heldByAgent[id] >= cap {
			continue
		}
		live = append(live, c)
	}
	qc := make(map[string][]runtime.Candidate, len(queueCodes))
	for _, code := range queueCodes {
		qc[code] = live
	}
	return &runtime.Snapshot{QueueCandidates: qc}, nil
}

// liveSnapshotReads does every DB read for the live snapshot in one short
// read-only tx (candidate pool + bulk held counts + valid queue codes) and
// returns them so the caller can close the tx before touching Redis.
func (e *Endpoints) liveSnapshotReads(ctx context.Context, orgID uuid.UUID, channel string, graph *runtime.Graph) (pool []runtime.Candidate, idByCode map[string]uuid.UUID, heldByAgent map[uuid.UUID]int32, queueCodes []string, err error) {
	tx, err := e.deps.OrgDB.BeginTxWith(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := generated.New(tx)
	rows, err := q.ListRoutableCandidates(ctx, pgUUID(orgID))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	pool, idByCode = poolFromRows(rows)
	agentIDs := make([]pgtype.UUID, 0, len(idByCode))
	for _, id := range idByCode {
		agentIDs = append(agentIDs, pgUUID(id))
	}
	heldByAgent = make(map[uuid.UUID]int32, len(agentIDs))
	if len(agentIDs) > 0 {
		heldRows, hErr := q.CountHeldCapacityByAgents(ctx, generated.CountHeldCapacityByAgentsParams{OrgID: pgUUID(orgID), Channel: channel, Column3: agentIDs})
		if hErr != nil {
			return nil, nil, nil, nil, hErr
		}
		for _, hr := range heldRows {
			heldByAgent[apiUUID(hr.AgentID)] = hr.Held
		}
	}
	queueCodes, err = e.validQueueCodes(ctx, q, pgUUID(orgID), graph)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return pool, idByCode, heldByAgent, queueCodes, nil
}

// mapTraceSteps converts the runtime trace's steps to the API shape, assigning
// the ordinal index and mapping the runtime status ("failed") to the API enum
// ("error"). duration_ms is CPU time, not virtual wait time.
func mapTraceSteps(tr runtime.Trace) []api.TraceStep {
	out := make([]api.TraceStep, len(tr.Steps))
	for i, s := range tr.Steps {
		st := api.TraceStep{Index: i, NodeId: s.NodeID, NodeKind: string(s.Kind), Status: mapStepStatus(s.Status)}
		if s.Port != "" {
			p := s.Port
			st.Port = &p
		}
		d := float32(s.DurationMs)
		st.DurationMs = &d
		if len(s.Output) > 0 {
			o := map[string]any(s.Output)
			st.Output = &o
		}
		if s.Error != "" {
			e := s.Error
			st.Error = &e
		}
		if s.Region != "" {
			rg := s.Region
			st.Region = &rg
		}
		st.Iteration = s.Iteration
		st.Branch = s.Branch
		if s.Caught {
			c := true
			st.Caught = &c
		}
		out[i] = st
	}
	return out
}

func mapStepStatus(s string) api.TraceStepStatus {
	switch s {
	case "ok":
		return api.Ok
	case "suspended":
		return api.Suspended
	default: // "failed"
		return api.Error
	}
}

// scriptedOutcomes splits the request's scripted reservation outcomes: entries
// WITH node_id pin that node's result port (per-node map); the rest form the
// legacy ordered per-offer queue.
func scriptedOutcomes(in *[]api.SimulateScriptedReservationOutcome) ([]runtime.ReservationOutcome, map[string]string) {
	if in == nil {
		return nil, nil
	}
	var queue []runtime.ReservationOutcome
	byNode := map[string]string{}
	for _, o := range *in {
		if o.NodeId != nil && *o.NodeId != "" {
			byNode[*o.NodeId] = string(o.Outcome) // the result port (accepted/timeout/no_candidate)
			continue
		}
		switch o.Outcome {
		case api.Accepted:
			queue = append(queue, runtime.ResvAccepted)
		case api.Timeout:
			queue = append(queue, runtime.ResvTimeout)
		default:
			queue = append(queue, runtime.ResvRejected)
		}
	}
	if len(byNode) == 0 {
		byNode = nil
	}
	return queue, byNode
}

func (e *Endpoints) SimulateFlow(ctx context.Context, req api.SimulateFlowRequestObject) (api.SimulateFlowResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
			Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context",
		}}, nil
	}
	flowID := uuid.UUID(req.Id)
	q := generated.New(e.deps.OrgDB)
	flow, err := q.GetFlowByIdAnyVersion(ctx, generated.GetFlowByIdAnyVersionParams{ID: pgUUID(flowID), OrgID: pgUUID(orgID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.SimulateFlow404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Error: api.ErrorCodeNotFound, Reason: "flow_not_found"}}, nil
	}
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "simulate: load flow", "err", err)
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "load_failed"}}, nil
	}

	graph, gErr := parseGraph(flow.Graph)
	if gErr != nil {
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "graph_parse_failed"}}, nil
	}

	refs := newDBRefs(ctx, q, pgUUID(orgID))
	issues, vErr := runtime.ValidateGraph(ctx, graph, e.reg, refs)
	if vErr != nil || refs.Err() != nil {
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "validation_failed"}}, nil
	}
	if len(issues) > 0 {
		return api.SimulateFlow422JSONResponse(api.FlowValidationResult{Valid: false, Issues: toAPIIssues(issues)}), nil
	}

	plan, cErr := runtime.Compile(graph, e.reg)
	if cErr != nil {
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "compile_failed"}}, nil
	}

	// Resolve the virtual clock. Default to the draft's updated_at (deterministic
	// per draft version — NOT time.Now(), which would make an omitted-clock
	// simulation non-reproducible). Either way it is returned + persisted in
	// simulation_input so a replay is exact (cross-AI review BLOCK/HIGH).
	clockStart := time.Unix(0, 0).UTC()
	if flow.UpdatedAt.Valid {
		clockStart = flow.UpdatedAt.Time.UTC()
	}
	if req.Body != nil && req.Body.VirtualClockStart != nil {
		clockStart = req.Body.VirtualClockStart.UTC()
	}

	snapshot, sErr := e.buildSnapshot(ctx, pgUUID(orgID), graph)
	if sErr != nil {
		e.deps.Logger.ErrorContext(ctx, "simulate: snapshot", "err", sErr)
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "snapshot_failed"}}, nil
	}

	var input map[string]any
	var scripted []runtime.ReservationOutcome
	var nodeOutcomes map[string]string
	var scriptedInputs map[string]any
	if req.Body != nil {
		input = req.Body.InteractionInput
		scripted, nodeOutcomes = scriptedOutcomes(req.Body.ScriptedReservationOutcomes)
		if req.Body.ScriptedEffectOutputs != nil {
			scriptedInputs = *req.Body.ScriptedEffectOutputs
		}
	}

	trace, rErr := runtime.Simulate(ctx, e.reg, plan, runtime.SimInput{
		Input: input, Snapshot: snapshot, ScriptedOutcomes: scripted, NodeOutcomes: nodeOutcomes, ScriptedInputs: scriptedInputs, ClockStart: clockStart,
	})
	if rErr != nil {
		e.deps.Logger.ErrorContext(ctx, "simulate: run", "err", rErr)
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "simulate_failed"}}, nil
	}

	apiSteps := mapTraceSteps(trace)
	traceID := uuid.Must(uuid.NewV7())
	// Persist the RESOLVED clock (not the raw request, which may have a nil
	// virtual_clock_start) so simulation_input replays the exact run.
	var body api.SimulateFlowRequest
	if req.Body != nil {
		body = *req.Body
	}
	body.VirtualClockStart = &clockStart
	stepsJSON, mErr1 := json.Marshal(apiSteps)
	planJSON, mErr2 := json.Marshal(plan)
	inputJSON, mErr3 := json.Marshal(body)
	readSetJSON, mErr4 := json.Marshal(snapshot)
	if err := errors.Join(mErr1, mErr2, mErr3, mErr4); err != nil {
		e.deps.Logger.ErrorContext(ctx, "simulate: marshal trace", "err", err)
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "trace_marshal_failed"}}, nil
	}
	sum := sha256.Sum256(flow.Graph)
	graphHash := hex.EncodeToString(sum[:])
	pfv := int32(runtime.PlanFormatVersion)
	outcome := trace.Outcome

	row, iErr := q.InsertTrace(ctx, generated.InsertTraceParams{
		ID: pgUUID(traceID), OrgID: pgUUID(orgID), Kind: string(api.Simulation),
		FlowID: pgUUID(flowID), Steps: stepsJSON, Outcome: &outcome,
		CompiledPlanSnapshot: planJSON, PlanFormatVersion: &pfv,
		SimulationInput: inputJSON, ReadSetSnapshot: readSetJSON, GraphHash: &graphHash,
	})
	if iErr != nil {
		e.deps.Logger.ErrorContext(ctx, "simulate: persist trace", "err", iErr)
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "trace_persist_failed"}}, nil
	}

	apiTrace, tErr := rowToAPITrace(row)
	if tErr != nil {
		e.deps.Logger.ErrorContext(ctx, "simulate: map trace", "err", tErr)
		return api.SimulateFlow500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "trace_map_failed"}}, nil
	}
	return api.SimulateFlow200JSONResponse(api.SimulateFlowResponse{VirtualClockStart: clockStart, Trace: apiTrace}), nil
}

func (e *Endpoints) ListFlowTraces(ctx context.Context, req api.ListFlowTracesRequestObject) (api.ListFlowTracesResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListFlowTraces500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "missing_org_id_in_context"}}, nil
	}
	// Clamp to the OpenAPI bounds (1..100, default 20) so a hostile/odd limit
	// can't bypass the cap or hit a DB error.
	limit := int32(20)
	if req.Params.Limit != nil {
		limit = int32(*req.Params.Limit) //nolint:gosec // Limit query param is bounded (<=100)
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := generated.New(e.deps.OrgDB).ListFlowTraces(ctx, generated.ListFlowTracesParams{
		OrgID: pgUUID(orgID), FlowID: pgUUID(uuid.UUID(req.Id)), Limit: limit,
	})
	if err != nil {
		e.deps.Logger.ErrorContext(ctx, "list flow traces", "err", err)
		return api.ListFlowTraces500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "list_failed"}}, nil
	}
	out := make([]api.Trace, 0, len(rows))
	for _, r := range rows {
		t, mErr := rowToAPITrace(r)
		if mErr != nil {
			// A corrupt persisted trace is a server fault, not a row to hide.
			e.deps.Logger.ErrorContext(ctx, "list flow traces: map row", "err", mErr)
			return api.ListFlowTraces500JSONResponse{InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{Error: api.ErrorCodeInternal, Reason: "trace_map_failed"}}, nil
		}
		out = append(out, t)
	}
	return api.ListFlowTraces200JSONResponse{Traces: out}, nil
}

// rowToAPITrace maps a persisted traces row to the API shape, unmarshaling the
// stored API-shaped steps.
func rowToAPITrace(row generated.Trace) (api.Trace, error) {
	var steps []api.TraceStep
	if len(row.Steps) > 0 {
		if err := json.Unmarshal(row.Steps, &steps); err != nil {
			return api.Trace{}, err
		}
	}
	t := api.Trace{
		Id:      api.UUIDv7(uuid.UUID(row.ID.Bytes)),
		OrgId:   api.UUIDv7(uuid.UUID(row.OrgID.Bytes)),
		Kind:    api.TraceKind(row.Kind),
		Outcome: row.Outcome,
		Steps:   steps,
	}
	if row.FlowID.Valid {
		f := api.UUIDv7(uuid.UUID(row.FlowID.Bytes))
		t.FlowId = &f
	}
	if row.FlowVersionID.Valid {
		v := api.UUIDv7(uuid.UUID(row.FlowVersionID.Bytes))
		t.FlowVersionId = &v
	}
	if row.RouteRequestID.Valid {
		rr := api.UUIDv7(uuid.UUID(row.RouteRequestID.Bytes))
		t.RouteRequestId = &rr
	}
	if row.CreatedAt.Valid {
		ct := row.CreatedAt.Time
		t.CreatedAt = &ct
	}
	return t, nil
}
