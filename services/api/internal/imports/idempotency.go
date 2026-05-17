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
//
// # Phase 5 fix M5 — replay status code mirrors original
//
// Pre-fix rehydrateBulkImportResult was hardcoded to return
// BulkImportCatalog200JSONResponse for every replay. A replay of a job
// that originally returned 207 (partial) or 422 (all-failed)
// misrepresented the outcome to the client, masking the semantic
// failure on retry. The fix derives the replay status from the
// persisted SucceededRows / FailedRows counters:
//
//   - succeeded > 0 && failed == 0 → 200
//   - succeeded > 0 && failed > 0  → 207
//   - succeeded == 0 && failed > 0 → 422
//   - succeeded == 0 && failed == 0 → 200 (vacuous-empty case mirrors
//     the live-handler emptyResultResponse fall-through)
//
// Note: the persisted row also has a `status` column (pending |
// completed | failed). On a replay we are reading a TERMINAL row
// (the partial UNIQUE on idempotency_key only matches rows the
// finalise step wrote to; pending rows from in-flight requests are
// not yet keyed) — so deriving status from counters matches the
// original handler's switch at runImportPipeline's tail.
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
// response (matching the original status 200/207/422) with
// idempotent_replay=true. On miss (pgx.ErrNoRows), returns
// (nil, false, nil) — the handler proceeds with a fresh
// InsertImportJob.
//
// Real DB errors propagate with the (nil, false, err) shape so the
// handler can map them to HTTP 500.
//
// Phase 5 fix M5: pre-fix this returned BulkImportCatalog200JSONResponse
// directly; the return type is now the interface so the replay can
// surface as 207 (partial) or 422 (all-failed) matching the original
// outcome.
//
//nolint:unused // Reached by Plan 05-06 handler methods.
func (s *Importer) lookupIdempotentReplay(
	ctx context.Context,
	orgID uuid.UUID,
	key openapi_types.UUID,
) (api.BulkImportCatalogResponseObject, bool, error) {
	q := generated.New(s.deps.OrgDB)
	keyStr := uuid.UUID(key).String()
	prior, err := q.LookupImportJobByIdempotencyKey(ctx, generated.LookupImportJobByIdempotencyKeyParams{
		OrgID:          pgUUID(orgID),
		IdempotencyKey: &keyStr,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("imports: lookup idempotent replay: %w", err)
	}
	return rehydrateBulkImportResult(prior, true), true, nil
}

// rehydrateBulkImportResult builds the replay response from a persisted
// import_jobs row. Phase 5 fix M5: the return type is now the
// BulkImportCatalogResponseObject interface so the status code can be
// 200 / 207 / 422 to match the original import's outcome.
//
//   - succeeded: empty (KNOWN LIMITATION — see file header doc comment).
//   - failed:    decoded from the persisted Errors JSONB column.
//   - idempotent_replay: the `replay` bool (set true by
//     lookupIdempotentReplay, false by any future direct caller).
//
// Status selection mirrors runImportPipeline's tail switch — see the
// file-header §M5 comment for the truth table.
//
// JSONB decode failure on the persisted Errors column is treated
// defensively: failed stays nil, the rest of the response still
// returns. This matches mapImportJob's defensive posture (a corrupt
// persisted payload should not return 5xx; admins can re-import).
func rehydrateBulkImportResult(row generated.ImportJob, replay bool) api.BulkImportCatalogResponseObject {
	job := mapImportJob(row)

	var failed []api.BulkImportFailedRow
	if job.Errors != nil {
		failed = *job.Errors
	}
	if failed == nil {
		failed = []api.BulkImportFailedRow{}
	}

	r := replay
	result := api.BulkImportResult{
		Succeeded:        []api.UUIDv7{},
		Failed:           failed,
		IdempotentReplay: &r,
	}

	// M5 fix — choose the wrapper class based on the persisted counters
	// so the replay status code mirrors the original outcome.
	succeeded := int(row.SucceededRows)
	failedCount := int(row.FailedRows)
	switch {
	case succeeded > 0 && failedCount == 0:
		return api.BulkImportCatalog200JSONResponse(result)
	case succeeded > 0 && failedCount > 0:
		return api.BulkImportCatalog207JSONResponse(result)
	case succeeded == 0 && failedCount > 0:
		return api.BulkImportCatalog422JSONResponse(result)
	default:
		// Vacuous empty (zero-rows-after-header) → 200. Matches the live
		// handler's emptyResultResponse fall-through.
		return api.BulkImportCatalog200JSONResponse(result)
	}
}
