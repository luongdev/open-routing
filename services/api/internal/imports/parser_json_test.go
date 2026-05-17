// parser_json_test.go — coverage for the reMarshalAs round-trip
// helper (D5-23 / F1).
package imports

import (
	"testing"
)

// fakeRow is a local stand-in struct for the round-trip target.
// Plan 05-04 deliberately avoids depending on api.ImportAgentRequest
// here because that type's integration-level behaviour is exercised
// in Wave 5 handlers_test.
type fakeRow struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// TestReMarshalAs_HappyPath confirms a well-formed map[string]any
// element round-trips into the typed target with the correct field
// values.
func TestReMarshalAs_HappyPath(t *testing.T) {
	item := map[string]any{
		"code": "agent_alice",
		"name": "Alice Smith",
	}
	got, err := reMarshalAs[fakeRow](item)
	if err != nil {
		t.Fatalf("reMarshalAs: unexpected err %v", err)
	}
	if got.Code != "agent_alice" || got.Name != "Alice Smith" {
		t.Fatalf("reMarshalAs: got %+v, want {agent_alice Alice Smith}", got)
	}
}

// TestReMarshalAs_MarshalFailure feeds in a value json.Marshal
// refuses (a function). The eager-decoder never produces such a
// value in production — this guards against a future caller
// synthesising one programmatically.
func TestReMarshalAs_MarshalFailure(t *testing.T) {
	item := func() {} // json.Marshal returns UnsupportedTypeError.
	_, err := reMarshalAs[fakeRow](item)
	if err == nil {
		t.Fatalf("reMarshalAs: expected marshal error, got nil")
	}
}

// TestReMarshalAs_MissingField verifies that omitting a non-pointer
// field in the source map produces a zero-valued field in the target
// (consistent with json.Unmarshal's documented behaviour). The
// per-entity validator in Wave 3 is responsible for catching the
// missing-required-field condition; the re-marshal helper itself
// stays type-only.
func TestReMarshalAs_MissingField(t *testing.T) {
	item := map[string]any{
		"code": "agent_alice",
		// "name" omitted.
	}
	got, err := reMarshalAs[fakeRow](item)
	if err != nil {
		t.Fatalf("reMarshalAs: unexpected err %v", err)
	}
	if got.Code != "agent_alice" {
		t.Fatalf("reMarshalAs: want code agent_alice, got %q", got.Code)
	}
	if got.Name != "" {
		t.Fatalf("reMarshalAs: want zero-valued name, got %q", got.Name)
	}
}

// TestReMarshalAs_TypeMismatch feeds in a numeric value for a string
// field. json.Unmarshal rejects with a *json.UnmarshalTypeError.
func TestReMarshalAs_TypeMismatch(t *testing.T) {
	item := map[string]any{
		"code": 42, // int into string field.
		"name": "Alice",
	}
	_, err := reMarshalAs[fakeRow](item)
	if err == nil {
		t.Fatalf("reMarshalAs: expected unmarshal error, got nil")
	}
}

// TestReMarshalAs_NestedShape proves the helper works against
// non-trivial nested structures — important because the real
// Import*Request types have nested skills slices.
func TestReMarshalAs_NestedShape(t *testing.T) {
	type inner struct {
		Code        string `json:"code"`
		Proficiency int    `json:"proficiency"`
	}
	type outer struct {
		Code   string  `json:"code"`
		Skills []inner `json:"skills"`
	}
	item := map[string]any{
		"code": "agent_alice",
		"skills": []any{
			map[string]any{"code": "skill_voice", "proficiency": 7},
			map[string]any{"code": "skill_chat", "proficiency": 9},
		},
	}
	got, err := reMarshalAs[outer](item)
	if err != nil {
		t.Fatalf("reMarshalAs: unexpected err %v", err)
	}
	if len(got.Skills) != 2 {
		t.Fatalf("reMarshalAs: want 2 skills, got %d", len(got.Skills))
	}
	if got.Skills[0].Code != "skill_voice" || got.Skills[0].Proficiency != 7 {
		t.Fatalf("reMarshalAs: nested[0] = %+v", got.Skills[0])
	}
}
