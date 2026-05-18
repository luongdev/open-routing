// handlers_csv_test.go — Wave 5 CSV-spec edge tests for the bulk-import
// endpoint (Plan 05-07 Task 2). Coverage:
//
//   - Pitfall 1 (MANDATORY): testdata/windows-excel-agents.csv with UTF-8
//     BOM + CRLF + embedded comma + embedded RFC-4180-escaped quote round-
//     trips through stripBOM + newCSVReader + UpsertAgentByCode.
//   - D5-08 / IMP-02: invalid UTF-8 byte sequence returns 400 csv_not_utf8.
//   - D5-21 / IMP-07: oversize row count (importRowLimit+1) AND oversize body
//     (51 MB) return 413 with the canonical reason.
//   - IMP-08: missing or mismatched ?schema_version returns 400 listing
//     supported versions.
//   - Defensive: nested quoted newline inside a quoted CSV field round-
//     trips (RFC 4180 quoted-field newline support).
package imports

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/api"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
	"context"
)

// TestBulkImport_WindowsExcel_BOM_CRLF_200 — Pitfall 1 MANDATORY. POST
// testdata/windows-excel-agents.csv (BOM + CRLF + embedded comma + RFC
// 4180 quote-escape) → 200 + 3 succeeded + the persisted name field
// preserves the embedded comma and the embedded double quote.
func TestBulkImport_WindowsExcel_BOM_CRLF_200(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Sanity-check the file bytes BEFORE posting — author errors that
	// silently strip the BOM make this test pass only by accident.
	body := loadTestData(t, "windows-excel-agents.csv")
	require.GreaterOrEqualf(t, len(body), 3, "windows-excel-agents.csv too short: %d bytes", len(body))
	require.Equalf(t, []byte{0xEF, 0xBB, 0xBF}, body[:3],
		"Pitfall 1: windows-excel-agents.csv MUST start with UTF-8 BOM (0xEF 0xBB 0xBF); got %x", body[:3])
	require.Containsf(t, string(body), "\r\n",
		"Pitfall 1: windows-excel-agents.csv MUST use CRLF line endings")

	resp, respBody := postImportCSV(t, th, api.Agents, body)
	require.Equalf(t, http.StatusOK, resp.StatusCode,
		"BOM + CRLF golden file must produce 200; body=%s", string(respBody))

	r := decodeBulkImportResult(t, respBody)
	require.Len(t, r.Succeeded, 3,
		"windows-excel-agents.csv has 3 data rows; got Succeeded=%v Failed=%v",
		r.Succeeded, r.Failed)
	require.Empty(t, r.Failed)

	// Verify the persisted names retain the embedded comma + escaped quote.
	_, name2 := fetchAgentByCode(t, ctx, th.Pool, th.OrgID, "emp_002")
	require.Equal(t, "Smith, John", name2,
		"row 2's name with embedded comma must round-trip via RFC 4180 quoted field")
	_, name3 := fetchAgentByCode(t, ctx, th.Pool, th.OrgID, "emp_003")
	require.Equal(t, `Mary "The Boss" Doe`, name3,
		`row 3's name with RFC 4180 "" escape must decode to a single literal quote`)
}

// TestBulkImport_InvalidUTF8_400 — D5-08. POST invalid-utf8.csv (header
// valid UTF-8 + row 1 contains 0xC3 0x28 which is NOT valid UTF-8) →
// 400 with reason csv_not_utf8.
func TestBulkImport_InvalidUTF8_400(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	resp, respBody := postImportCSV(t, th, api.Agents, loadTestData(t, "invalid-utf8.csv"))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"invalid UTF-8 bytes must produce 400 csv_not_utf8; body=%s", string(respBody))
	require.Contains(t, string(respBody), "csv_not_utf8")
}

// TestBulkImport_OversizeRows_413 — Generate importRowLimit+1 CSV data rows
// programmatically (avoids checking in a giant file); POST → 413 with
// the canonical reason.
func TestBulkImport_OversizeRows_413(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	var buf bytes.Buffer
	buf.WriteString("code,name,email\n")
	for i := 1; i <= importRowLimit+1; i++ {
		fmt.Fprintf(&buf, "emp_%05d,Agent%d,a%d@example.com\n", i, i, i)
	}

	resp, respBody := postImportCSV(t, th, api.Agents, buf.Bytes())
	require.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
		"%d rows must produce 413 (streaming cap); body=%s", importRowLimit+1, string(respBody))
	require.Contains(t, string(respBody), "request_too_large_use_async_pathway")
}

