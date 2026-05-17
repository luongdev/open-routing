// chunk.go — chunked-savepoint orchestrator (D5-09).
//
// One chunk == 50 rows (handlers.go chunkSize). The orchestrator opens
// ONE outer Tx via OrgDB.BeginTx, runs ResolveSkillCodes ONCE for the
// chunk to satisfy Pitfall 6 (N+1 avoidance), then loops over rows with
// per-row Savepoints driven by Plan 05-02's (*OrgTx).BeginSavepoint:
//
//   1. outerTx.Begin → opens a single chunk Tx.
//   2. resolveChunkSkillCodes → ONE ResolveSkillCodes call (only for
//      agent entity; other entities have no nested skill arrays).
//   3. per-row: sp.Begin → rowProc.process → sp.Commit (RELEASE) on
//      success / sp.Rollback (ROLLBACK TO) on rowError.
//   4. outerTx.Commit. On commit failure ALL chunk's `succeeded` rows
//      transfer to `failed[]` with reason `chunk_commit_failed`
//      (D-37 chunk atomicity wrt persistence).
//   5. POST-COMMIT only: cache.Del per succeeded row (Pitfall 4 —
//      NEVER inside the savepoint scope, NEVER before commit).
//
// Anti-pattern enforcement (RESEARCH §Anti-Patterns + Pitfall 4 + 8):
//
//   - All per-row queries route through generated.New(sp) — NEVER
//     generated.New(s.deps.OrgDB) (would short-circuit the savepoint).
//   - cache.Del fires AFTER outerTx.Commit succeeds, NEVER per-row or
//     before commit. A Del failure logs warn but never fails the chunk.
//   - catalog's Phase 3 PUT-style replace helper is NEVER called from
//     this package — D5-18 mandates MERGE via MergeAgentSkill.
package imports

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/cache"
	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// parsedRow is one record after the JSON or CSV parser has produced
// either a typed []interface{} item (`raw` set, `cells` nil) or a
// coerced cell map (`cells` set, `raw` set to the materialised typed
// shape). The row processor reads `raw` and re-marshals into the typed
// Import*Request shape; the CSV path materialises `raw` from `cells`
// in Plan 05-06 dispatch.
//
// lineNo is the 1-based row index used in BulkImportFailedRow.Row (the
// JSON path counts array items; the CSV path counts data rows after
// the header). The chunk loop NEVER renumbers — the parser is the
// single source of truth for line numbers (Phase 5 OQ-2A).
//
// coerceFailure is set by the CSV path's per-cell coerce callback when
// a cell value fails coercion (D5-01..D5-08 sentinels). The chunk loop
// surfaces these as per-row failures BEFORE entering a savepoint so
// the row never reaches the row processor. JSON path leaves this nil
// (per-row re-marshal happens inside the row processor under savepoint).
type parsedRow struct {
	lineNo        int
	raw           interface{}
	cells         map[string]any
	coerceFailure *rowError
}

// succeededRow is the chunk loop's per-row success shape. Captured by
// the row processor, accumulated in chunk.processChunk, finally drained
// into the response's BulkImportResult.Succeeded slice (after the
// outer Tx commits) by the Plan 05-06 handler.
//
// entity is the api.ImportEntityType so the post-commit cache.Del can
// build the correct `or:{orgID}:{entity}:{id}` key (D-58); each row
// processor sets it to the entity it owns.
type succeededRow struct {
	id     uuid.UUID
	entity api.ImportEntityType
	lineNo int
}

// chunkSkillResolver caches the (skill_code → skill UUID) mapping for
// ONE chunk. Constructed once by resolveChunkSkillCodes; consumed by
// the agent row processor via resolveAll. Other entities never touch
// the resolver (they have no nested skill arrays in v0.1 per D5-20).
//
// callCount is a test-visibility counter — every resolveAll call
// increments it so chunk_test.go can assert exactly one ResolveSkillCodes
// round-trip per chunk (Pitfall 6 / T-05-05-06 mitigation). Production
// code never inspects it.
type chunkSkillResolver struct {
	byCode    map[string]uuid.UUID
	callCount int
}

