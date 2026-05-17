// handler_import.go — Wave 4 controller for the BulkImportCatalog
// strict-server method (RESEARCH §Pattern 1).
//
// The full pipeline assembled here:
//
//  1. orgID extraction (Cross-Cutting Pattern 1).
//  2. ?entity= validation via api.ImportEntityType.Valid() (typed enum from
//     oapi-codegen).
//  3. Idempotency-Key replay lookup BEFORE any work (D5-13). Hit ⇒ return
//     the persisted job's BulkImportResult with idempotent_replay=true; do
//     NOT re-import.
//  4. Content-Type dispatch:
//       - req.JSONBody != nil → JSON path (eager-decoded per §F1; memory
//         bounded by D5-21 BodyLimit middleware).
//       - req.Body != nil      → CSV path (text/csv; requires
//         ?schema_version=v0.1 per IMP-08).
//       - else                 → 400 unsupported_content_type (defensive;
//         strict-server should reject earlier on unknown media type).
//  5. JSON path: enforce 500-row cap (D5-22) defensively at the array
//     boundary; build parsedRow slice with the raw item attached for the
//     Wave 3 row processor's reMarshalAs round-trip.
//  6. CSV path: ReadAll bounded by the upstream MaxBytesReader (D5-21).
//     stripBOM (Pitfall 5.1) → validateUTF8 (D5-08) → newCSVReader. Read
//     header → validateHeader (D5-05 strict batch). Stream data rows
//     through the per-column coerce callbacks; >500 rows → 413 (D5-22).
//  7. runImportPipeline:
//       a. Empty rows → 200 with empty arrays (Open Q6 "zero-rows-after-
//          header is vacuously a success").
//       b. createJob (status='pending', D5-10 step 1).
//       c. select rowProcessor by entity (one of the 6 row_<entity>.go
//          structs from Plan 05-05).
//       d. chunk loop: 50 rows per chunk, processChunk per chunk
//          (Plan 05-05 owns the chunk orchestrator).
//       e. finaliseJob with terminal status + counters + errors JSONB
//          (D5-10 step 3).
//       f. status decision (D-37): all-succeed → 200, partial → 207,
//          all-fail → 422, empty-after-header → 200 with empty arrays.
//
// Mid-stream *http.MaxBytesError handling (Pitfall 5):
//
//   The CSV path's io.ReadAll calls Read on the wrapped Body; the chunked
//   transfer-encoding / lying Content-Length surfaces as *http.MaxBytesError
//   here. The handler maps via errors.As to BulkImportCatalog413JSONResponse
//   with the canonical "request_too_large_use_async_pathway" reason.
//
// Streaming-JSON note (D5-23 vs §F1): Phase 5 ships the EAGER-DECODE
// pathway. Strict-server populates req.JSONBody as a fully-decoded
// []interface{} before this handler runs; memory is bounded by the
// upstream BodyLimit middleware + the 500-row cap. D5-23 documents a
// streaming-JSON refactor candidate for v0.2 but is NOT what we actually
// ship — see RESEARCH §F1 + §A6 for the trade-off analysis.
//
// Anti-pattern guard: BulkImportCatalog NEVER opens its own
// outerTx — chunk.processChunk owns the chunk-level Tx lifecycle. The
// handler only orchestrates the high-level "parse → createJob → loop
// processChunk → finaliseJob → emit BulkImportResult" sequence.
package imports

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// BulkImportCatalog implements the api.StrictServerInterface method of
// the same name on *Importer. See file header for the 6-step flow.
//
// Returns nil for the error value on EVERY branch — the strict-server
// pipeline maps the typed *JSONResponse to wire shape; returning a non-nil
// error would bypass the typed-response marshalling and produce a generic
// 500.
func (s *Importer) BulkImportCatalog(
	ctx context.Context,
	req api.BulkImportCatalogRequestObject,
) (api.BulkImportCatalogResponseObject, error) {
	// STEP 0: orgID extraction (Cross-Cutting Pattern 1).
	orgID, ok := orgkey.OrgIDFromContext(ctx)
	if !ok {
		return api.BulkImportCatalog500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "missing_org_id_in_context",
			},
		}, nil
	}

	// STEP 1: ?entity= validation. Empty string + unknown value both
	// fail Valid(). Catches a malformed enum value reaching the handler
	// despite the spec-level enum constraint.
	if !req.Params.Entity.Valid() {
		return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: "unsupported_entity",
		}), nil
	}

	// STEP 2: Idempotency-Key replay lookup BEFORE work (D5-13).
	if req.Params.IdempotencyKey != nil {
		replay, found, err := s.lookupIdempotentReplay(ctx, orgID, *req.Params.IdempotencyKey)
		if err != nil {
			s.deps.Logger.ErrorContext(ctx, "import.idempotency.lookup", "err", err)
			return api.BulkImportCatalog500JSONResponse{
				InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
					Error:  api.ErrorCodeInternal,
					Reason: "idempotency_lookup_failed",
				},
			}, nil
		}
		if found {
			return replay, nil
		}
	}

	// STEP 3: Content-Type dispatch.
	switch {
	case req.JSONBody != nil:
		return s.importJSON(ctx, orgID, req.Params.Entity, *req.JSONBody, req)
	case req.Body != nil:
		// CSV requests MUST carry ?schema_version=v0.1 per IMP-08.
		if req.Params.SchemaVersion == nil || *req.Params.SchemaVersion != "v0.1" {
			return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidBody,
				Reason: "unsupported_schema_version_supported_versions=v0.1",
			}), nil
		}
		return s.importCSV(ctx, orgID, req.Params.Entity, req.Body, req)
	default:
		return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: "unsupported_content_type",
		}), nil
	}
}

