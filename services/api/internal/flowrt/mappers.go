package flowrt

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/runtime"
)

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func apiUUID(p pgtype.UUID) uuid.UUID { return uuid.UUID(p.Bytes) }

func ptrTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time
	return &tt
}

// parseGraph decodes a flow's opaque graph JSONB into the runtime authoring
// model. A missing/empty graph decodes to an empty Graph so the validator
// reports `empty_graph` rather than crashing.
func parseGraph(b []byte) (*runtime.Graph, error) {
	g := &runtime.Graph{}
	if len(b) == 0 {
		return g, nil
	}
	if err := json.Unmarshal(b, g); err != nil {
		return nil, fmt.Errorf("flowrt: parse graph: %w", err)
	}
	return g, nil
}

func jsonbToMap(b []byte) (map[string]any, error) {
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func mapFlowVersion(row generated.FlowVersion) (api.FlowVersion, error) {
	graph, err := jsonbToMap(row.Graph)
	if err != nil {
		return api.FlowVersion{}, fmt.Errorf("jsonbToMap(version graph): %w", err)
	}
	return api.FlowVersion{
		Id:                api.UUIDv7(apiUUID(row.ID)),
		OrgId:             api.UUIDv7(apiUUID(row.OrgID)),
		FlowId:            api.UUIDv7(apiUUID(row.FlowID)),
		FlowCode:          row.FlowCode,
		VersionNumber:     int(row.VersionNumber),
		Graph:             graph,
		PlanFormatVersion: int(row.PlanFormatVersion),
		CreatedAt:         ptrTime(row.CreatedAt),
	}, nil
}

func mapBinding(row generated.FlowEntryBinding) api.FlowEntryBinding {
	return api.FlowEntryBinding{
		Id:            api.UUIDv7(apiUUID(row.ID)),
		OrgId:         api.UUIDv7(apiUUID(row.OrgID)),
		Channel:       row.Channel,
		EntryCode:     row.EntryCode,
		FlowVersionId: api.UUIDv7(apiUUID(row.FlowVersionID)),
		FlowCode:      row.FlowCode,
		Active:        row.Active,
		CreatedAt:     ptrTime(row.CreatedAt),
		UpdatedAt:     ptrTime(row.UpdatedAt),
	}
}

// toAPIIssues converts runtime validation issues to the API DTO. Empty
// locators map to nil pointers (omitted from JSON) so a graph-level issue isn't
// rendered with blank node_id/edge_id/field.
func toAPIIssues(issues []runtime.ValidationIssue) []api.FlowValidationIssue {
	out := make([]api.FlowValidationIssue, 0, len(issues))
	for _, i := range issues {
		issue := api.FlowValidationIssue{Code: i.Code, Message: i.Message}
		if i.NodeID != "" {
			n := i.NodeID
			issue.NodeId = &n
		}
		if i.EdgeID != "" {
			e := i.EdgeID
			issue.EdgeId = &e
		}
		if i.Field != "" {
			f := i.Field
			issue.Field = &f
		}
		out = append(out, issue)
	}
	return out
}
