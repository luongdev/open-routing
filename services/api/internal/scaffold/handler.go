// Package scaffold owns the throw-away _scaffold CRUD surface that exercises
// the full HTTP -> OrgContext -> OrgDB -> sqlc chain in Phase 1 (D-17, D-18,
// FOUND-02, FOUND-05). Phase 3 deletes this package and table when real
// catalog entities replace it.
//
// The package contract is one exported constructor: Routes(orgDB) returns a
// chi.Router that the server.NewMux mounts at /v1/orgs/{org_id}/_scaffold.
// The URL parameter {org_id} is intentionally unread by code paths — the
// authoritative org_id is read from ctx (FOUND-05) so a hostile client cannot
// drive cross-org behavior by editing the URL.
package scaffold

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"github.com/luongdev/open-routing/services/api/internal/middleware"
)

// Routes returns a chi.Router carrying the three Phase 1 _scaffold routes
// (D-17). The shared *db.OrgDB is constructed once in cmd/api/main.go; each
// request's org_id is read from ctx by orgDB.preflight before any SQL runs,
// so a missing org_id in the request context guarantees orgDB.preflight
// emits ErrOrgIDMissingFromContext rather than falling through to a
// cross-org query.
//
// chi mount pattern: Routes returns a Router (not http.Handler) so the
// caller in server.NewMux can use chi.Router.Mount semantics — preserving
// path stripping and middleware inheritance the way chi expects.
func Routes(orgDB *db.OrgDB) chi.Router {
	h := &handler{orgDB: orgDB}
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	return r
}

// handler holds the shared OrgDB so each HTTP method receiver can construct
// a fresh sqlc *Queries via generated.New(orgDB) per request. Constructing
// Queries is cheap (one struct copy), so we don't memoize at the handler.
type handler struct {
	orgDB *db.OrgDB
}

// createRequest is the POST /v1/orgs/{org_id}/_scaffold body shape (D-17).
// External clients send {"external_id":"...","name":"..."} — these are the
// only writable fields; id, org_id, and created_at are server-controlled.
type createRequest struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
}

// scaffoldResponse is the JSON-stable shape returned by every handler.
//
// We cannot return generated.Scaffold directly because sqlc emitted
// pgtype.UUID / pgtype.Timestamptz fields, which encode as
// {"Bytes":[...],"Valid":true} — useless to API clients. scaffoldResponse
// is the public shape; conversion happens in toResponse below.
type scaffoldResponse struct {
	ID         uuid.UUID `json:"id"`
	OrgID      uuid.UUID `json:"org_id"`
	ExternalID string    `json:"external_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
}

// toResponse converts the sqlc-generated row into the public JSON shape.
// pgtype.UUID.Bytes is the raw 16-byte UUID and Valid is the SQL non-null
// flag; sqlc always returns Valid=true for NOT NULL columns so we trust the
// bytes directly. Same for pgtype.Timestamptz.
func toResponse(s generated.Scaffold) scaffoldResponse {
	return scaffoldResponse{
		ID:         uuid.UUID(s.ID.Bytes),
		OrgID:      uuid.UUID(s.OrgID.Bytes),
		ExternalID: s.ExternalID,
		Name:       s.Name,
		CreatedAt:  s.CreatedAt.Time,
	}
}

// toPgUUID wraps a google/uuid value into pgtype.UUID for sqlc parameter
// passing. Valid=true because every UUID we pass is a real non-null value
// (NOT NULL columns in _scaffold).
func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// create handles POST /v1/orgs/{org_id}/_scaffold (D-17).
//
// Authoritative org_id comes from ctx (FOUND-05, never from the URL). The
// id field is server-minted as UUIDv7 (D-19, project-wide UUIDv7+ rule).
// Request body validation is minimal — Phase 1 ships no validation library.
func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		// Defensive guard: OrgContext middleware guarantees presence inside
		// the /v1 sub-router; this branch only fires if the route is wired
		// outside that scope (a regression we want to surface loudly).
		middleware.WriteError(ctx, w, http.StatusInternalServerError, "internal", "missing_org_id_in_context")
		return
	}

	var body createRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.WriteError(ctx, w, http.StatusBadRequest, "invalid_body", "malformed_json")
		return
	}
	if body.ExternalID == "" || body.Name == "" {
		middleware.WriteError(ctx, w, http.StatusBadRequest, "invalid_body", "external_id_and_name_required")
		return
	}

	q := generated.New(h.orgDB)
	id := uuid.Must(uuid.NewV7()) // D-19 UUIDv7+ everywhere; entropy-exhaustion panic caught by chi.Recoverer
	row, err := q.InsertScaffold(ctx, generated.InsertScaffoldParams{
		ID:         toPgUUID(id),
		OrgID:      toPgUUID(orgID),
		ExternalID: body.ExternalID,
		Name:       body.Name,
	})
	if err != nil {
		middleware.WriteError(ctx, w, http.StatusInternalServerError, "internal", "insert_failed")
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, toResponse(row))
}

// list handles GET /v1/orgs/{org_id}/_scaffold (D-17).
//
// Sqlc's ListScaffolds returns []Scaffold (non-nil empty slice when no
// rows match) so the JSON encoder produces "[]" rather than "null" — no
// special-case needed at the handler.
func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		middleware.WriteError(ctx, w, http.StatusInternalServerError, "internal", "missing_org_id_in_context")
		return
	}
	q := generated.New(h.orgDB)
	rows, err := q.ListScaffolds(ctx, toPgUUID(orgID))
	if err != nil {
		middleware.WriteError(ctx, w, http.StatusInternalServerError, "internal", "list_failed")
		return
	}
	out := make([]scaffoldResponse, 0, len(rows))
	for _, s := range rows {
		out = append(out, toResponse(s))
	}
	middleware.WriteJSON(w, http.StatusOK, out)
}

// get handles GET /v1/orgs/{org_id}/_scaffold/{id} (D-17).
//
// 404 vs 500 split: pgx.ErrNoRows is the sqlc-returned error for :one
// queries with no match — we map that to 404. Any other error is a real
// DB failure and maps to 500. The query body in sqlc filters by both
// id AND org_id so a cross-org probe (correct id, wrong org) returns
// the same 404 as a genuinely-missing id (FOUND-08 leakage proof relies
// on this — Plan 07's testcontainer suite asserts the cross-org GET-by-id
// case returns 404 not 200).
func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		middleware.WriteError(ctx, w, http.StatusInternalServerError, "internal", "missing_org_id_in_context")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		middleware.WriteError(ctx, w, http.StatusBadRequest, "invalid_id", "malformed_uuid")
		return
	}
	q := generated.New(h.orgDB)
	row, err := q.GetScaffoldByID(ctx, generated.GetScaffoldByIDParams{
		ID:    toPgUUID(id),
		OrgID: toPgUUID(orgID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			middleware.WriteError(ctx, w, http.StatusNotFound, "not_found", "no_such_scaffold")
			return
		}
		middleware.WriteError(ctx, w, http.StatusInternalServerError, "internal", "get_failed")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, toResponse(row))
}
