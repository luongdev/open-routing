// handler_get_job.go — Wave 4 controller for the GetImportJob
// strict-server method (IMP-06 read-side, RESEARCH §Pattern 1 step 9).
//
// Reads import_jobs by (id, org_id) and maps the sqlc-generated row to
// the wire ImportJob shape via mapImportJob. The composite WHERE clause
// in the GetImportJob SQL is the FOUND-08 isolation seam — a cross-org
// probe returns pgx.ErrNoRows which we surface as 404 import_job_not_found,
// never as 403, so the existence of another org's job is never leaked.
//
// No cache wrap (contrast with state.GetAgentStatus): import_jobs is
// monotonic event data and reads are rare. The 60s catalog cache TTL
// pattern is not justified here.
//
// Analog: services/api/internal/state/agent_states.go GetAgentStatus
// (lines 40-99) — same orgID extract → DB lookup → ErrNoRows-to-404
// mapping, but without the cache layer.
package imports

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// GetImportJob implements the api.StrictServerInterface method of the
// same name on *Importer. Returns 200 on hit, 404 on miss (including
// cross-org probe — FOUND-08), 500 on internal failure.
//
// Returns nil for the error value on every branch — the strict-server
// pipeline writes the typed *JSONResponse to the wire; a non-nil error
// would bypass that and produce a generic 500.
func (s *Importer) GetImportJob(
	ctx context.Context,
	req api.GetImportJobRequestObject,
) (api.GetImportJobResponseObject, error) {
	// Cross-Cutting Pattern 1: orgID extract. The /v1/* OrgContext
	// middleware populates the ctx; a request that reaches the handler
	// without it is a wiring bug, not user error.
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.GetImportJob500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "missing_org_id_in_context",
			},
		}, nil
	}

	jobID := uuid.UUID(req.ImportId)

	q := generated.New(s.deps.OrgDB)
	row, err := q.GetImportJob(ctx, generated.GetImportJobParams{
		ID:    pgUUID(jobID),
		OrgID: pgUUID(orgID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// FOUND-08 isolation: cross-org probe also lands here because
		// the composite (id, org_id) WHERE clause returns ErrNoRows when
		// the row belongs to a different org. NEVER 403 — that would
		// leak the existence of other-org rows.
		return api.GetImportJob404JSONResponse{
			NotFoundJSONResponse: api.NotFoundJSONResponse{
				Error:  api.ErrorCodeNotFound,
				Reason: "import_job_not_found",
			},
		}, nil
	}
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "get import job",
			"job_id", jobID, "org_id", orgID, "err", err)
		return api.GetImportJob500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "import_job_lookup_failed",
			},
		}, nil
	}

	return api.GetImportJob200JSONResponse(mapImportJob(row)), nil
}
