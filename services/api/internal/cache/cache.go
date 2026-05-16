// Package cache wraps go-redis with a typed read-through GetOrSet helper
// (D-50/D-51), singleflight miss-dedup (D-52), and refresh-ahead at 10s
// remaining (D-53). RED-phase stub — the GREEN commit fills in the body.
package cache
