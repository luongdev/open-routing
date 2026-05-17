// errors.go — per-row error wrapping.
//
// The constraint-name introspection (catalog/errors.go MapPgError) is
// REUSED via the export added in Plan 05-02; Phase 5 wraps the verdict
// into the row-level shape expected by BulkImportFailedRow.
//
// rowError is the chunk loop's internal per-row failure shape. It is
// converted to api.BulkImportFailedRow in chunk.go (Plan 05-05) when
// the final BulkImportResult is assembled.
//
// Threat T-05-04-02 (Information Disclosure): rowError.Reason MUST
// NEVER echo raw row values. wrapPgError satisfies this by emitting
// `<code>:<constraint-detail>` (e.g. `duplicate_code:agents:emp_001`)
// where the constraint detail is the constraint name plus any
// validator-friendly hint — never the user's input cell content.
// Phase 2 OQ-2A locked this contract on BulkImportFailedRow.
package imports

import (
	"github.com/luongdev/open-routing/services/api/internal/catalog"
)

// rowError is the chunk loop's per-row failure shape. The Field is
// the empty string when the error is row-level (e.g. tx commit
// failure); non-empty when field-level (e.g. `skills[2].skill_code`).
//
// The conversion to api.BulkImportFailedRow happens in chunk.go
// (Plan 05-05) — that function fills in the 1-based row index and
// the `error: import_failed` constant.
type rowError struct {
	Field  string
	Reason string
}

// wrapPgError converts the catalog.MapPgError triple into a rowError.
// Drops the HTTP status (Phase 5 is row-level; the batch-level HTTP
// status is decided by the chunk loop based on succeeded vs failed
// counts — D-37).
//
// Reason format: `<code>:<constraint-detail>`. The code half is the
// closed enum from api.ErrorCode (machine-readable); the
// constraint-detail half is whatever MapPgError emitted as the third
// triple element (`duplicate_code`, `fk_violation`, etc.). Joining
// them with `:` preserves both signals in a single string field while
// staying schema-stable — `failed[i].reason` is documented as
// human-readable text in the OpenAPI spec.
func wrapPgError(err error, entity string) *rowError {
	_, code, reason := catalog.MapPgError(err, entity)
	return &rowError{Field: "", Reason: string(code) + ":" + reason}
}
