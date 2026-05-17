// idempotency.go — D5-13 replay lookup.
//
// Idempotency-Key contract (RFC-style draft-ietf-httpapi-idempotency-key):
// the client mints a UUIDv7 and sends it in the Idempotency-Key header.
// On the server side:
//
//  1. lookupIdempotentReplay — handler probes this BEFORE InsertImportJob.
//     Hit ⇒ persisted job re-serialised + idempotent_replay=true returned;
//     do NOT re-import. Miss ⇒ proceed normally, persist key on the new
//     row via the createJob INSERT.
//  2. rehydrateBulkImportResult — pure conversion from the persisted
//     generated.ImportJob row to the wire BulkImportResult shape.
//
// # KNOWN LIMITATION (D5-13 + D5-27)
//
// The persisted import_jobs row stores counters + errors JSONB but NOT
// the full succeeded[] UUID list. A replay response therefore CANNOT
// reconstruct the original `succeeded` UUIDs. v0.1 returns:
//
//	{
//	  "succeeded": [],
//	  "failed":    [...as stored...],
//	  "idempotent_replay": true
//	}
//
// Callers detect a replay via the boolean and rely on `failed[]` for
// forensics. Future v0.2 work (deferred per 05-CONTEXT.md §Deferred):
// persist `succeeded_ids JSONB` on import_jobs so replays can return
// the full UUID set.
package imports

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// lookupIdempotentReplay probes the import_jobs table for a prior row
// keyed by (org_id, idempotency_key). On hit, returns the rehydrated
// BulkImportResult with idempotent_replay=true. On miss
// (pgx.ErrNoRows), returns (zero, false, nil) — the handler proceeds
// with a fresh InsertImportJob.
//
// Real DB errors propagate with the (false, err) shape so the handler
// can map them to HTTP 500.
//
//nolint:unused // Reached by Plan 05-06 handler methods.
func (s *Importer) lookupIdempotentReplay(
	ctx context.Context,
	orgID uuid.UUID,
	key openapi_types.UUID,
) (api.BulkImportCatalog200JSONResponse, bool, error) {
	q := generated.New(s.deps.OrgDB)
	keyStr := uuid.UUID(key).String()
	prior, err := q.LookupImportJobByIdempotencyKey(ctx, generated.LookupImportJobByIdempotencyKeyParams{
		OrgID:          pgUUID(orgID),
		IdempotencyKey: &keyStr,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.BulkImportCatalog200JSONResponse{}, false, nil
	}
	if err != nil {
		return api.BulkImportCatalog200JSONResponse{}, false, fmt.Errorf("imports: lookup idempotent replay: %w", err)
	}
	return rehydrateBulkImportResult(prior, true), true, nil
}

// rehydrateBulkImportResult builds a 200 response from a persisted
// import_jobs row. The shape mirrors what the handler would have
// returned at the time of the original import:
//
//   - succeeded: empty (KNOWN LIMITATION — see file header doc comment).
//   - failed:    decoded from the persisted Errors JSONB column.
//   - idempotent_replay: the `replay` bool (set true by
//     lookupIdempotentReplay, false by any future direct caller).
//
// JSONB decode failure on the persisted Errors column is treated
// defensively: failed stays nil, the rest of the response still
// returns. This matches mapImportJob's defensive posture (a corrupt
// persisted payload should not return 5xx; admins can re-import).
func rehydrateBulkImportResult(row generated.ImportJob, replay bool) api.BulkImportCatalog200JSONResponse {
	job := mapImportJob(row)

	var failed []api.BulkImportFailedRow
	if job.Errors != nil {
		failed = *job.Errors
	}

	r := replay
	return api.BulkImportCatalog200JSONResponse(api.BulkImportResult{
		Succeeded:        []api.UUIDv7{},
		Failed:           failed,
		IdempotentReplay: &r,
	})
}