// resolveAll returns the UUIDs for the requested skill codes alongside
// the indexes of any codes that were NOT resolved (i.e. unknown skills).
//
// The first-encountered missing code's index is what the agent row
// processor uses to build the `skills[N].skill_code` field on the
// rowError per D5-16. Index ordering is preserved from the caller's
// `codes` slice so the field index in the error message matches the
// admin's original payload position.
//
// callCount increments on every invocation — test fixtures use it to
// verify Pitfall 6 (N+1 avoidance) by asserting it equals 1 after a
// chunk runs (one batched lookup at resolveChunkSkillCodes is the
// expected count; per-row resolveAll uses the cached map and does not
// re-hit the DB).
func (r *chunkSkillResolver) resolveAll(codes []string) (uuids map[string]uuid.UUID, missing []int) {
	r.callCount++
	if len(codes) == 0 {
		return nil, nil
	}
	out := make(map[string]uuid.UUID, len(codes))
	for i, c := range codes {
		if id, ok := r.byCode[c]; ok {
			out[c] = id
		} else {
			missing = append(missing, i)
		}
	}
	return out, missing
}

// rowProcessor is the per-entity contract the chunk loop invokes. Each
// row_<entity>.go in Task 2 implements this with a small struct that
// captures the *Importer (for the deps.Logger when an Adapter Config
// marshal failure needs to be logged inside the per-row context).
//
// Contract:
//
//   - process MUST NOT call sp.Commit / sp.Rollback / outerTx.Commit /
//     outerTx.Rollback. Savepoint lifecycle is OWNED by the chunk loop.
//   - process MUST bind generated.New to the supplied *db.OrgTx (the
//     savepoint), NEVER to the parent *db.OrgDB.
//   - process returns (succeededRow{}, *rowError) on failure so the
//     chunk loop ROLLBACK TO's the savepoint and continues to row+1.
//   - process returns (succeededRow{...}, nil) on success so the chunk
//     loop RELEASEs the savepoint and accumulates the row in the
//     chunk-level succeeded slice.
//   - resolver is non-nil for the agent entity; nil for every other
//     entity (chunk.processChunk passes nil when the entity is not
//     agents, and ResolveSkillCodes is never called for those chunks).
type rowProcessor interface {
	process(ctx context.Context, sp *db.OrgTx, orgID uuid.UUID, r parsedRow, resolver *chunkSkillResolver) (succeededRow, *rowError)
}

// resolveChunkSkillCodes gathers all unique skill_code values across
// the chunk's `parsedRow.raw` items (agent entity only) and runs ONE
// qtx.ResolveSkillCodes call against the supplied savepoint-or-outer
// Tx. Returns a populated *chunkSkillResolver — callers pass it on to
// rowProcessor.process so per-row code → uuid lookups are O(1) map
// reads instead of per-row DB round-trips (T-05-05-06 / Pitfall 6).
//
// Pre-condition: tx is a live OrgTx (the chunk's outer Tx). The query
// runs through SQLChecker preflight (ResolveSkillCodes has the
// `org_id = $1` literal) — no bypass needed.
//
// Non-agent entities: rows are scanned but no codes are gathered. The
// returned resolver has an empty byCode map; agent row processor is
// never invoked for those chunks so resolveAll is never called either.
//
// Errors propagate verbatim — the caller (processChunk) marks every
// row in the chunk as `skill_resolve_failed` so admins see a clear
// signal that the chunk's nested-skill resolution step failed rather
// than per-row "unknown_skill" misfires.
func (s *Importer) resolveChunkSkillCodes(
	ctx context.Context,
	tx *db.OrgTx,
	orgID uuid.UUID,
	rows []parsedRow,
) (*chunkSkillResolver, error) {
	resolver := &chunkSkillResolver{byCode: map[string]uuid.UUID{}}

	// Collect unique skill_code strings from every agent row's nested
	// skills array. The JSON path's `raw` is map[string]interface{} for
	// each ImportAgentRequest; CSV path produces a SkillToken slice
	// before reaching this function (parser_csv coerceSkillsCell
	// already validates token shape). For the JSON path we re-marshal
	// to peek at typed.Skills without committing to row validation —
	// that is the row processor's job.
	codeSet := make(map[string]struct{})
	for _, r := range rows {
		// Skip rows whose `raw` is not a map — defensive: a parser
		// error would have surfaced as a per-row failure before chunk
		// loop runs, but the chunk processor stays resilient against
		// future parser shape changes.
		typed, err := reMarshalAs[api.ImportAgentRequest](r.raw)
		if err != nil {
			// Skip — per-row decode error will surface in the row
			// processor's reMarshalAs call with a clear field/reason.
			// Skipping here keeps the resolver path narrow.
			continue
		}
		if typed.Skills == nil {
			continue
		}
		for _, sk := range *typed.Skills {
			if sk.SkillCode == "" {
				continue
			}
			codeSet[sk.SkillCode] = struct{}{}
		}
	}
	if len(codeSet) == 0 {
		// No skills in this chunk — resolver stays empty; row processor
		// will pass an empty `codes` slice into resolveAll and get an
		// empty map back, signalling "no skills to merge."
		return resolver, nil
	}

	// Flatten the set into the slice ResolveSkillCodes expects.
	codes := make([]string, 0, len(codeSet))
	for c := range codeSet {
		codes = append(codes, c)
	}

	// ONE DB round-trip per chunk. The query has `WHERE org_id = $1
	// AND code = ANY($2::text[])` — SQLChecker preflight green.
	q := generated.New(tx)
	resolved, err := q.ResolveSkillCodes(ctx, generated.ResolveSkillCodesParams{
		OrgID:   pgUUID(orgID),
		Column2: codes,
	})
	if err != nil {
		return nil, fmt.Errorf("imports: resolve skill codes: %w", err)
	}
	for _, r := range resolved {
		resolver.byCode[r.Code] = uuid.UUID(r.ID.Bytes)
	}
	return resolver, nil
}

