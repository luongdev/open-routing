// Package cache implements the v0.1 Redis-backed read-through cache for
// single-entity GETs in the catalog (CAT-11). It is intentionally
// entity-agnostic: callers pass typed loaders via GetOrSet[T any] and
// receive typed values back. One *Cache is constructed at boot and
// shared across every catalog handler (Phase 3 wires it via catalog.Deps;
// Phase 4 will reuse it for agent-status lookups).
//
// Locked contracts (.planning/phases/03-catalog-crud-go/03-CONTEXT.md
// §Implementation Decisions D-49..D-60):
//
//   - D-49: cache contains DTO values (e.g., api.Agent), NOT sqlc rows.
//     Caching sits AFTER the sqlc→DTO mapping step in each handler.
//   - D-50: one shared *Cache for all entity types — generics, not
//     per-entity instances.
//   - D-51: GetOrSet[T any] takes a typed loader and returns a typed
//     value; JSON marshal/unmarshal is internal.
//   - D-52: miss path is deduped via singleflight.Group per process so
//     concurrent requests for the same key issue one DB load.
//   - D-53: refresh-ahead fires when PTTL < refreshThreshold (10s),
//     async, via context.WithoutCancel so request cancel does NOT kill
//     the refresh. Pitfall 4 (Phase 3 RESEARCH) explicitly requires this
//     so trace_id + org_id propagate into the refresh goroutine.
//   - D-54: no negative caching — a loader returning ErrNotFound
//     propagates without writing an empty value. Catalog handlers map
//     this to HTTP 404.
//   - D-55: Del is best-effort. Callers MUST log+continue on failure —
//     a cache deletion failure NEVER converts a successful DB write
//     into a 5xx. The microsecond race between commit and DEL is
//     acceptable; the cache TTLs out in 60s either way.
//   - D-57: observability via slog only (no OTel metrics in v0.1).
//     Every cache call emits slog.DebugContext(ctx, "cache", "key", k,
//     "outcome", "hit"|"miss"|"refresh"|"del"|"error").
//   - D-58: keys are "or:{orgID}:{entity}:{id}" verbatim; see Key().
//   - D-59: TTL is 60s — passed in by callers (not constanted here so
//     test callers can use shorter windows when convenient).
//   - D-60: payload codec is encoding/json (stdlib). No msgpack, no
//     json-iterator. Codegen-drift CI catches any DTO break.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// ErrNotFound is the sentinel a loader returns to signal "row absent"
// so GetOrSet can propagate the not-found condition WITHOUT writing an
// empty cache entry. Catalog handlers wrap this into the typed 404
// JSONResponse defined by the strict-server generated code (D-54).
var ErrNotFound = errors.New("cache: not found")

// refreshThreshold is the locked 10s window from D-53. When PTTL(key)
// drops below this on a hit, GetOrSet spawns an async refresh goroutine
// so the next request is served from a freshly populated cache.
const refreshThreshold = 10 * time.Second

// refreshTimeout bounds the background refresh goroutine so a hung
// loader (stuck DB, network partition) cannot leak the goroutine
// indefinitely. 10s is generous relative to typical load() latency
// while remaining short enough to free up workers (Wave 1 gemini
// review).
const refreshTimeout = 10 * time.Second

// Cache wraps a Redis client + a singleflight group + a slog logger.
// Constructed once at boot via New(rdb, logger) and passed into every
// consumer via the catalog.Deps struct (Wave 2 wiring).
//
// The singleflight group is process-local — cross-instance stampede
// protection (Redis SETNX lock) is deliberately NOT in v0.1 per D-52.
// Bounded fan-out of one DB load per process per key per miss is
// acceptable at v0.1 catalog scale.
type Cache struct {
	rdb    *redis.Client
	sf     *singleflight.Group
	logger *slog.Logger
}