// importJSON handles the application/json path. Strict-server has already
// eager-decoded the body into []interface{} (per §F1) so we get
// random-access semantics — count, then walk.
//
// 500-row defence in depth: the OpenAPI spec already enforces
// `maxItems: 500` on the request body, but the codegen does not always
// honour it at decode time. A handler-level guard keeps the contract
// honest against future codegen drift.
func (s *Importer) importJSON(
	ctx context.Context,
	orgID uuid.UUID,
	entity api.ImportEntityType,
	body []interface{},
	req api.BulkImportCatalogRequestObject,
) (api.BulkImportCatalogResponseObject, error) {
	if len(body) > importRowLimit {
		return api.BulkImportCatalog413JSONResponse{
			RequestEntityTooLargeJSONResponse: api.RequestEntityTooLargeJSONResponse{
				Error:  api.ErrorCodeInvalidBody,
				Reason: "request_too_large_use_async_pathway",
			},
		}, nil
	}

	// Build parsedRow slice for the chunk loop. Wave 3's row processors
	// reMarshalAs[T] each `raw` into the typed Import*Request shape;
	// per-row schema errors surface as `invalid_json_row` rowError there.
	rows := make([]parsedRow, 0, len(body))
	for i, item := range body {
		rows = append(rows, parsedRow{
			lineNo: i + 1, // 1-based; matches BulkImportFailedRow.Row convention.
			raw:    item,
		})
	}

	return s.runImportPipeline(ctx, orgID, entity, rows, req)
}

