package runtime

import (
	"sort"
	"time"
)

// Candidate is one agent eligible for routing, with its proficiency per skill
// code and the instant it became available (for the longest-available
// tie-break). The runtime builds these from the catalog snapshot; simulation
// supplies them directly.
type Candidate struct {
	AgentID        string         `json:"agent_id"`
	Proficiency    map[string]int `json:"proficiency"`
	AvailableSince time.Time      `json:"available_since"`
}

// RequiredSkill is a skill the flow demands, with an optional minimum
// proficiency (0 = "any proficiency, presence suffices").
type RequiredSkill struct {
	Code           string `json:"code"`
	MinProficiency int    `json:"min_proficiency"`
}

// eligible reports whether a candidate has every required skill at or above its
// minimum proficiency. A required skill the agent lacks entirely fails, even at
// MinProficiency 0 — "0" means "presence suffices", not "skill optional".
func (c Candidate) eligible(required []RequiredSkill) bool {
	for _, rs := range required {
		prof, ok := c.Proficiency[rs.Code]
		if !ok || prof < rs.MinProficiency {
			return false
		}
	}
	return true
}

// RankCandidates returns the eligible candidates in selection order (best
// first): by summed required-skill proficiency descending, then longest
// available (earliest AvailableSince) ascending, then AgentID ascending as the
// stable final tie-break so the order is deterministic for replay.
func RankCandidates(cands []Candidate, required []RequiredSkill) []Candidate {
	eligible := make([]Candidate, 0, len(cands))
	for _, c := range cands {
		if c.eligible(required) {
			eligible = append(eligible, c)
		}
	}
	score := func(c Candidate) int {
		sum := 0
		for _, rs := range required {
			sum += c.Proficiency[rs.Code]
		}
		return sum
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		si, sj := score(eligible[i]), score(eligible[j])
		if si != sj {
			return si > sj
		}
		if !eligible[i].AvailableSince.Equal(eligible[j].AvailableSince) {
			return eligible[i].AvailableSince.Before(eligible[j].AvailableSince)
		}
		return eligible[i].AgentID < eligible[j].AgentID
	})
	return eligible
}

// SelectCandidate returns the single best candidate, or ok=false when none are
// eligible (the no_eligible_candidate routing failure).
func SelectCandidate(cands []Candidate, required []RequiredSkill) (Candidate, bool) {
	ranked := RankCandidates(cands, required)
	if len(ranked) == 0 {
		return Candidate{}, false
	}
	return ranked[0], true
}
