// parser_csv_test.go — coverage for stripBOM + newCSVReader.
//
// PITFALLS 5.1 carve-out: the BOM strip + CRLF + embedded quote
// fixtures here are NON-NEGOTIABLE — Wave 5 testdata files will
// re-exercise these against a real Excel export, but the unit-level
// guards live here.
package imports

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestStripBOM_StripsBOMWhenPresent feeds the 3-byte UTF-8 BOM
// followed by "code" — the returned reader must surface only "code".
func TestStripBOM_StripsBOMWhenPresent(t *testing.T) {
	input := append([]byte{0xEF, 0xBB, 0xBF}, []byte("code")...)
	br, err := stripBOM(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("stripBOM: unexpected err %v", err)
	}
	got, _ := io.ReadAll(br)
	if string(got) != "code" {
		t.Fatalf("stripBOM: want %q, got %q", "code", string(got))
	}
}

// TestStripBOM_NoBOM_PassthroughIntact confirms the reader is
// position-preserving when no BOM is present.
func TestStripBOM_NoBOM_PassthroughIntact(t *testing.T) {
	input := []byte("code,name\n")
	br, err := stripBOM(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("stripBOM: unexpected err %v", err)
	}
	got, _ := io.ReadAll(br)
	if string(got) != "code,name\n" {
		t.Fatalf("stripBOM: passthrough mismatched. want %q got %q", "code,name\n", string(got))
	}
}

// TestStripBOM_EmptyInput_NoOp confirms that EOF on Peek is not
// treated as a hard error.
func TestStripBOM_EmptyInput_NoOp(t *testing.T) {
	br, err := stripBOM(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("stripBOM: unexpected err %v", err)
	}
	got, _ := io.ReadAll(br)
	if len(got) != 0 {
		t.Fatalf("stripBOM: want empty output, got %q", string(got))
	}
}

// TestStripBOM_OneByte_NoOp_LooksLikeBOMStart confirms that a
// 1-byte input matching the BOM's first byte is NOT consumed (we
// can't peek 3 — defensively pass through).
func TestStripBOM_OneByte_NoOp_LooksLikeBOMStart(t *testing.T) {
	input := []byte{0xEF}
	br, err := stripBOM(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("stripBOM: unexpected err %v", err)
	}
	got, _ := io.ReadAll(br)
	if len(got) != 1 || got[0] != 0xEF {
		t.Fatalf("stripBOM: short-input passthrough mismatched. got %v", got)
	}
}

// TestNewCSVReader_StrictQuotes_RejectsBareQuote feeds a malformed
// quoted field (unterminated quote) — csv.Reader with LazyQuotes=false
// must surface a parse error.
func TestNewCSVReader_StrictQuotes_RejectsBareQuote(t *testing.T) {
	r := newCSVReader(strings.NewReader(`a,"unterminated`))
	if _, err := r.ReadAll(); err == nil {
		t.Fatalf("newCSVReader: want parse error, got nil")
	}
}

// TestNewCSVReader_ParsesQuotedNewlines confirms embedded newlines
// inside a quoted field are preserved (RFC 4180 compliance).
func TestNewCSVReader_ParsesQuotedNewlines(t *testing.T) {
	// One record with two fields; the second field has an embedded \n.
	r := newCSVReader(strings.NewReader("a,\"line1\nline2\"\n"))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("newCSVReader: unexpected err %v", err)
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("newCSVReader: want 1 row of 2 fields, got %v", rows)
	}
	if rows[0][1] != "line1\nline2" {
		t.Fatalf("newCSVReader: embedded newline lost. got %q", rows[0][1])
	}
}

// TestNewCSVReader_CRLFAutoConversion confirms csv.Reader normalises
// CRLF to LF on output (stdlib contract).
func TestNewCSVReader_CRLFAutoConversion(t *testing.T) {
	r := newCSVReader(strings.NewReader("a,b\r\nc,d\r\n"))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("newCSVReader: unexpected err %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("newCSVReader: want 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "a" || rows[0][1] != "b" {
		t.Fatalf("newCSVReader: row 0 mismatched %v", rows[0])
	}
	if rows[1][0] != "c" || rows[1][1] != "d" {
		t.Fatalf("newCSVReader: row 1 mismatched %v", rows[1])
	}
}

// TestNewCSVReader_FieldsPerRecordZeroLatches confirms that the
// FieldsPerRecord=0 setting latches the first record's column count
// and rejects records that don't match.
func TestNewCSVReader_FieldsPerRecordZeroLatches(t *testing.T) {
	r := newCSVReader(strings.NewReader("a,b,c\n1,2\n"))
	_, err := r.ReadAll()
	if err == nil {
		t.Fatalf("newCSVReader: want ErrFieldCount, got nil")
	}
}

// TestStripBOM_BOMThenCRLF_RealisticExcelExport reproduces the
// Excel-export shape verbatim: BOM + header + CRLF + row.
// PITFALLS 5.1 carve-out — this is the unit-level shadow of the
// Wave 5 windows-excel-agents.csv golden file.
func TestStripBOM_BOMThenCRLF_RealisticExcelExport(t *testing.T) {
	raw := append([]byte{0xEF, 0xBB, 0xBF}, []byte("code,name\r\nagent_alice,Alice\r\n")...)
	br, err := stripBOM(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("stripBOM: unexpected err %v", err)
	}
	r := newCSVReader(br)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: unexpected err %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ReadAll: want 2 rows, got %d", len(rows))
	}
	if rows[0][0] != "code" {
		t.Fatalf("ReadAll: BOM not stripped — got header[0]=%q", rows[0][0])
	}
	if rows[1][0] != "agent_alice" {
		t.Fatalf("ReadAll: data row 0 = %v", rows[1])
	}
}
