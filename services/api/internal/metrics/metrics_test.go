package metrics

import (
	"strings"
	"testing"
)

func TestCounterAndGauge(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("or_test_total", "a counter")
	g := r.Gauge("or_test_active", "a gauge")
	c.Inc()
	c.Add(4)
	g.Inc()
	g.Inc()
	g.Dec()
	if c.Value() != 5 {
		t.Fatalf("counter = %d, want 5", c.Value())
	}
	if g.Value() != 1 {
		t.Fatalf("gauge = %d, want 1", g.Value())
	}
	g.Set(9)
	if g.Value() != 9 {
		t.Fatalf("gauge after Set = %d, want 9", g.Value())
	}
}

func TestWritePrometheusSortedAndTyped(t *testing.T) {
	r := NewRegistry()
	r.Counter("or_z_total", "z").Inc()
	r.Gauge("or_a_active", "a").Set(3)
	var sb strings.Builder
	r.WritePrometheus(&sb)
	out := sb.String()

	// Name-sorted: or_a_active block precedes or_z_total.
	if strings.Index(out, "or_a_active") > strings.Index(out, "or_z_total") {
		t.Fatalf("output not name-sorted:\n%s", out)
	}
	for _, want := range []string{
		"# TYPE or_a_active gauge",
		"or_a_active 3",
		"# TYPE or_z_total counter",
		"or_z_total 1",
		"# HELP or_z_total z",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
