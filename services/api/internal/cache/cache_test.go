package cache_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/luongdev/open-routing/services/api/internal/cache"
)

// fakeAgent is a stand-in DTO shape — represents the api.Agent type the
// Wave 2 catalog handlers will pass as the generic T. Using a local test
// struct rather than the real api.Agent keeps this package's unit tests
// independent of the OpenAPI codegen (which has no edit dependency here).
type fakeAgent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// newCache spins up an in-process miniredis, wires a go-redis client at
// the miniredis Addr, and returns a *cache.Cache + the *miniredis.Miniredis
// handle so tests can inspect/manipulate the underlying store (Exists,
// FastForward, Close).
func newCache(t *testing.T) (*cache.Cache, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return cache.New(rdb, slog.New(slog.NewTextHandler(io.Discard, nil))), s
}

// TestKey_Format pins D-58: cache.Key(orgID, entity, id) produces
// "or:{orgID}:{entity}:{id}" verbatim. Accepts any-typed inputs so callers
// can pass uuid.UUID, string, or pgtype.UUID interchangeably.
func TestKey_Format(t *testing.T) {
	require.Equal(t, "or:orgA:agents:ag-1", cache.Key("orgA", "agents", "ag-1"))
	require.Equal(t, "or:01900000-0000-7000-8000-000000000000:channels:c-1",
		cache.Key("01900000-0000-7000-8000-000000000000", "channels", "c-1"))
}

// TestGetOrSet_Miss_LoadsAndSets pins the miss-path contract: loader runs
// exactly once, the returned value reaches the caller, and the encoded
// payload lands in Redis under the requested key.
func TestGetOrSet_Miss_LoadsAndSets(t *testing.T) {
	c, s := newCache(t)
	ctx := context.Background()
	key := cache.Key("orgA", "agents", "ag-1")

	loads := atomic.Int32{}
	got, err := cache.GetOrSet[fakeAgent](ctx, c, key, 60*time.Second,
		func(ctx context.Context) (fakeAgent, error) {
			loads.Add(1)
			return fakeAgent{ID: "ag-1", Name: "alice"}, nil
		})

	require.NoError(t, err)
	require.Equal(t, "alice", got.Name)
	require.Equal(t, int32(1), loads.Load(), "miss path MUST invoke loader exactly once")
	require.True(t, s.Exists(key), "miss path MUST write to redis (D-59 60s TTL)")
}

// TestGetOrSet_Hit_NoLoad pins the hit-path contract: a pre-warmed key is
// returned without invoking the loader. Loader counter must stay 0; the
// cached value MUST win even when the loader would return a different
// payload (proves the loader is genuinely skipped, not racing the cache).
func TestGetOrSet_Hit_NoLoad(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	key := cache.Key("orgA", "agents", "ag-1")

	_, err := cache.GetOrSet[fakeAgent](ctx, c, key, 60*time.Second,
		func(ctx context.Context) (fakeAgent, error) {
			return fakeAgent{ID: "ag-1", Name: "alice"}, nil
		})
	require.NoError(t, err)

	loads := atomic.Int32{}
	got, err := cache.GetOrSet[fakeAgent](ctx, c, key, 60*time.Second,
		func(ctx context.Context) (fakeAgent, error) {
			loads.Add(1)
			return fakeAgent{ID: "ag-1", Name: "different"}, nil
		})

	require.NoError(t, err)
	require.Equal(t, "alice", got.Name, "hit MUST return cached value, not fresh loader result")
	require.Equal(t, int32(0), loads.Load(), "hit MUST NOT invoke loader")
}

// TestGetOrSet_ErrNotFound_NotCached pins D-54: a loader returning
// cache.ErrNotFound propagates to the caller (so the handler can map to
// HTTP 404) AND the empty zero-value is NOT written to Redis. This is
// the regression guard for "no negative caching".
func TestGetOrSet_ErrNotFound_NotCached(t *testing.T) {
	c, s := newCache(t)
	ctx := context.Background()
	key := cache.Key("orgA", "agents", "missing")

	_, err := cache.GetOrSet[fakeAgent](ctx, c, key, 60*time.Second,
		func(ctx context.Context) (fakeAgent, error) {
			return fakeAgent{}, cache.ErrNotFound
		})

	require.ErrorIs(t, err, cache.ErrNotFound)
	require.False(t, s.Exists(key), "ErrNotFound MUST NOT write to redis (D-54)")
}

