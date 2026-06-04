package presence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisStore is the production lease store. Renew is a plain SET-with-TTL (a
// reconnect with a new session id takes over the agent's lease). Drop is a Lua
// compare-and-delete keyed on the session token so a stale teardown is a no-op.
type RedisStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRedisStore(rdb *redis.Client, ttl time.Duration) *RedisStore {
	if ttl <= 0 {
		ttl = DefaultTTL * time.Second
	}
	return &RedisStore{rdb: rdb, ttl: ttl}
}

func (s *RedisStore) Renew(ctx context.Context, org, agent uuid.UUID, sessionID string) error {
	return s.rdb.Set(ctx, key(org, agent), sessionID, s.ttl).Err()
}

func (s *RedisStore) Connected(ctx context.Context, org, agent uuid.UUID) (bool, error) {
	n, err := s.rdb.Exists(ctx, key(org, agent)).Result()
	if err != nil {
		return false, err // unreachable → infra error; caller must NOT read as "disconnected"
	}
	return n > 0, nil
}

// ConnectedMany batches the lease check with one MGET. A nil value = the key is
// absent (not connected). An MGET error propagates (infra error — callers must
// not read it as "everyone disconnected").
func (s *RedisStore) ConnectedMany(ctx context.Context, org uuid.UUID, agents []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(agents))
	if len(agents) == 0 {
		return out, nil
	}
	keys := make([]string, len(agents))
	for i, a := range agents {
		keys[i] = key(org, a)
	}
	vals, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	for i, a := range agents {
		out[a] = i < len(vals) && vals[i] != nil
	}
	return out, nil
}

// dropCAS deletes the lease only when its stored value still equals the caller's
// session id — so a late Drop from an old connection can't evict a fresh one.
var dropCAS = redis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
  return redis.call('del', KEYS[1])
end
return 0
`)

func (s *RedisStore) Drop(ctx context.Context, org, agent uuid.UUID, sessionID string) error {
	return dropCAS.Run(ctx, s.rdb, []string{key(org, agent)}, sessionID).Err()
}
