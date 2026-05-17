// header_test.go — coverage for validateHeader (D5-05 strict-batch
// policy) and the coerceSkillsCell pipeline (D5-17).
package imports

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// TestValidateHeader_AgentsExactMatch — happy path for the largest
// entity (agents has the most columns).
func TestValidateHeader_AgentsExactMatch(t *testing.T) {
	header := []string{"code", "external_id", "name", "email", "enabled", "skills"}
	missing, unknown := validateHeader(api.Agents, header)
	if len(missing) != 0 || len(unknown) != 0 {
		t.Fatalf("validateHeader: want (nil, nil), got (%v, %v)", missing, unknown)
	}
}

// TestValidateHeader_AgentsMissingRequired drops the `code` column —
// validator must surface it under `missing`.
func TestValidateHeader_AgentsMissingRequired(t *testing.T) {
	header := []string{"external_id", "name", "email", "enabled", "skills"}
	missing, unknown := validateHeader(api.Agents, header)
	if len(unknown) != 0 {
		t.Fatalf("validateHeader: unexpected unknown %v", unknown)
	}
	if !equalSet(missing, []string{"code"}) {
		t.Fatalf("validateHeader: want missing=[code], got %v", missing)
	}
}

// TestValidateHeader_AgentsUnknownColumn adds a v0.2-style extra
// column (`salary`) — D5-05 strict policy rejects.
func TestValidateHeader_AgentsUnknownColumn(t *testing.T) {
	header := []string{"code", "name", "email", "salary"}
	missing, unknown := validateHeader(api.Agents, header)
	if !equalSet(unknown, []string{"salary"}) {
		t.Fatalf("validateHeader: want unknown=[salary], got %v", unknown)
	}
	// `external_id` is optional, `enabled` is optional, `skills` is
	// optional — only `code`, `name`, `email` are required; all
	// present. Therefore missing should be empty.
	if len(missing) != 0 {
		t.Fatalf("validateHeader: unexpected missing %v", missing)
	}
}

// TestValidateHeader_SkillsExactMatch covers the skills entity.
func TestValidateHeader_SkillsExactMatch(t *testing.T) {
	header := []string{"code", "external_id", "name", "skill_type", "description", "enabled"}
	missing, unknown := validateHeader(api.Skills, header)
	if len(missing) != 0 || len(unknown) != 0 {
		t.Fatalf("validateHeader(skills): want clean, got missing=%v unknown=%v", missing, unknown)
	}
}

// TestValidateHeader_BreakReasonsExactMatch covers break_reasons.
func TestValidateHeader_BreakReasonsExactMatch(t *testing.T) {
	header := []string{"code", "external_id", "name", "routable", "display_order", "enabled"}
	missing, unknown := validateHeader(api.BreakReasons, header)
	if len(missing) != 0 || len(unknown) != 0 {
		t.Fatalf("validateHeader(break_reasons): want clean, got missing=%v unknown=%v", missing, unknown)
	}
}

// TestValidateHeader_QueuesMissingChannelTypes — queues schema has
// channel_types required.
func TestValidateHeader_QueuesMissingChannelTypes(t *testing.T) {
	header := []string{"code", "name"}
	missing, _ := validateHeader(api.Queues, header)
	if !contains(missing, "channel_types") {
		t.Fatalf("validateHeader(queues): missing did not contain channel_types: %v", missing)
	}
}

// TestValidateHeader_ChannelsMissingChannelType — channels schema
// has channel_type required (singular, not the queues' channel_types
// plural).
func TestValidateHeader_ChannelsMissingChannelType(t *testing.T) {
	header := []string{"code", "name"}
	missing, _ := validateHeader(api.Channels, header)
	if !contains(missing, "channel_type") {
		t.Fatalf("validateHeader(channels): missing did not contain channel_type: %v", missing)
	}
}

// TestValidateHeader_UnknownEntity — defensive return when entity is
// unknown. validateHeader returns ([], header) so the handler can
// still produce a useful error envelope.
func TestValidateHeader_UnknownEntity(t *testing.T) {
	header := []string{"code", "name"}
	missing, unknown := validateHeader(api.ImportEntityType("unknown_entity"), header)
	if len(missing) != 0 {
		t.Fatalf("validateHeader(unknown): missing must be empty, got %v", missing)
	}
	if !equalSet(unknown, header) {
		t.Fatalf("validateHeader(unknown): want every column as unknown, got %v", unknown)
	}
}