// processChunk runs the 5-step pipeline documented at the top of this
// file. Inputs:
//
//   - orgID — the request org (extracted from ctx upstream; passed
//     explicitly so the row processor doesn't have to re-call
//     orgkey.OrgIDFromContext per row).
//   - entity — the api.ImportEntityType being imported; controls whether
//     resolveChunkSkillCodes is invoked (agents only) and what entity
//     the post-commit cache.Del targets.
//   - rowProc — the per-entity rowProcessor implementation (one of the
//     6 row_<entity>.go structs).
//   - rows — the parsed rows for THIS chunk (≤ chunkSize = 50 per D5-09).
//
// Outputs:
//
//   - succeeded — rows that landed (chunk Tx committed). The Plan 05-06
//     handler concatenates these across chunks to build
//     BulkImportResult.Succeeded.
//   - failed — rows that did NOT land (savepoint rolled back; chunk
//     commit failed; outer tx begin failed). Plan 05-06 handler
//     concatenates these to build BulkImportResult.Failed.
//
// On outer Tx commit failure ALL chunk succeeded rows transfer to
// failed[] with reason `chunk_commit_failed` and (nil, failed) is
// returned — admin sees "nothing landed for this chunk" rather than
// "some random subset survived." Chunk atomicity wrt persistence is
// the D5-09 contract.
func (s *Importer) processChunk(
	ctx context.Context,
	orgID uuid.UUID,
	entity api.ImportEntityType,
	rowProc rowProcessor,
	rows []parsedRow,
) (succeeded []succeededRow, failed []api.BulkImportFailedRow) {
	if len(rows) == 0 {
		return nil, nil
	}

	outerTx, err := s.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		// Outer Tx begin failed — every row in this chunk is unrecoverable.
		s.deps.Logger.WarnContext(ctx, "import.chunk.tx_begin_failed", "err", err)
		for _, r := range rows {
			failed = append(failed, api.BulkImportFailedRow{
				Row:    r.lineNo,
				Error:  api.BulkImportFailedRowErrorImportFailed,
				Reason: "tx_begin_failed",
			})
		}
		return nil, failed
	}
	// Safe after commit — pgx ignores Rollback on a closed Tx (Pitfall 8).
	defer func() { _ = outerTx.Rollback(ctx) }()

	// Resolve skill codes ONCE per chunk for agent entity (Pitfall 6).
	// Other entities pass nil resolver since they have no nested skills
	// (D5-20 — flat in v0.1).
	var resolver *chunkSkillResolver
	if entity == api.Agents {
		r, rErr := s.resolveChunkSkillCodes(ctx, outerTx, orgID, rows)
		if rErr != nil {
			s.deps.Logger.WarnContext(ctx, "import.chunk.skill_resolve_failed", "err", rErr)
			for _, r := range rows {
				failed = append(failed, api.BulkImportFailedRow{
					Row:    r.lineNo,
					Error:  api.BulkImportFailedRowErrorImportFailed,
					Reason: "skill_resolve_failed",
				})
			}
			return nil, failed
		}
		resolver = r
	}

	// Per-row savepoint loop. Each iteration is independent — a failed
	// row ROLLBACK TO's its own savepoint without affecting siblings.
	for _, r := range rows {
		sp, spErr := outerTx.BeginSavepoint(ctx)
		if spErr != nil {
			// Savepoint begin failure is rare — the outer Tx may be in
			// a degraded state. Mark the row failed and continue; the
			// next BeginSavepoint will return the same error and the
			// remaining rows will join the failed[] tally.
			failed = append(failed, api.BulkImportFailedRow{
				Row:    r.lineNo,
				Error:  api.BulkImportFailedRowErrorImportFailed,
				Reason: "savepoint_begin_failed",
			})
			continue
		}

		out, procErr := rowProc.process(ctx, sp, orgID, r, resolver)
		if procErr != nil {
			// ROLLBACK TO savepoint — per-row failure leaves the outer
			// Tx healthy for the next row.
			_ = sp.Rollback(ctx)
			field := procErr.Field
			row := api.BulkImportFailedRow{
				Row:    r.lineNo,
				Error:  api.BulkImportFailedRowErrorImportFailed,
				Reason: procErr.Reason,
			}
			if field != "" {
				row.Field = &field
			}
			failed = append(failed, row)
			continue
		}

		// RELEASE savepoint — per-row success.
		if commitErr := sp.Commit(ctx); commitErr != nil {
			failed = append(failed, api.BulkImportFailedRow{
				Row:    r.lineNo,
				Error:  api.BulkImportFailedRowErrorImportFailed,
				Reason: "savepoint_commit_failed",
			})
			continue
		}
		succeeded = append(succeeded, out)
	}

	// Outer commit. On failure every "succeeded" row in this chunk is
	// lost — transfer to failed[] with chunk_commit_failed and clear
	// the succeeded slice so the caller doesn't double-count.
	if commitErr := outerTx.Commit(ctx); commitErr != nil {
		s.deps.Logger.WarnContext(ctx, "import.chunk.commit_failed", "err", commitErr)
		for _, sr := range succeeded {
			failed = append(failed, api.BulkImportFailedRow{
				Row:    sr.lineNo,
				Error:  api.BulkImportFailedRowErrorImportFailed,
				Reason: "chunk_commit_failed",
			})
		}
		return nil, failed
	}

	// POST-COMMIT cache invalidation. ALWAYS after outerTx.Commit
	// succeeded — Pitfall 4 prevents a stale cache from masking the
	// import's writes. cache.Del is best-effort (D-55); failures log
	// warn but never downgrade the chunk's success.
	if s.deps.Cache != nil {
		for _, sr := range succeeded {
			key := cache.Key(orgID, string(sr.entity), sr.id)
			if delErr := s.deps.Cache.Del(ctx, key); delErr != nil {
				s.deps.Logger.WarnContext(ctx, "import.chunk.cache_del_failed",
					"key", key, "err", delErr)
			}
		}
	}
	return succeeded, failed
}

// derefBool returns *p or fallback when p is nil. Convenience helper
// for the row processors' enabled defaulting (api request bodies use
// *bool with omitempty for optional fields).
//
//nolint:unused // Used by Task 2 row processors.
func derefBool(p *bool, fallback bool) bool {
	if p == nil {
		return fallback
	}
	return *p
}

// nullablePgUUID returns a pgtype.UUID with Valid=false when the input
// is the zero UUID and Valid=true otherwise. Used by row_channel.go to
// pass `nil → NULL default_queue_id` through to UpsertChannelByCode.
//
//nolint:unused // Used by Task 2 row_channel.go.
func nullablePgUUID(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}
