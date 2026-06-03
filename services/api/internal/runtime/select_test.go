package runtime

import (
	"testing"
	"time"
)

func TestRankCandidates_EligibilityAndOrder(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	cands := []Candidate{
		{AgentID: "a", Proficiency: map[string]int{"es": 3}, AvailableSince: t1},
		{AgentID: "b", Proficiency: map[string]int{"es": 3}, AvailableSince: t0}, // longer available
		{AgentID: "c", Proficiency: map[string]int{"es": 1}, AvailableSince: t0}, // below min
		{AgentID: "d", Proficiency: map[string]int{"fr": 5}, AvailableSince: t0}, // lacks es
		{AgentID: "e", Proficiency: map[string]int{"es": 5}, AvailableSince: t1}, // highest
	}
	req := []RequiredSkill{{Code: "es", MinProficiency: 2}}
	ranked := RankCandidates(cands, req)

	got := make([]string, len(ranked))
	for i, c := range ranked {
		got[i] = c.AgentID
	}
	// e (prof 5) first; then a,b tie on prof 3 -> b longer-available -> b before a.
	want := []string{"e", "b", "a"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("rank = %v, want %v (c below-min and d missing-skill excluded)", got, want)
	}
}

func TestSelectCandidate_AgentIDFinalTieBreak(t *testing.T) {
	same := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	cands := []Candidate{
		{AgentID: "zzz", Proficiency: map[string]int{"es": 3}, AvailableSince: same},
		{AgentID: "aaa", Proficiency: map[string]int{"es": 3}, AvailableSince: same},
	}
	got, ok := SelectCandidate(cands, []RequiredSkill{{Code: "es"}})
	if !ok || got.AgentID != "aaa" {
		t.Fatalf("select = %q ok=%v, want aaa (stable AgentID tie-break)", got.AgentID, ok)
	}
}

func TestSelectCandidate_NoneEligible(t *testing.T) {
	cands := []Candidate{{AgentID: "a", Proficiency: map[string]int{"fr": 5}}}
	if _, ok := SelectCandidate(cands, []RequiredSkill{{Code: "es"}}); ok {
		t.Fatal("expected no eligible candidate")
	}
}

func TestEvalCondition(t *testing.T) {
	vars := map[string]any{"tier": "gold", "age": float64(30), "vip": true}
	get := func(k string) (any, bool) { v, ok := vars[k]; return v, ok }
	cases := []struct {
		expr string
		want bool
	}{
		{`tier == gold`, true},
		{`tier == "silver"`, false},
		{`tier != silver`, true},
		{`age > 18`, true},
		{`age <= 30`, true},
		{`age < 30`, false},
		{`vip`, true},
		{`missing`, false},
	}
	for _, c := range cases {
		got, err := evalCondition(c.expr, get)
		if err != nil {
			t.Fatalf("evalCondition(%q): %v", c.expr, err)
		}
		if got != c.want {
			t.Errorf("evalCondition(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}
