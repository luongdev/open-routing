// errors.go — pgx/pgconn error → strict-server response triple.
//
// Every catalog handler that touches the DB calls mapPgError on the
// non-happy-path return to translate pgx errors into the locked
// (httpStatus, ErrorCode, reason) triple, then wraps the triple in the
// operation-specific *JSONResponse type.
//
// Translations:
//   - pgx.ErrNoRows          → 404 / not_found         / "{entity}_not_found"
//   - pgconn 23505 (UNIQUE)  → 409 / version_conflict  / "external_id_collision"
//   - pgconn 23503 (FK)      → 422 / invalid_reference / "fk_violation"  (Wave 3 codex review)
//   - pgconn 23514 (CHECK)   → 422 / invalid_value     / "check_violation" (Wave 3 codex review)
//   - everything else        → 500 / internal          / "internal"
//
// 23505 is mapped to version_conflict because the catalog schema's
// only application-level uniqueness constraints are external_id-style.
// 23503 + 23514 paths are needed for Wave 4-5 entity handlers (e.g.,
// channels.default_queue_id FK probe via D-76; agent_skills proficiency
// CHECK as defense-in-depth backstop) — adding now so handler authors
// don't have to re-introduce the mappings.
package catalog

import (
	"errors"
	"net/http"

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
			return http.StatusConflict, api.ErrorCodeVersionConflict, "external_id_collision"
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
