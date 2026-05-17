// parser_csv.go — CSV reader primitives (D5-05, D5-08; PITFALLS 5.1).
//
// stripBOM is the non-destructive prefix peeker that handles
// Windows / Excel exports prepending UTF-8 BOM (0xEF 0xBB 0xBF) to the
// CSV stream. csv.Reader does NOT strip the BOM on its own; left
// alone, the first header column reads as <BOM>"code" instead of
// "code" and the strict header validator (D5-05) rejects every row.
// PITFALLS 5.1 locks this as a NON-NEGOTIABLE Phase 5 test case.
//
// newCSVReader configures csv.Reader with the strict policy decided
// during planning:
//
//   - LazyQuotes=false  — a bare " inside an unquoted field is a syntax
//     error. Tolerating it leaks malformed cells into the per-entity
//     validators and produces confusing per-row errors.
//   - FieldsPerRecord=0 — first record's column count latches; every
//     subsequent record must match. Rejects mid-file truncation.
//   - TrimLeadingSpace=false — a cell with deliberate leading
//     whitespace (e.g. a code containing space — caught later by
//     ValidateCodeFormat) is preserved as-is so we can emit a precise
//     per-row error rather than silently mangling the input.
package imports

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
)

// utf8BOM is the three-byte UTF-8 BOM marker. Pulled out as a package
// var so stripBOM and any future call site share one definition.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripBOM wraps r in a bufio.Reader and consumes the leading three
// bytes IFF they match utf8BOM. The wrapping is required because the
// stdlib io.Reader contract does not expose Peek — we need bufio to
// inspect without consuming on the no-BOM path.
//
// Behaviours:
//
//   - 3+ bytes available AND first three == utf8BOM → Discard(3),
//     return reader positioned at byte 4.
//   - 3+ bytes available AND first three != utf8BOM → no Discard,
//     return reader positioned at byte 0.
//   - < 3 bytes available (including 0 — empty input) → no Discard,
//     return reader positioned at byte 0. The csv.Reader handles
//     short/empty input cleanly downstream.
//
// io.EOF from Peek is non-fatal; any other error from Peek surfaces to
// the caller (network reader failure, etc.).
func stripBOM(r io.Reader) (*bufio.Reader, error) {
	br := bufio.NewReader(r)
	peeked, err := br.Peek(3)
	if err != nil {
		if err == io.EOF {
			// Short or empty input — fine; csv.Reader handles it.
			return br, nil
		}
		return nil, err
	}
	if bytes.Equal(peeked, utf8BOM) {
		_, _ = br.Discard(3)
	}
	return br, nil
}

// newCSVReader builds a *csv.Reader with the locked strict-import
// policy. Callers feed the *bufio.Reader returned by stripBOM (or any
// io.Reader if BOM handling is not required for the input — e.g.
// internal-generated rows in v0.2 dry-run mode). Returns the reader
// configured but not yet read from.
func newCSVReader(r io.Reader) *csv.Reader {
	cr := csv.NewReader(r)
	cr.LazyQuotes = false
	cr.FieldsPerRecord = 0
	cr.TrimLeadingSpace = false
	return cr
}