// TestGetOrSet_RefreshAhead pins D-53: when PTTL drops below the 10s
// refresh threshold, the next hit returns the cached value AND fires an
// async refresh goroutine that invokes the loader a second time. The
// goroutine uses context.WithoutCancel so request cancellation does NOT
// kill the refresh (Pitfall 4 guard).
func TestGetOrSet_RefreshAhead(t *testing.T) {
	c, s := newCache(t)
	ctx := context.Background()
	key := cache.Key("orgA", "agents", "ag-1")

	loads := atomic.Int32{}
	loader := func(ctx context.Context) (fakeAgent, error) {
		loads.Add(1)
		return fakeAgent{ID: "ag-1", Name: "v" + string(rune('0'+loads.Load()))}, nil
	}

	// Initial miss + set with 60s TTL (D-59).
	_, err := cache.GetOrSet[fakeAgent](ctx, c, key, 60*time.Second, loader)
	require.NoError(t, err)
	require.Equal(t, int32(1), loads.Load(), "initial miss invokes loader once")

	// Fast-forward miniredis clock so PTTL < 10s (D-53 threshold).
	s.FastForward(51 * time.Second)

	// Next hit should trigger async refresh — loader fires a second time.
	got, err := cache.GetOrSet[fakeAgent](ctx, c, key, 60*time.Second, loader)
	require.NoError(t, err)
	require.NotEmpty(t, got.Name, "hit MUST return the cached value synchronously")

	// Wait briefly for the async refresh goroutine to land.
	require.Eventually(t, func() bool {
		return loads.Load() >= 2
	}, 2*time.Second, 10*time.Millisecond,
		"refresh-ahead goroutine MUST fire when PTTL < refreshThreshold (D-53)")
}

// TestGetOrSet_Singleflight_Dedups pins D-52: 32 concurrent goroutines
// calling GetOrSet for the same cold key must dedup to ONE loader
// invocation. The loader sleeps 20ms to hold the singleflight Do() call
// open long enough for concurrent callers to join. After WaitGroup, the
// loader counter must read exactly 1.
func TestGetOrSet_Singleflight_Dedups(t *testing.T) {
	c, _ := newCache(t)
	ctx := context.Background()
	key := cache.Key("orgA", "agents", "ag-x")

	loads := atomic.Int32{}
	loader := func(ctx context.Context) (fakeAgent, error) {
		loads.Add(1)
		time.Sleep(20 * time.Millisecond)
		return fakeAgent{ID: "ag-x", Name: "alice"}, nil
	}

	var wg sync.WaitGroup
	const N = 32
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cache.GetOrSet[fakeAgent](ctx, c, key, 60*time.Second, loader)
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	require.Equal(t, int32(1), loads.Load(),
		"singleflight MUST dedup concurrent misses to ONE loader call (D-52)")
}

// TestDel pins the invalidation contract: Del removes the cached entry.
// Used by every write handler AFTER a successful DB commit (D-55).
func TestDel(t *testing.T) {
	c, s := newCache(t)
	ctx := context.Background()
	key := cache.Key("orgA", "agents", "ag-1")

	_, err := cache.GetOrSet[fakeAgent](ctx, c, key, 60*time.Second,
		func(ctx context.Context) (fakeAgent, error) {
			return fakeAgent{ID: "ag-1", Name: "alice"}, nil
		})
	require.NoError(t, err)
	require.True(t, s.Exists(key), "pre-warm: key must exist before Del")

	require.NoError(t, c.Del(ctx, key))
	require.False(t, s.Exists(key), "Del MUST remove the key")
}

// TestDel_RedisErrorPropagates pins D-55's caller contract: Del returns
// the underlying Redis error so callers can log+continue (NEVER converting
// a successful DB write into a 5xx). Simulated by closing miniredis.
func TestDel_RedisErrorPropagates(t *testing.T) {
	c, s := newCache(t)
	ctx := context.Background()
	s.Close() // simulate Redis outage
	err := c.Del(ctx, "or:any:key")
	require.Error(t, err,
		"Del MUST propagate redis errors so callers log+continue per D-55")
}
