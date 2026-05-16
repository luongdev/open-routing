// Package scaffold implements api.StrictServerInterface for the throwaway
// _scaffold routes (D-46). Phase 1's hand-written (w,r) handlers are
// migrated to typed strict-server handlers; Phase 3 deletes both the
// package and the corresponding openapi.yaml paths when real catalog
// entities replace _scaffold.
//
// Invariants carried forward from Phase 1 (MUST NOT regress — FOUND-08
// integration suite proves them):
//   - Authoritative org_id from ctx (FOUND-05, never URL/body)
//   - UUIDv7 minted server-side at write boundary (D-19)
//   - pgx.ErrNoRows → 404 with "not_found" code (FOUND-08 leakage guard)
package scaffold

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// Handler implements the Scaffold portion of api.StrictServerInterface
// (D-46). The compositeServer in server.go embeds this and provides 501
// stubs for Agents/Skills/Queues/Channels/Adapters/BreakReasons/AgentStates/
// Imports — see server/stubs.go.
//
// NOTE: handlers do NOT populate the ErrorResponse.RequestId field — that
// concern is centralized in server.RequestIDInjectionMiddleware (B-1, D-35),
// a StrictMiddlewareFunc that wraps every operation and injects request_id
// from ctx into any returned ErrorResponse-shaped body. Handlers focus on
// business logic only.
type Handler struct {
	orgDB *db.OrgDB
}

// NewHandler returns a scaffold.Handler backed by the given OrgDB.
func NewHandler(orgDB *db.OrgDB) *Handler {
	return &Handler{orgDB: orgDB}
}

// CreateScaffold implements api.StrictServerInterface.CreateScaffold.
// Spec contract: POST /v1/orgs/{org_id}/_scaffold with {external_id, name}
// returns 201 Scaffold (CAT-like — but _scaffold is throwaway).
//
// oapi-codegen has already JSON-decoded the body into req.Body and
// rejected missing required fields per openapi.yaml. The handler focuses
// on org_id resolution + UUIDv7 mint + DB insert.
func (h *Handler) CreateScaffold(ctx context.Context, req api.CreateScaffoldRequestObject) (api.CreateScaffoldResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		// Defensive guard: OrgContext middleware guarantees presence inside
		// /v1; this branch only fires on a wiring regression. RequestId
		// stays empty here; server.RequestIDInjectionMiddleware fills it.
		return api.CreateScaffold500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "missing_org_id_in_context",
			},
		}, nil
	}

	q := generated.New(h.orgDB)
	id := uuid.Must(uuid.NewV7()) // D-19 UUIDv7 everywhere
	row, err := q.InsertScaffold(ctx, generated.InsertScaffoldParams{
		ID:         toPgUUID(id),
		OrgID:      toPgUUID(orgID),
		ExternalID: req.Body.ExternalId,
		Name:       req.Body.Name,
	})
	if err != nil {
		return api.CreateScaffold500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "insert_failed",
			},
		}, nil
	}
	return api.CreateScaffold201JSONResponse(toAPIScaffold(row)), nil
}

// ListScaffolds implements api.StrictServerInterface.ListScaffolds.
func (h *Handler) ListScaffolds(ctx context.Context, req api.ListScaffoldsRequestObject) (api.ListScaffoldsResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.ListScaffolds500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "missing_org_id_in_context",
			},
		}, nil
	}
	q := generated.New(h.orgDB)
	rows, err := q.ListScaffolds(ctx, toPgUUID(orgID))
	if err != nil {
		return api.ListScaffolds500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "list_failed",
			},
		}, nil
	}
	out := make([]api.Scaffold, 0, len(rows))
	for _, s := range rows {
		out = append(out, toAPIScaffold(s))
	}
	return api.ListScaffolds200JSONResponse(out), nil
}

// GetScaffoldById implements api.StrictServerInterface.GetScaffoldById.
// FOUND-08 cross-org probe: a row in another org returns pgx.ErrNoRows
// because the sqlc query body filters by (id, org_id); the handler maps
// that to 404 — identical to a genuinely-missing row.
func (h *Handler) GetScaffoldById(ctx context.Context, req api.GetScaffoldByIdRequestObject) (api.GetScaffoldByIdResponseObject, error) {
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetScaffoldById500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "missing_org_id_in_context",
			},
		}, nil
	}
	// oapi-codegen has already parsed {id} path parameter as uuid.UUID
	// into req.Id (EntityIdPath = UUIDv7 = openapi_types.UUID).
	q := generated.New(h.orgDB)
	row, err := q.GetScaffoldByID(ctx, generated.GetScaffoldByIDParams{
		ID:    toPgUUID(uuid.UUID(req.Id)),
		OrgID: toPgUUID(orgID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return api.GetScaffoldById404JSONResponse{
				NotFoundJSONResponse: api.NotFoundJSONResponse{
					Error:  api.ErrorCodeNotFound,
					Reason: "no_such_scaffold",
				},
			}, nil
		}
		return api.GetScaffoldById500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "get_failed",
			},
		}, nil
	}
	return api.GetScaffoldById200JSONResponse(toAPIScaffold(row)), nil
}

// toPgUUID wraps a google/uuid value into pgtype.UUID for sqlc parameter
// passing. Valid=true because every UUID we pass is a real non-null value
// (NOT NULL columns in _scaffold).
func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// toAPIScaffold converts the sqlc-generated row into the api.Scaffold type.
// pgtype.UUID.Bytes is the raw 16-byte UUID; Valid is always true for NOT
// NULL columns. pgtype.Timestamptz.Time is the Go time.Time value.
// api.Scaffold.CreatedAt is *time.Time (omitempty); we set it only when valid.
func toAPIScaffold(s generated.Scaffold) api.Scaffold {
	out := api.Scaffold{
		Id:         uuid.UUID(s.ID.Bytes),
		OrgId:      uuid.UUID(s.OrgID.Bytes),
		ExternalId: s.ExternalID,
		Name:       s.Name,
	}
	if s.CreatedAt.Valid {
		t := s.CreatedAt.Time
		out.CreatedAt = &t
	}
	return out
}
