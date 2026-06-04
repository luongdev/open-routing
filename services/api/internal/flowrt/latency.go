package flowrt

import (
	"log/slog"
	"sort"
	"sync"
	"time"
)

// routeDecisionBudget is the v0.2 report-only p95 target for a route decision
// (entry → first park/terminal). Exceeding it logs a WARN; it never fails a
// request — v0.2 measures and reports, it does not enforce (ROADMAP).
const routeDecisionBudget = 50 * time.Millisecond

// decisionLatency is a fixed-window p95 recorder for route-decision wall time.
// Lock-guarded ring buffer of the last N samples — bounded memory, no deps
// (the /metrics stub stays empty; this reports via the structured log).
type decisionLatency struct {
	mu      sync.Mutex
	samples []time.Duration
	next    int
	full    bool
}

const decisionWindow = 512

var routeDecisions = &decisionLatency{samples: make([]time.Duration, decisionWindow)}

func (d *decisionLatency) record(v time.Duration) {
	d.mu.Lock()
	d.samples[d.next] = v
	d.next = (d.next + 1) % len(d.samples)
	if d.next == 0 {
		d.full = true
	}
	d.mu.Unlock()
}

// p95 returns the 95th-percentile sample and the window size (0,0 when empty).
func (d *decisionLatency) p95() (time.Duration, int) {
	d.mu.Lock()
	n := d.next
	if d.full {
		n = len(d.samples)
	}
	if n == 0 {
		d.mu.Unlock()
		return 0, 0
	}
	cp := make([]time.Duration, n)
	copy(cp, d.samples[:n])
	d.mu.Unlock()
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	// Nearest-rank: ceil(0.95*n) - 1.
	idx := (95*n+99)/100 - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return cp[idx], n
}

// observeRouteDecision records a route-decision duration and reports the rolling
// p95 against the budget (report-only: WARN over budget, DEBUG otherwise).
func observeRouteDecision(logger *slog.Logger, phase string, dur time.Duration) {
	routeDecisions.record(dur)
	p95, n := routeDecisions.p95()
	if logger == nil {
		return
	}
	attrs := []any{
		"phase", phase,
		"decision_ms", float64(dur.Microseconds()) / 1000,
		"p95_ms", float64(p95.Microseconds()) / 1000,
		"budget_ms", float64(routeDecisionBudget.Milliseconds()),
		"window", n,
	}
	if p95 > routeDecisionBudget {
		logger.Warn("route-decision p95 over budget (report-only)", attrs...)
		return
	}
	logger.Debug("route-decision latency", attrs...)
}
