// jobs.go — import_jobs lifecycle (D5-10).
//
// Each handler call (Plan 05-06 BulkImportCatalog method body) does:
//
//  1. createJob — INSERT status='pending' in its own short tx BEFORE
//     chunk 1 opens (so admin GET sees the job mid-import).
//  2. (chunk loop runs — Plan 05-05 owns chunk.go)
//  3. finaliseJob — UPDATE counters + errors JSONB + status=
//     'completed'|'failed' after the last chunk commits.
//
// Both functions take *Importer because they need s.deps.OrgDB (which
// routes the sqlc query through SQLChecker preflight). They are
// unexported because the only legitimate callers are this package's
// own handler methods.
package imports

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// createJob inserts a fresh import_jobs row with status='pending'
// (D5-10 lifecycle step 1). Mints a UUIDv7 for the new row's ID per
// D-19; the row's RETURNING result is intentionally discarded — only
// the input ID is propagated to the chunk loop because the persisted
// counters start at zero and will be re-written by finaliseJob.
//
// idempotencyKey is the persisted client-supplied header value (D5-13).
// nil ⇒ no key supplied; the partial UNIQUE on (org_id, idempotency_key)
// ignores NULL rows so multiple unkeyed POSTs from the same org may
// coexist.
//
//nolint:unused // Reached by Plan 05-06 handler methods.
func (s *Importer) createJob(
	ctx context.Context,
	orgID uuid.UUID,
	entity api.ImportEntityType,
	totalRows int,
	idempotencyKey *string,
) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("imports: mint uuidv7: %w", err)
	}
	q := generated.New(s.deps.OrgDB)
	if _, err := q.InsertImportJob(ctx, generated.InsertImportJobParams{
		ID:             pgUUID(id),
		OrgID:          pgUUID(orgID),
		EntityType:     string(entity),
		TotalRows:      int32(totalRows),
		IdempotencyKey: idempotencyKey,
	}); err != nil {
		return uuid.Nil, fmt.Errorf("imports: insert job: %w", err)
	}
	return id, nil
}

// finaliseJob writes terminal status + counters + errors JSONB after
// the last chunk commits (D5-10 lifecycle step 3). errorsJSON is the
// already-serialised []BulkImportFailedRow body; the caller (chunk.go,
// Plan 05-05) marshals it once at the end of the chunk loop rather
// than per-row to avoid quadratic allocation.
//
// errorsJSON nil ⇒ no errors column update (UPDATE sets it to NULL
// via the []byte=nil branch in pgx). Passing []byte("[]") explicitly
// preserves the "no errors" intent in the persisted row.
//
//nolint:unused // Reached by Plan 05-06 handler methods.
func (s *Importer) finaliseJob(
	ctx context.Context,
	jobID, orgID uuid.UUID,
	status api.ImportJobStatus,
	succeeded, failed int,
	errorsJSON []byte,
) error {
	q := generated.New(s.deps.OrgDB)
	_, err := q.FinaliseImportJob(ctx, generated.FinaliseImportJobParams{
		Status:        string(status),
		SucceededRows: int32(succeeded),
		FailedRows:    int32(failed),
		Errors:        errorsJSON,
		ID:            pgUUID(jobID),
		OrgID:         pgUUID(orgID),
	})
	if err != nil {
		return fmt.Errorf("imports: finalise job: %w", err)
	}
	return nil
}