// New constructs a Cache. Both rdb and logger MUST be non-nil; passing
// nil logger will panic on the first slog call. The caller is
// responsible for the rdb lifecycle (typically closed in main.go after
// the HTTP server shuts down).
func New(rdb *redis.Client, logger *slog.Logger) *Cache {
	return &Cache{
		rdb:    rdb,
		sf:     &singleflight.Group{},
		logger: logger,
	}
}

// Key returns the canonical cache key "or:{orgID}:{entity}:{id}" per
// D-58. Helper constructor — every catalog handler calls this rather
// than building the string manually. Accepts any-typed arguments so
// callers can pass uuid.UUID, string, pgtype.UUID, or other Stringer
// implementations interchangeably; fmt.Sprintf %v defends against bad
// caller usage by falling back to default value formatting when the
// argument lacks a Stringer (avoids "%!s(...)" output — Wave 1 codex
// review).
func Key(orgID, entity, id any) string {
	return fmt.Sprintf("or:%v:%v:%v", orgID, entity, id)
}

// GetOrSet looks up `key` in Redis; on hit, JSON-unmarshals the payload
// to T and returns it. On miss it invokes `load`, marshals the result,
// writes it with `ttl`, and returns. Concurrent misses for the same key
// are deduped via singleflight (D-52). On hit, if remaining PTTL drops
// below refreshThreshold, spawns an async refresh goroutine using
// context.WithoutCancel(ctx) so the request finishing does NOT cancel
// the refresh (D-53; Pitfall 4 guard preserves trace_id + org_id).
//
// If `load` returns ErrNotFound (or wraps it via errors.Is), the
// not-found condition propagates to the caller WITHOUT caching the
// empty value (D-54).
//
// Cache layer errors (Redis Get/Set/PTTL failures, JSON unmarshal
// failures) are logged via slog but never propagated when the loader
// can serve the request — the DB remains the source of truth, and a
// Redis outage MUST NOT turn into a 5xx (D-55 spirit).
func GetOrSet[T any](
	ctx context.Context,
	c *Cache,
	key string,
	ttl time.Duration,
	load func(context.Context) (T, error),
) (T, error) {
	var zero T

	// (1) Hit path. rdb.Get returns redis.Nil for an absent key, the
	// payload bytes for a hit, or a non-Nil error for a real Redis
	// outage. Treat any non-Nil error as "fall through to loader" so
	// Redis problems never block reads.
	raw, gerr := c.rdb.Get(ctx, key).Bytes()
	if gerr == nil {
		var v T
		if jerr := json.Unmarshal(raw, &v); jerr == nil {
			c.logger.DebugContext(ctx, "cache", "key", key, "outcome", "hit")
			// (1a) Refresh-ahead check (D-53). If the remaining PTTL is
			// below refreshThreshold, kick off an async re-load. The
			// goroutine MUST use context.WithoutCancel so request cancel
			// doesn't kill the refresh (Pitfall 4) — values like trace_id
			// and org_id stay attached for log correlation.
			if pttl, perr := c.rdb.PTTL(ctx, key).Result(); perr == nil && pttl > 0 && pttl < refreshThreshold {
				// Bound the background refresh with a hard timeout so a
				// hung load() (e.g. stuck DB query, network partition) can
				// never leak a goroutine indefinitely. WithoutCancel keeps
				// trace_id + org_id attached for log correlation even when
				// the request finishes (Pitfall 4); WithTimeout adds the
				// upper bound (Wave 1 gemini review).
				bgCtx, bgCancel := context.WithTimeout(context.WithoutCancel(ctx), refreshTimeout)
				go func() {
					defer bgCancel()
					_, _, _ = c.sf.Do("refresh:"+key, func() (any, error) {
						c.logger.DebugContext(bgCtx, "cache",
							"key", key, "outcome", "refresh")
						nv, lerr := load(bgCtx)
						if lerr != nil {
							c.logger.WarnContext(bgCtx, "cache refresh load",
								"key", key, "outcome", "error", "err", lerr)
							return nil, lerr
						}
						nb, jerr := json.Marshal(nv)
						if jerr != nil {
							c.logger.WarnContext(bgCtx, "cache refresh marshal",
								"key", key, "outcome", "error", "err", jerr)
							return nil, jerr
						}
						if serr := c.rdb.Set(bgCtx, key, nb, ttl).Err(); serr != nil {
							c.logger.WarnContext(bgCtx, "cache refresh set",
								"key", key, "outcome", "error", "err", serr)
						}
						return nv, nil
					})
				}()
			}
			return v, nil
		} else {
			// Unmarshal failure — log warn and fall through to loader.
			// Covers schema drift (forward-incompatible cached payload).
			c.logger.WarnContext(ctx, "cache unmarshal",
				"key", key, "outcome", "error", "err", jerr)
		}
	} else if !errors.Is(gerr, redis.Nil) {
		// Real Redis error (timeout, connection refused, etc.). Log and
		// fall through — the cache is best-effort per D-55 spirit.
		c.logger.WarnContext(ctx, "cache get",
			"key", key, "outcome", "error", "err", gerr)
	}

	// (2) Miss path: dedup concurrent identical misses via singleflight
	// (D-52). The closure either succeeds (returns the loaded value) or
	// fails (returns the loader error). On success we Set; on
	// ErrNotFound we do NOT Set (D-54).
	res, sferr, _ := c.sf.Do(key, func() (any, error) {
		c.logger.DebugContext(ctx, "cache", "key", key, "outcome", "miss")
		nv, lerr := load(ctx)
		if lerr != nil {
			// Surface loader errors verbatim; ErrNotFound MUST be
			// distinguishable to the outer caller (errors.Is) and MUST
			// NOT be written to Redis (D-54).
			return nv, lerr
		}
		nb, jerr := json.Marshal(nv)
		if jerr != nil {
			// Marshal failure on a successful DB load is a programmer
			// bug (non-JSON-able DTO). Log and return the typed value
			// so the caller still gets it — we just can't cache.
			c.logger.WarnContext(ctx, "cache marshal",
				"key", key, "outcome", "error", "err", jerr)
			return nv, nil
		}
		if serr := c.rdb.Set(ctx, key, nb, ttl).Err(); serr != nil {
			// Set failure logs but does NOT propagate (D-55 spirit):
			// the request still returns the loaded value. The next
			// miss will retry.
			c.logger.WarnContext(ctx, "cache set",
				"key", key, "outcome", "error", "err", serr)
		}
		return nv, nil
	})
	if sferr != nil {
		if errors.Is(sferr, ErrNotFound) {
			return zero, ErrNotFound
		}
		return zero, sferr
	}
	typed, ok := res.(T)
	if !ok {
		// Defensive: should be unreachable. singleflight returns
		// whatever load() returned; if two callers used the same key
		// with different T's (programmer error), surface the
		// mismatch as a typed error rather than panicking.
		return zero, fmt.Errorf("cache: singleflight produced unexpected type %T for key %q", res, key)
	}
	return typed, nil
}

// Ping checks Redis liveness. Used by the /readyz bypass handler to fail
// the readiness probe when Redis is unreachable. go-redis's Ping respects
// the supplied ctx deadline so the call cannot hang past the handler's
// timeout budget (mitigation for T-3-43 DoS).
func (c *Cache) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Del removes a cache entry. Returns the underlying Redis error so the
// caller can log+continue per D-55. A cache deletion failure NEVER
// converts a successful DB write into a 5xx — every catalog write
// handler treats Del as best-effort.
func (c *Cache) Del(ctx context.Context, key string) error {
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		c.logger.WarnContext(ctx, "cache",
			"key", key, "outcome", "error", "op", "del", "err", err)
		return err
	}
	c.logger.DebugContext(ctx, "cache", "key", key, "outcome", "del")
	return nil
}