// importCSV handles the text/csv path. The body is bounded by the
// upstream BodyLimit middleware (50 MB cap, D5-21); a malicious lying
// Content-Length or chunked transfer surfaces as *http.MaxBytesError on
// the first Read.
//
// Pragmatic v0.1 decision: ReadAll the bounded body into memory, then
// validate UTF-8 + parse with csv.Reader. This is NOT streaming CSV; the
// 50 MB upstream cap keeps the working set bounded. A streaming CSV
// refactor is a v0.2 candidate per 05-CONTEXT.md §Deferred.
func (s *Importer) importCSV(
	ctx context.Context,
	orgID uuid.UUID,
	entity api.ImportEntityType,
	body io.Reader,
	req api.BulkImportCatalogRequestObject,
) (api.BulkImportCatalogResponseObject, error) {
	// Step CSV-A: stripBOM (Pitfall 5.1) — Windows / Excel exports
	// prepend UTF-8 BOM. Strip BEFORE ReadAll so the BOM bytes never
	// reach validateUTF8 (BOM IS valid UTF-8 but would smuggle into the
	// first header column).
	br, err := stripBOM(body)
	if err != nil {
		// Pre-ReadAll error from bufio.Peek — surface as 400.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return api.BulkImportCatalog413JSONResponse{
				RequestEntityTooLargeJSONResponse: api.RequestEntityTooLargeJSONResponse{
					Error:  api.ErrorCodeInvalidBody,
					Reason: "request_too_large_use_async_pathway",
				},
			}, nil
		}
		s.deps.Logger.WarnContext(ctx, "import.csv.bom_strip_failed", "err", err)
		return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: "malformed_csv",
		}), nil
	}

	// Step CSV-B: ReadAll into memory. The upstream BodyLimit wrap on
	// r.Body means a > 50 MB body surfaces as *http.MaxBytesError on
	// the first Read here (the middleware doesn't peek inside io.Reader
	// use; the handler is the canonical 413 emitter).
	bodyBytes, err := io.ReadAll(br)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return api.BulkImportCatalog413JSONResponse{
				RequestEntityTooLargeJSONResponse: api.RequestEntityTooLargeJSONResponse{
					Error:  api.ErrorCodeInvalidBody,
					Reason: "request_too_large_use_async_pathway",
				},
			}, nil
		}
		s.deps.Logger.WarnContext(ctx, "import.csv.read_failed", "err", err)
		return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: "malformed_csv",
		}), nil
	}

	// Step CSV-C: UTF-8 validation (D5-08). PITFALLS 5.1 lists charset
	// auto-detect as the #1 silent-corruption source.
	if vErr := validateUTF8(bodyBytes); vErr != nil {
		return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: "csv_not_utf8",
		}), nil
	}

	// Step CSV-D: header parse. Empty input (no bytes at all) → 200 with
	// empty arrays per Open Q6 — "zero-rows-after-header is vacuously a
	// success." A header-only body lands in the same branch (cr.Read
	// returns the header; the data-row loop exits with EOF on its first
	// iteration).
	cr := newCSVReader(bytes.NewReader(bodyBytes))
	header, hErr := cr.Read()
	if errors.Is(hErr, io.EOF) {
		// Truly empty body — vacuous success.
		return s.emptyResultResponse(api.BulkImportCatalog200JSONResponse{}), nil
	}
	if hErr != nil {
		return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: "malformed_csv",
		}), nil
	}

	// Step CSV-E: header validation (D5-05 strict batch policy).
	missing, unknown := validateHeader(entity, header)
	if len(missing) > 0 || len(unknown) > 0 {
		// Combined reason string lists both halves; admin sees a single
		// 400 explaining exactly which columns are wrong.
		reason := buildHeaderReason(missing, unknown)
		return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
			Error:  api.ErrorCodeInvalidBody,
			Reason: reason,
		}), nil
	}

	// Step CSV-F: row-stream loop. Each iteration counts; exit at row
	// 501 with 413 per D5-22.
	rows := make([]parsedRow, 0, 32)
	rowIdx := 0
	for {
		record, rErr := cr.Read()
		if errors.Is(rErr, io.EOF) {
			break
		}
		if rErr != nil {
			// Mid-file CSV syntax error (LazyQuotes=false catches bare
			// quotes; FieldsPerRecord=0 latches column count). v0.1
			// rejects the entire batch — admin's CSV is malformed.
			return api.BulkImportCatalog400JSONResponse(api.ErrorResponse{
				Error:  api.ErrorCodeInvalidBody,
				Reason: "malformed_csv",
			}), nil
		}
		rowIdx++
		if rowIdx > importRowLimit {
			// Row 501 — 413 per D5-22. The csv.Reader does not load the
			// rest of the file, so the early-exit is genuine (not just
			// a post-parse count).
			return api.BulkImportCatalog413JSONResponse{
				RequestEntityTooLargeJSONResponse: api.RequestEntityTooLargeJSONResponse{
					Error:  api.ErrorCodeInvalidBody,
					Reason: "request_too_large_use_async_pathway",
				},
			}, nil
		}

		// Step CSV-G: per-cell coercion via the entityRegistry callbacks.
		// Cell-level errors surface as per-row failures with the matching
		// `field` populated; the row is excluded from the chunk loop so it
		// never reaches a savepoint.
		cells, coercionErr := coerceRow(entity, header, record)
		raw := materialiseTypedRaw(entity, cells)
		rows = append(rows, parsedRow{
			lineNo:        rowIdx,
			raw:           raw,
			cells:         cells,
			coerceFailure: coercionErr,
		})
	}

	return s.runImportPipeline(ctx, orgID, entity, rows, req)
}

