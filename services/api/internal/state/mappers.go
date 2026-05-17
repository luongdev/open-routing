package state

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// mapAgentState converts a sqlc-generated agent_states row to the wire
// DTO. Nullable columns surface as *T per OpenAPI nullable semantics.
// Caller is responsible for orgID scoping (the row already carries the
// right org_id from the WHERE clause).
func mapAgentState(row generated.AgentState) api.AgentState {
	out := api.AgentState{
		AgentId:      openapi_types.UUID(row.AgentID.Bytes),
		OrgId:        openapi_types.UUID(row.OrgID.Bytes),
		Status:       api.AgentStatus(row.Status),
		StateVersion: int(row.StateVersion),
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	if row.EngagedChannel != nil {
		ec := api.ChannelType(*row.EngagedChannel)
		out.EngagedChannel = &ec
	}
	if row.BreakReasonID.Valid {
		br := openapi_types.UUID(row.BreakReasonID.Bytes)
		out.BreakReasonId = &br
	}
	if row.PostInteractionState != nil {
		pis := api.PostInteractionState(*row.PostInteractionState)
		out.PostInteractionState = &pis
	}
	if row.WrapupUntil.Valid {
		wu := row.WrapupUntil.Time
		out.WrapupUntil = &wu
	}
	return out
}

// pgUUID wraps uuid.UUID into pgtype.UUID for sqlc params. Always Valid=true —
// the state schema has no nullable UUID PRIMARY KEYs; callers needing
// to pass NULL should build pgtype.UUID{Valid: false} directly.
// Mirrors the catalog helper of the same name.
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
