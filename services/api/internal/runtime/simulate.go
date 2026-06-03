package runtime

import (
	"context"
	"time"
)

// SimInput is everything a deterministic simulation needs: synthetic variables,
// the pinned catalog+state Snapshot, the ordered scripted reservation outcomes,
// and the virtual clock origin. The caller (the SimulateFlow handler) resolves
// ClockStart (request value or server-chosen) and persists it for replay.
type SimInput struct {
	Input            map[string]any
	Snapshot         *Snapshot
	ScriptedOutcomes []ReservationOutcome
	ClockStart       time.Time
}

// Simulate runs a compiled plan deterministically against a virtual clock and a
// scripted reservation driver, with auto-resume so wait/offer-timeout advance
// the clock instead of parking. It returns the ordered Trace; the same
// (plan, snapshot, input, clock, scripts) always yields the same steps.
func Simulate(ctx context.Context, reg *Registry, plan CompiledPlan, in SimInput) (Trace, error) {
	clock := NewVirtualClock(in.ClockStart)
	driver := NewScriptedDriver(in.ScriptedOutcomes)
	ex := NewExecutor(reg, WithAutoResume(), WithRouting(in.Snapshot, driver))
	res, err := ex.Run(ctx, clock, plan, in.Input)
	return res.Trace, err
}
