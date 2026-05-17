// mappers.go — sqlc-row → wire DTO conversion for import_jobs.
//
// Pure functions only (mirror state/mappers.go). The chunk loop and
// handler methods compose mapImportJob over either generated.ImportJob
// row returned by InsertImportJob / FinaliseImportJob / GetImportJob
// or LookupImportJobByIdempotencyKey.
//
// The pgUUID + ptrToTime helpers duplicate the catalog/state equivalents
// because Phase 5's mappers package is the wrong place to introduce a
// cross-package shared helper (Phase 04.1 left state and catalog with
// independent copies — see state/mappers.go lines 51-53). Flagged as a
// v0.2 refactor candidate.
package imports

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// mapImportJob converts a sqlc-generated import_jobs row into the
// OpenAPI wire DTO. Nullable columns surface as *T per the generated
// type's nullable semantics. Caller is responsible for orgID scoping
// (the row already carries the correct org_id from the WHERE clause).
//
// Errors JSONB column unmarshals to []api.BulkImportFailedRow when
// present and non-empty; on unmarshal failure the field is left nil
// rather than failing the mapping (defensive — a corrupt persisted
// payload should still return a 200 ImportJob with empty errors so
// admins can see the rest of the counters).
func mapImportJob(row generated.ImportJob) api.ImportJob {
	out := api.ImportJob{
		Id:            openapi_types.UUID(row.ID.Bytes),
		OrgId:         openapi_types.UUID(row.OrgID.Bytes),
		EntityType:    api.ImportEntityType(row.EntityType),
		Status:        api.ImportJobStatus(row.Status),
		TotalRows:     int(row.TotalRows),
		SucceededRows: int(row.SucceededRows),
		FailedRows:    int(row.FailedRows),
	}

	if len(row.Errors) > 0 {
		var failed []api.BulkImportFailedRow
		if jerr := json.Unmarshal(row.Errors, &failed); jerr == nil {
			out.Errors = &failed
		}
		// Unmarshal failure: leave out.Errors nil. Defensive — a
		// corrupt JSONB payload should not gate the rest of the
		// ImportJob fields.
	}

	if row.CreatedAt.Valid {
		t := row.CreatedAt.Time
		out.CreatedAt = &t
	}

	return out
}

// pgUUID wraps uuid.UUID into pgtype.UUID for sqlc params. Always
// Valid=true — the import_jobs schema has no nullable UUID columns
// (idempotency_key is text, not uuid; org_id is NOT NULL).
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// ptrToTime extracts a *time.Time from a pgtype.Timestamptz. Returns
// nil when the column is NULL. Exists as a helper here because
// mappers.go callers consistently need to convert Valid pgtype.Timestamptz
// to *time.Time for the OpenAPI wire shape (which uses *time.Time for
// optional timestamps).
//
//nolint:unused // Used by Wave 3 row processors when they land.
func ptrToTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
