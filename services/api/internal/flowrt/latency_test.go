package flowrt

import (
	"testing"
	"time"
)

func TestDecisionLatency_P95(t *testing.T) {
	d := &decisionLatency{samples: make([]time.Duration, decisionWindow)}
	if v, n := d.p95(); v != 0 || n != 0 {
		t.Fatalf("empty p95 = %v,%d want 0,0", v, n)
	}
	// 1..100 ms → p95 (nearest-rank) is the 95th value = 95ms.
	for i := 1; i <= 100; i++ {
		d.record(time.Duration(i) * time.Millisecond)
	}
	v, n := d.p95()
	if n != 100 {
		t.Fatalf("window = %d, want 100", n)
	}
	if v != 95*time.Millisecond {
		t.Fatalf("p95 = %v, want 95ms", v)
	}
}

func TestDecisionLatency_RingWraps(t *testing.T) {
	d := &decisionLatency{samples: make([]time.Duration, decisionWindow)}
	// Overfill so only the last `decisionWindow` samples count; all 5ms → p95 5ms.
	for i := 0; i < decisionWindow*2; i++ {
		d.record(5 * time.Millisecond)
	}
	v, n := d.p95()
	if n != decisionWindow || v != 5*time.Millisecond {
		t.Fatalf("p95 = %v window = %d, want 5ms / %d", v, n, decisionWindow)
	}
}