// runImportPipeline is the chunk-loop driver shared by the JSON and CSV
// paths. After this returns, the caller's typed *JSONResponse carries the
// final BulkImportResult.
//
// Status decision (D-37):
//
//   - empty input (succeeded == 0 && failed == 0) → 200 with empty arrays
//   - all-succeed (failed == 0)                   → 200 BulkImportResult
//   - all-fail   (succeeded == 0 && failed > 0)   → 422 BulkImportResult
//   - partial    (succeeded > 0 && failed > 0)    → 207 BulkImportResult
func (s *Importer) runImportPipeline(
	ctx context.Context,
	orgID uuid.UUID,
	entity api.ImportEntityType,
	rows []parsedRow,
	req api.BulkImportCatalogRequestObject,
) (api.BulkImportCatalogResponseObject, error) {
	// Open Q6: zero-rows-after-header is vacuously a success.
	if len(rows) == 0 {
		return s.emptyResultResponse(api.BulkImportCatalog200JSONResponse{}), nil
	}

	// D5-10 step 1: createJob in its own short tx BEFORE chunk 1 opens.
	// The idempotency-key string (nil-able) is persisted on the new row.
	var idemKeyStr *string
	if req.Params.IdempotencyKey != nil {
		k := uuid.UUID(*req.Params.IdempotencyKey).String()
		idemKeyStr = &k
	}
	jobID, jobErr := s.createJob(ctx, orgID, entity, len(rows), idemKeyStr)
	if jobErr != nil {
		s.deps.Logger.ErrorContext(ctx, "import.create_job", "err", jobErr)
		return api.BulkImportCatalog500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "create_job_failed",
			},
		}, nil
	}

	// Row processor dispatch by entity. Each rowProcessor is stateless
	// per row; one instance reused across the entire chunk loop.
	rowProc := s.rowProcessorFor(entity)
	if rowProc == nil {
		// Defensive — Valid() check above should have rejected; if a new
		// entity ever escapes the registry, fail the job with a clear
		// signal so admins see something better than "all rows failed."
		s.deps.Logger.ErrorContext(ctx, "import.no_row_processor", "entity", string(entity))
		_ = s.finaliseJob(ctx, jobID, orgID, api.ImportJobStatus("failed"), 0, len(rows), nil)
		return api.BulkImportCatalog500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "no_row_processor",
			},
		}, nil
	}

	// Chunk loop (D5-09 50-row chunks).
	var (
		allSucceeded []succeededRow
		allFailed    []api.BulkImportFailedRow
	)
	for start := 0; start < len(rows); start += chunkSize {
		end := start + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]

		// Pre-chunk: rows that already failed coercion (CSV path) are
		// emitted as per-row failures without entering a savepoint. The
		// chunk loop receives only the post-coercion rows.
		processable := make([]parsedRow, 0, len(chunk))
		for _, r := range chunk {
			if r.coerceFailure != nil {
				row := api.BulkImportFailedRow{
					Row:    r.lineNo,
					Error:  api.BulkImportFailedRowErrorImportFailed,
					Reason: r.coerceFailure.Reason,
				}
				if r.coerceFailure.Field != "" {
					f := r.coerceFailure.Field
					row.Field = &f
				}
				allFailed = append(allFailed, row)
				continue
			}
			processable = append(processable, r)
		}

		if len(processable) == 0 {
			continue
		}

		succ, fail := s.processChunk(ctx, orgID, entity, rowProc, processable)
		allSucceeded = append(allSucceeded, succ...)
		allFailed = append(allFailed, fail...)
	}

	// D5-10 step 3 + D5-12: serialise errors JSONB once, finalise the
	// job. The wire shape mirrors BulkImportResult.failed[] exactly so
	// GetImportJob can re-emit it without transformation.
	var errorsJSON []byte
	if len(allFailed) > 0 {
		if b, mErr := json.Marshal(allFailed); mErr == nil {
			errorsJSON = b
		} else {
			// Defensive — failed[] is a generated typed struct; Marshal
			// failure is a programming error, not a runtime concern.
			s.deps.Logger.ErrorContext(ctx, "import.errors_marshal_failed", "err", mErr)
		}
	}

	terminalStatus := api.ImportJobStatus("completed")
	if len(allSucceeded) == 0 && len(allFailed) > 0 {
		terminalStatus = api.ImportJobStatus("failed")
	}
	if finErr := s.finaliseJob(ctx, jobID, orgID, terminalStatus, len(allSucceeded), len(allFailed), errorsJSON); finErr != nil {
		// Phase 5 fix H3 — finaliseJob failure MUST surface as HTTP 500.
		// Pre-fix: handler logged warn and returned the 200/207 success
		// result anyway. Three downstream consequences poisoned the audit
		// trail:
		//   1. GET /imports/{id} returned status=pending with zero counters
		//      (the persisted row never advanced past the createJob step).
		//   2. Idempotency-Key replay rehydrated the pending row, returning
		//      empty succeeded[] + empty failed[] with status 200 — masking
		//      the original outcome from clients that retried.
		//   3. The 24h crash sweep flipped the pending row to
		//      failed/server_crash, overwriting the actual outcome with a
		//      synthetic crash entry.
		// The import IS already committed at the DB level (each chunk's
		// outer tx ran independently), so admin retry is the recovery path
		// — but the audit row needs to reflect reality. Returning 500
		// signals "audit corrupt; verify state via GET" without lying
		// about the import's outcome.
		s.deps.Logger.ErrorContext(ctx, "import.finalise_job_failed",
			"err", finErr, "job_id", jobID,
			"succeeded", len(allSucceeded), "failed", len(allFailed),
			"note", "data committed but audit row stayed pending; admin must verify via GET /imports/{id}")
		return api.BulkImportCatalog500JSONResponse{
			InternalServerErrorJSONResponse: api.InternalServerErrorJSONResponse{
				Error:  api.ErrorCodeInternal,
				Reason: "finalise_job_failed_data_committed_audit_corrupt",
			},
		}, nil
	}

	// Build the wire-shape Succeeded slice (UUIDv7s only — we drop the
	// entity tag and lineNo before crossing the response boundary).
	succUUIDs := make([]api.UUIDv7, 0, len(allSucceeded))
	for _, sr := range allSucceeded {
		succUUIDs = append(succUUIDs, api.UUIDv7(sr.id))
	}
	// failed[] may be nil for the empty case — normalise to [] so the
	// wire shape is consistent (BulkImportResult.Failed has the
	// `json:"failed"` tag without omitempty).
	if allFailed == nil {
		allFailed = []api.BulkImportFailedRow{}
	}

	result := api.BulkImportResult{
		Succeeded: succUUIDs,
		Failed:    allFailed,
	}

	switch {
	case len(allSucceeded) > 0 && len(allFailed) == 0:
		return api.BulkImportCatalog200JSONResponse(result), nil
	case len(allSucceeded) > 0 && len(allFailed) > 0:
		return api.BulkImportCatalog207JSONResponse(result), nil
	case len(allSucceeded) == 0 && len(allFailed) > 0:
		return api.BulkImportCatalog422JSONResponse(result), nil
	default:
		// allSucceeded == 0 && allFailed == 0 — every row was filtered
		// out before the chunk loop (e.g. all coercion errors were
		// transferred to failed[], but allFailed happened to be empty —
		// impossible in practice). Fall back to 200 empty.
		return s.emptyResultResponse(api.BulkImportCatalog200JSONResponse{}), nil
	}
}

