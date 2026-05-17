// row_channel.go — per-row processor for entity=channels (D-76 / IMP-03).
//
// Special case: channels carry an optional FK `default_queue_id` that
// the import surface references as `default_queue_code`. The processor
// resolves the code → uuid inside the savepoint Tx via Phase 04.1's
// GetQueueByCode so:
//
//   - missing queue → row-level invalid_reference failure (the channel
//     does NOT land; ROLLBACK TO savepoint).
//   - cross-org queue → ErrNoRows (GetQueueByCode has org_id=$1) →
//     same invalid_reference path (FOUND-08 — no oracle for which org).
//   - happy path → resolved queueID passes to UpsertChannelByCode as
//     pgtype.UUID{Valid:true}; nil DefaultQueueCode passes through as
//     pgtype.UUID{Valid:false} → SQL NULL.
//
// The FK probe runs inside the SAVEPOINT Tx so it sees uncommitted
// same-chunk writes (a queue inserted earlier in this chunk's loop
// is visible to a channel inserted later — D-76 same-tx visibility).
package imports

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/catalog"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

type channelRowProc struct {
	handlers *Importer
}

func (p *channelRowProc) process(
	ctx context.Context,
	sp *db.OrgTx,
	orgID uuid.UUID,
	r parsedRow,
	_ *chunkSkillResolver,
) (succeededRow, *rowError) {
	typed, err := reMarshalAs[api.ImportChannelRequest](r.raw)
	if err != nil {
		return succeededRow{}, &rowError{Field: "", Reason: "invalid_json_row"}
	}
	if !catalog.ValidateCodeFormat(typed.Code) {
		return succeededRow{}, &rowError{Field: "code", Reason: "invalid_code_format"}
	}

	// Bind to the savepoint Tx — the GetQueueByCode probe MUST see
	// uncommitted same-chunk writes.
	qtx := generated.New(sp)

	// Optional FK: resolve default_queue_code if provided. D04_1-03
	// regex applies to the FK target's code too (Pitfall 9).
	defaultQueueID := pgtype.UUID{Valid: false}
	if typed.DefaultQueueCode != nil && *typed.DefaultQueueCode != "" {
		dqCode := *typed.DefaultQueueCode
		if !catalog.ValidateCodeFormat(dqCode) {
			return succeededRow{}, &rowError{
				Field:  "default_queue_code",
				Reason: "invalid_code_format",
			}
		}
		queue, qErr := qtx.GetQueueByCode(ctx, generated.GetQueueByCodeParams{
			OrgID: pgUUID(orgID),
			Code:  dqCode,
		})
		if errors.Is(qErr, pgx.ErrNoRows) {
			return succeededRow{}, &rowError{
				Field:  "default_queue_code",
				Reason: "invalid_reference",
			}
		}
		if qErr != nil {
			p.handlers.deps.Logger.WarnContext(ctx, "import.channel.queue_probe_failed",
				"default_queue_code", dqCode, "err", qErr)
			return succeededRow{}, &rowError{Field: "default_queue_code", Reason: "queue_probe_failed"}
		}
		defaultQueueID = queue.ID
	}

	id := uuid.Must(uuid.NewV7())
	row, upErr := qtx.UpsertChannelByCode(ctx, generated.UpsertChannelByCodeParams{
		ID:             pgUUID(id),
		OrgID:          pgUUID(orgID),
		Code:           typed.Code,
		ExternalID:     typed.ExternalId,
		Name:           typed.Name,
		ChannelType:    typed.ChannelType,
		DefaultQueueID: defaultQueueID,
		Enabled:        derefBool(typed.Enabled, true),
	})
	if upErr != nil {
		return succeededRow{}, wrapPgError(upErr, "channel")
	}
	return succeededRow{
		id:     uuid.UUID(row.ID.Bytes),
		entity: api.Channels,
		lineNo: r.lineNo,
	}, nil
}
