// errors.go — pgx/pgconn error → strict-server response triple.
//
// Every catalog handler that touches the DB calls mapPgError on the
// non-happy-path return to translate pgx errors into the locked
// (httpStatus, ErrorCode, reason) triple, then wraps the triple in the
// operation-specific *JSONResponse type.
//
// Translations:
//   - pgx.ErrNoRows          → 404 / not_found         / "{entity}_not_found"
//   - pgconn 23505 (UNIQUE)  → 409 / duplicate_code OR duplicate_external_id (Phase 04.1 — see below)
//   - pgconn 23503 (FK)      → 422 / invalid_reference / "fk_violation"  (Wave 3 codex review)
//   - pgconn 23514 (CHECK)   → 422 / invalid_value     / "check_violation" (Wave 3 codex review)
//   - everything else        → 500 / internal          / "internal"
//
// Phase 04.1 amendment (D04_1-21): 23505 now introspects pgErr.ConstraintName
// to distinguish duplicate_code (suffix _org_id_code_key) from
// duplicate_external_id (prefix ix_, suffix _org_external_id). The previous
// Phase 3 mapping returned `version_conflict` for every 23505 regardless of
// which constraint fired — flagged MED by 03-SIMPLICITY-REVIEW.md ("pg error
// mapping hides entity-specific conflicts") and is fixed here as a side
// effect.
//
// 23503 + 23514 paths are needed for Wave 4-5 entity handlers (e.g.,
// channels.default_queue_id FK probe via D-76; agent_skills proficiency
// CHECK as defense-in-depth backstop) — adding now so handler authors
// don't have to re-introduce the mappings.
package catalog

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// mapPgError translates a sqlc/pgx-returned error into the canonical
// (httpStatus, ErrorCode, reason) triple. The handler then wraps the
// triple in the operation-specific *JSONResponse type — this function
// stays response-type-agnostic so it can be reused across all 6 entities.
//
// `entity` is the short slug ("agent", "skill", etc.) used to construct
// the 404 reason ("{entity}_not_found"). It is NOT used for any other
// branch — keeping the call sites uniform.
func mapPgError(err error, entity string) (int, api.ErrorCode, string) {
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound, api.ErrorCodeNotFound, entity + "_not_found"
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			// Phase 04.1 (D04_1-21): introspect pgErr.ConstraintName so the
			// handler-side 409 wire shape distinguishes "another row already
			// owns this code" from "another row already binds this external_id".
			// Plan 01 locked the constraint names: composite UNIQUE on
			// (org_id, code) auto-names `<entity>_org_id_code_key`; the partial
			// unique index on (org_id, external_id) is explicitly named
			// `ix_<entity>_org_external_id`.
			name := pgErr.ConstraintName
			switch {
			case strings.HasSuffix(name, "_org_id_code_key"):
				return http.StatusConflict, api.ErrorCodeDuplicateCode, "duplicate_code"
			case strings.HasPrefix(name, "ix_") && strings.HasSuffix(name, "_org_external_id"):
				return http.StatusConflict, api.ErrorCodeDuplicateExternalId, "duplicate_external_id"
			default:
				// Defensive — a future constraint we forgot to map. Return 409
				// with a generic reason so clients still see "4xx, retry with
				// new values" instead of 5xx. The constraint name is in the
				// reason for log-grep / observability.
				return http.StatusConflict, api.ErrorCodeInvalidBody, "unique_violation:" + name
			}
		case "23503":
			// FK violation. The handler-level D-76 probes (e.g.,
			// QueueExistsAndEnabledInOrg) catch most cases before the
			// INSERT, but this stays as a defense-in-depth net for
			// races where the referenced row is deleted between probe
			// and INSERT (Wave 3 codex review iter 1).
			return http.StatusUnprocessableEntity, api.ErrorCodeInvalidReference, "fk_violation"
		case "23514":
			// CHECK constraint violation. The handler-level proficiency
			// validation (1..10) catches the common case before the
			// INSERT, but this protects against any future schema
			// CHECKs (Wave 3 codex review iter 1).
			return http.StatusUnprocessableEntity, api.ErrorCodeInvalidValue, "check_violation"
		}
	}
	return http.StatusInternalServerError, api.ErrorCodeInternal, "internal"
}
