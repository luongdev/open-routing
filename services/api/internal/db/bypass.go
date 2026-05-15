package db

import (
	"context"
	"fmt"
	"runtime"
)

// bypassCtxKey is the unexported context key type that scopes the bypass
// marker. Using an empty-struct unexported type prevents collisions with
// any other package's context key (Go ctx-key anti-pattern S1).
type bypassCtxKey struct{}

// bypassMarker carries the reason and the runtime caller location at the
// point WithBypass was invoked. Both fields are surfaced as structured
// slog fields by orgdb.go preflight so the audit trail captures who
// authorized the bypass and why.
type bypassMarker struct {
	Reason string
	Caller string
}

// WithBypass returns a derived ctx authorizing the next orgDB call to skip
// the org_id filter check. The reason becomes a slog.Warn structured field
// (event=orgdb_bypass) emitted by orgdb.preflight. Phase 1 contract (D-04):
// only cmd/migrate may legitimately call this. Any other caller is a
// security regression — grep-able via the WithBypass identifier itself.
func WithBypass(ctx context.Context, reason string) context.Context {
	return context.WithValue(ctx, bypassCtxKey{}, &bypassMarker{
		Reason: reason,
		Caller: callerInfo(2),
	})
}

// BypassReason returns the bypass reason if ctx carries a marker, and
// ("", false) otherwise. Used by orgdb.preflight step 1 to short-circuit
// the validator and emit the slog audit event.
func BypassReason(ctx context.Context) (string, bool) {
	m, ok := ctx.Value(bypassCtxKey{}).(*bypassMarker)
	if !ok {
		return "", false
	}
	return m.Reason, true
}

// BypassCaller returns the runtime caller info captured by WithBypass.
// Exposed so orgdb.preflight can emit the caller alongside the reason in
// the slog event for forensic clarity (D-05).
func BypassCaller(ctx context.Context) (string, bool) {
	m, ok := ctx.Value(bypassCtxKey{}).(*bypassMarker)
	if !ok {
		return "", false
	}
	return m.Caller, true
}

// callerInfo captures "<file>:<line>" for the call site that invoked
// WithBypass. skip=2 walks past callerInfo + WithBypass to land on the
// real caller. Returns "unknown" if runtime.Caller cannot resolve the
// frame (impossible under normal operation but defensively handled).
func callerInfo(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", file, line)
}