// TestCoerceSkillsCell_HappyPath confirms D5-17 multi-token parse.
func TestCoerceSkillsCell_HappyPath(t *testing.T) {
	raw := "skill_voice:7|skill_chat:9"
	v, err := coerceSkillsCell(raw)
	if err != nil {
		t.Fatalf("coerceSkillsCell: unexpected err %v", err)
	}
	got, ok := v.([]SkillToken)
	if !ok {
		t.Fatalf("coerceSkillsCell: want []SkillToken, got %T", v)
	}
	want := []SkillToken{
		{SkillCode: "skill_voice", Proficiency: 7},
		{SkillCode: "skill_chat", Proficiency: 9},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coerceSkillsCell: got %+v, want %+v", got, want)
	}
}

// TestCoerceSkillsCell_MissingColon — token without `:` separator
// → ErrInvalidSkillToken.
func TestCoerceSkillsCell_MissingColon(t *testing.T) {
	if _, err := coerceSkillsCell("skill_voice7"); !errors.Is(err, ErrInvalidSkillToken) {
		t.Fatalf("coerceSkillsCell: want ErrInvalidSkillToken, got %v", err)
	}
}

// TestCoerceSkillsCell_NonIntProficiency — non-numeric proficiency
// → ErrInvalidSkillToken.
func TestCoerceSkillsCell_NonIntProficiency(t *testing.T) {
	if _, err := coerceSkillsCell("skill_voice:abc"); !errors.Is(err, ErrInvalidSkillToken) {
		t.Fatalf("coerceSkillsCell: want ErrInvalidSkillToken, got %v", err)
	}
}

// TestCoerceSkillsCell_BadCodeFormat — code part of token violates
// D04_1-03 regex → ErrInvalidCodeFormat.
func TestCoerceSkillsCell_BadCodeFormat(t *testing.T) {
	if _, err := coerceSkillsCell("INVALID-Upper:7"); !errors.Is(err, ErrInvalidCodeFormat) {
		t.Fatalf("coerceSkillsCell: want ErrInvalidCodeFormat, got %v", err)
	}
}

// TestCoerceSkillsCell_EmptyCell — empty input returns empty slice
// + nil error (admin chose no-skills agent).
func TestCoerceSkillsCell_EmptyCell(t *testing.T) {
	v, err := coerceSkillsCell("")
	if err != nil {
		t.Fatalf("coerceSkillsCell: unexpected err %v", err)
	}
	got, ok := v.([]SkillToken)
	if !ok {
		t.Fatalf("coerceSkillsCell: want []SkillToken, got %T", v)
	}
	if len(got) != 0 {
		t.Fatalf("coerceSkillsCell: want empty slice, got %v", got)
	}
}

// TestCoerceCode_HappyPath_TrimsAndValidates — happy path through
// the catalog.ValidateCodeFormat delegation.
func TestCoerceCode_HappyPath_TrimsAndValidates(t *testing.T) {
	v, err := coerceCode("  agent_alice  ")
	if err != nil {
		t.Fatalf("coerceCode: unexpected err %v", err)
	}
	if s, ok := v.(string); !ok || s != "agent_alice" {
		t.Fatalf("coerceCode: want \"agent_alice\", got %v", v)
	}
}

// TestCoerceCode_RejectsUpperCase — D04_1-03 regex disallows
// uppercase.
func TestCoerceCode_RejectsUpperCase(t *testing.T) {
	if _, err := coerceCode("AGENT_alice"); !errors.Is(err, ErrInvalidCodeFormat) {
		t.Fatalf("coerceCode: want ErrInvalidCodeFormat, got %v", err)
	}
}

// equalSet compares two []string for set equality (order-independent).
// Helper used by the validateHeader tests because missing/unknown
// slices have no guaranteed order.
func equalSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

// contains is a tiny helper for "did this slice include this value?".
func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