// rowProcessorFor returns the per-entity rowProcessor for the chunk
// loop. Returns nil for unknown entities (which Valid() should have
// already rejected).
func (s *Importer) rowProcessorFor(entity api.ImportEntityType) rowProcessor {
	switch entity {
	case api.Agents:
		return &agentRowProc{handlers: s}
	case api.Skills:
		return &skillRowProc{handlers: s}
	case api.Queues:
		return &queueRowProc{handlers: s}
	case api.Channels:
		return &channelRowProc{handlers: s}
	case api.Adapters:
		return &adapterRowProc{handlers: s}
	case api.BreakReasons:
		return &breakReasonRowProc{handlers: s}
	default:
		return nil
	}
}

// emptyResultResponse builds a 200 BulkImportCatalog response with empty
// Succeeded + Failed arrays. Used for the zero-rows-after-header case
// (Open Q6) and the fall-through safety branch in runImportPipeline.
func (s *Importer) emptyResultResponse(_ api.BulkImportCatalog200JSONResponse) api.BulkImportCatalog200JSONResponse {
	return api.BulkImportCatalog200JSONResponse(api.BulkImportResult{
		Succeeded: []api.UUIDv7{},
		Failed:    []api.BulkImportFailedRow{},
	})
}

// buildHeaderReason joins missing + unknown column lists into a single
// human-readable reason string for the 400 ErrorResponse. The format is
// stable so admin UIs can parse it for column-level highlighting.
func buildHeaderReason(missing, unknown []string) string {
	var b bytes.Buffer
	b.WriteString("invalid_header")
	if len(missing) > 0 {
		b.WriteString(":missing=")
		for i, m := range missing {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(m)
		}
	}
	if len(unknown) > 0 {
		b.WriteString(":unknown=")
		for i, u := range unknown {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(u)
		}
	}
	return b.String()
}

