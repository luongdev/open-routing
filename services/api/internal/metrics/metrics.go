// Package metrics is a dependency-light in-process metric registry rendered in
// Prometheus text-exposition format. v0.3 W6 wires it to the existing /metrics
// route — a real OTel/Prometheus exporter remains an additive swap (the route
// signature is stable), but the engine needs basic counters NOW (gateway
// connections, relay overflow, matcher offers/reclaims) without pulling a client
// library into the build.
package metrics

import (
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
)

type kind int

const (
	counter kind = iota
	gauge
)

// Metric is a single named counter or gauge. Reads/writes are atomic so any
// goroutine (gateway connection, matcher tick) can update it lock-free.
type Metric struct {
	name string
	help string
	typ  kind
	v    atomic.Int64
}

func (m *Metric) Inc()         { m.v.Add(1) }
func (m *Metric) Add(n int64)  { m.v.Add(n) }
func (m *Metric) Dec()         { m.v.Add(-1) }
func (m *Metric) Set(n int64)  { m.v.Store(n) }
func (m *Metric) Value() int64 { return m.v.Load() }

type Registry struct {
	mu      sync.Mutex
	metrics []*Metric
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) register(name, help string, t kind) *Metric {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := &Metric{name: name, help: help, typ: t}
	r.metrics = append(r.metrics, m)
	return m
}

func (r *Registry) Counter(name, help string) *Metric { return r.register(name, help, counter) }
func (r *Registry) Gauge(name, help string) *Metric   { return r.register(name, help, gauge) }

// WritePrometheus renders the registry in text-exposition format, name-sorted so
// the output is stable (scrape diffs + golden tests don't churn on map order).
func (r *Registry) WritePrometheus(w io.Writer) {
	r.mu.Lock()
	ms := make([]*Metric, len(r.metrics))
	copy(ms, r.metrics)
	r.mu.Unlock()
	sort.Slice(ms, func(i, j int) bool { return ms[i].name < ms[j].name })
	for _, m := range ms {
		typ := "counter"
		if m.typ == gauge {
			typ = "gauge"
		}
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %d\n", m.name, m.help, m.name, typ, m.name, m.v.Load())
	}
}

// Default is the process-wide registry the engine writes to and /metrics serves.
var Default = NewRegistry()

// Engine metrics. Names are prefixed or_ (open-routing) and follow Prometheus
// conventions (_total for counters). Kept label-free deliberately — per-org/
// per-status breakdowns are an additive upgrade when a real exporter lands.
var (
	WSConnectionsActive   = Default.Gauge("or_ws_connections_active", "Currently open agent WebSocket connections.")
	WSConnectionsTotal    = Default.Counter("or_ws_connections_total", "Agent WebSocket connections accepted since start.")
	WSConnectionsRejected = Default.Counter("or_ws_connections_rejected_total", "Agent WebSocket upgrades rejected by the per-org connection cap.")
	WSRelayOverflow       = Default.Counter("or_ws_relay_overflow_total", "WS send-buffer overflows that dropped a slow consumer (it reconnects + replays).")
	WSCommandsTotal       = Default.Counter("or_ws_commands_total", "Agent WS commands processed (accept/reject/complete).")
	WSCommandErrors       = Default.Counter("or_ws_command_errors_total", "Agent WS commands that returned an error ack.")

	MatcherOffers   = Default.Counter("or_matcher_offers_total", "Offers attached by the availability-driven matcher.")
	MatcherReclaims = Default.Counter("or_matcher_reclaims_total", "Confirmed-slot reclaims after an agent's presence lapsed mid-call.")
)
