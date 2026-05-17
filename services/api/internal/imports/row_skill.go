// row_skill.go — per-row processor for entity=skills (D5-15 / IMP-03).
//
// Flat shape — no nested arrays, no FK probes. The processor:
//
//   1. reMarshalAs into ImportSkillRequest.
//   2. Layer 1 — validateCodeFormat on `code` (D04_1-03 / Pitfall 9).
//   3. qtx.UpsertSkillByCode (Phase 04.1 — INSERT ... ON CONFLICT
//      (org_id, code) DO UPDATE ... RETURNING).
//   4. Wrap pg errors via wrapPgError (catalog.MapPgError reused).
//   5. Return succeededRow on success.
//
// `description` is optional (*string); passed verbatim. The csv path's
// coerce.coerceString never returns an error so this flow is the
// "happy" case for skills with empty descriptions.
package imports

import (
	"context"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// skillRowProc is the rowProcessor implementation for entity=skills.
// Stateless across rows; one instance reused per chunk.
type skillRowProc struct {
	handlers *Importer
}

func (p *skillRowProc) process(
	ctx context.Context,
	sp *db.OrgTx,
	orgID uuid.UUID,
	r parsedRow,
	_ *chunkSkillResolver, // skills entity has no nested array; resolver unused.
) (succeededRow, *rowError) {
	typed, err := reMarshalAs[api.ImportSkillRequest](r.raw)
	if err != nil {
		return succeededRow{}, &rowError{Field: "", Reason: "invalid_json_row"}
	}
	if !catalog.ValidateCodeFormat(typed.Code) {
		return succeededRow{}, &rowError{Field: "code", Reason: "invalid_code_format"}
	}

	id := uuid.Must(uuid.NewV7())
	qtx := generated.New(sp)
	row, upErr := qtx.UpsertSkillByCode(ctx, generated.UpsertSkillByCodeParams{
		ID:          pgUUID(id),
		OrgID:       pgUUID(orgID),
		Code:        typed.Code,
		ExternalID:  typed.ExternalId,
		Name:        typed.Name,
		Description: typed.Description,
		SkillType:   typed.SkillType,
		Enabled:     derefBool(typed.Enabled, true),
	})
	if upErr != nil {
		return succeededRow{}, wrapPgError(upErr, "skill")
	}
	return succeededRow{
		id:     uuid.UUID(row.ID.Bytes),
		entity: api.Skills,
		lineNo: r.lineNo,
	}, nil
}
