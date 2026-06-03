package runtime

import (
	"sync"
	"time"
)

// Clock is the runtime's only time source. Nodes and the executor MUST read it
// (never time.Now()) so a flow is deterministic under the simulator's virtual
// clock and reproducible on replay.
type Clock interface {
	Now() time.Time
	// Advance moves a virtual clock forward; a real clock ignores it. The
	// executor calls Advance when a `wait`/timeout continuation fires during
	// simulation so downstream Now() reads reflect the elapsed delay.
	Advance(d time.Duration)
}

// RealClock is the production clock: Now() is wall time, Advance is a no-op.
type RealClock struct{}

func (RealClock) Now() time.Time        { return time.Now() }
func (RealClock) Advance(time.Duration) {}

// VirtualClock is the deterministic clock for simulation and replay. Start it
// at a pinned instant; Advance moves it forward explicitly. Safe for concurrent
// reads (the executor is single-goroutine per run, but Emit/observers may read).
type VirtualClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewVirtualClock(start time.Time) *VirtualClock { return &VirtualClock{now: start} }

func (c *VirtualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *VirtualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d > 0 {
		c.now = c.now.Add(d)
	}
}