// coerceRow runs the per-column coerce callback for each cell in a CSV
// data row. Returns a map[columnName]coercedValue plus the first
// coercion error encountered (cell-level failure). The first failure
// short-circuits the row — chunk.processChunk's loop will surface it as
// a BulkImportFailedRow with the matching field name.
//
// header is the validated header slice from validateHeader; row is the
// raw record from csv.Reader.Read. The two are guaranteed to have the
// same length thanks to csv.Reader's FieldsPerRecord=0 latching.
func coerceRow(entity api.ImportEntityType, header, row []string) (map[string]any, *rowError) {
	registry, ok := entityRegistry[entity]
	if !ok {
		// Defensive — Valid() check upstream should have rejected.
		return nil, &rowError{Field: "", Reason: "unsupported_entity"}
	}
	specByName := make(map[string]columnSpec, len(registry))
	for _, spec := range registry {
		specByName[spec.name] = spec
	}

	cells := make(map[string]any, len(header))
	for i, col := range header {
		raw := ""
		if i < len(row) {
			raw = row[i]
		}
		spec, known := specByName[col]
		if !known {
			// validateHeader already rejected this case; defensive only.
			continue
		}
		val, err := spec.coerce(raw)
		if err != nil {
			return nil, &rowError{Field: col, Reason: err.Error()}
		}
		cells[col] = val
	}
	return cells, nil
}

// materialiseTypedRaw shapes the coerced cells map into a JSON-friendly
// map that the row processor's reMarshalAs[T] can decode into the
// matching Import*Request struct. This bridges the CSV path (cells map)
// and the JSON path (raw item) at the row-processor boundary so both
// paths can share the same chunk loop without per-source switches.
//
// For agents, the `skills` column carries a []SkillToken that must be
// re-shaped into the {skill_code, proficiency} JSON shape expected by
// ImportAgentRequest.Skills. The other entities are passthrough — their
// columns map 1:1 to the typed struct field names.
//
// Returns nil when cells is empty (which the caller treats as no row
// data — runImportPipeline drops the entry).
func materialiseTypedRaw(entity api.ImportEntityType, cells map[string]any) interface{} {
	if cells == nil {
		return nil
	}
	out := make(map[string]interface{}, len(cells))
	for k, v := range cells {
		switch k {
		case "skills":
			// Agents-only: rewrite []SkillToken into the typed JSON shape
			// that ImportAgentRequest.Skills expects.
			if tokens, ok := v.([]SkillToken); ok {
				arr := make([]map[string]interface{}, 0, len(tokens))
				for _, t := range tokens {
					arr = append(arr, map[string]interface{}{
						"skill_code":  t.SkillCode,
						"proficiency": t.Proficiency,
					})
				}
				out["skills"] = arr
				continue
			}
			out[k] = v
		case "config":
			// Adapters-only: CSV's `config` cell is a JSON-encoded string
			// (per RFC 4180 escaping); ImportAdapterRequest.Config expects
			// a *map[string]interface{}. Decode the string into a map so
			// the downstream reMarshalAs[ImportAdapterRequest] round-trip
			// produces the typed shape the row processor expects. An empty
			// string or nil falls through to omit the field, matching the
			// JSONB column's NULL/empty-object behaviour.
			if s, ok := v.(string); ok && s != "" {
				var m map[string]interface{}
				if err := json.Unmarshal([]byte(s), &m); err == nil {
					out[k] = m
					continue
				}
				// Decode failure: leave as string and let the row processor
				// surface invalid_json_row. Defensive — coerce.go already
				// trimmed the input; only malformed JSON lands here.
				out[k] = v
				continue
			}
			// Nil / empty string → omit the field (row processor's Config
			// will be nil, which defaults to "{}" at JSONB write time).
			if v == nil || v == "" {
				continue
			}
			out[k] = v
		default:
			out[k] = v
		}
	}
	_ = entity // entity-specific reshape stays explicit per-column above.
	return out
}