// TestBulkImport_Oversize50MBBody_413 — D5-21. Generate a > 50 MB CSV
// body programmatically. The BodyLimit middleware wraps r.Body in
// http.MaxBytesReader; the handler's ReadAll surfaces this as
// *http.MaxBytesError → 413.
//
// Body construction: ~ 64 chars per row × ~1 M rows ≈ 64 MB > 50 MB cap.
// Allocated once per test and freed when GC catches up.
func TestBulkImport_Oversize50MBBody_413(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Build a body > 50 MB. The exact row count is irrelevant — we only
	// need to trip the 50 MB cap. Use strings.Repeat so the allocation is
	// a single contiguous chunk.
	header := "code,name,email\n"
	row := "emp_0001,filler_name_padding_to_grow_the_body,a@example.com\n"
	const cap50MB = 50 << 20
	repeats := (cap50MB / len(row)) + 1024 // safely over the cap
	body := header + strings.Repeat(row, repeats)
	require.Greater(t, len(body), cap50MB,
		"test setup error: built body must exceed 50 MB cap (cap=%d, got=%d)",
		cap50MB, len(body))

	resp, respBody := postImportCSV(t, th, api.Agents, []byte(body))
	require.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
		"oversize body must produce 413 (D5-21 BodyLimit middleware); body=%s",
		string(respBody))
}

// TestBulkImport_SchemaVersionMismatch_400 — IMP-08. POST a CSV body
// with ?schema_version=v0.2 → 400 with a reason that lists the supported
// versions.
func TestBulkImport_SchemaVersionMismatch_400(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	body := []byte("code,name,email\nemp_999,Test,test@example.com\n")
	resp, respBody := postImportCSVWithSchemaVersion(t, th, api.Agents, body, "v0.2")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(respBody))
	require.Contains(t, string(respBody), "unsupported_schema_version")
	require.Contains(t, string(respBody), "v0.1",
		"reason must list the supported version v0.1; got %s", string(respBody))
}

// TestBulkImport_SchemaVersionMissing_400 — IMP-08. POST a CSV body
// with NO ?schema_version query param → 400.
func TestBulkImport_SchemaVersionMissing_400(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	body := []byte("code,name,email\nemp_999,Test,test@example.com\n")
	resp, respBody := postImportCSVNoSchemaVersion(t, th, api.Agents, body)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "body=%s", string(respBody))
	require.Contains(t, string(respBody), "unsupported_schema_version")
}

// TestBulkImport_NestedQuotedNewline_RoundTrips — RFC 4180 quoted-field
// newline support. The CSV `name` cell contains an embedded \n inside a
// quoted field; the parser must accept it and the persisted row must
// preserve the byte sequence.
//
// CSV body authored inline:
//
//	code,name,email
//	emp_quoted_newline,"Line1\nLine2",a@example.com
//
// (where \n is a literal LF inside the quoted name field).
func TestBulkImport_NestedQuotedNewline_RoundTrips(t *testing.T) {
	th := newTestImports(t)
	if th == nil {
		return
	}
	ctx := orgkey.SetOrgID(context.Background(), th.OrgID)
	cleanImportTables(t, ctx, th.Pool, th.OrgID)
	defer cleanImportTables(t, ctx, th.Pool, th.OrgID)

	// Header + one row whose name contains a literal LF embedded in a
	// quoted field. RFC 4180 §2.6 permits this.
	body := []byte("code,name,email\nemp_quoted_newline,\"Line1\nLine2\",a@example.com\n")

	resp, respBody := postImportCSV(t, th, api.Agents, body)
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"RFC 4180 quoted newline in a field must produce 200; body=%s", string(respBody))

	_, name := fetchAgentByCode(t, ctx, th.Pool, th.OrgID, "emp_quoted_newline")
	require.Equal(t, "Line1\nLine2", name,
		"quoted-field newline must round-trip exactly; got %q", name)
}
