// Package cache provides a Redis-backed, type-generic read-through cache
// for catalog single-entity GETs (CAT-11).
//
// This file holds the package-level documentation and the locked-contract
// pointer; the implementation lives in cache.go.
//
// Quick reference for callers:
//
//	c := cache.New(rdb, logger) // boot once
//
//	// In a GetByID handler:
//	key := cache.Key(orgID, "agents", agentID)
//	agent, err := cache.GetOrSet[api.Agent](ctx, c, key, 60*time.Second,
//	    func(ctx context.Context) (api.Agent, error) {
//	        row, qerr := q.GetAgent(ctx, agentID)
//	        if errors.Is(qerr, pgx.ErrNoRows) {
//	            return api.Agent{}, cache.ErrNotFound
//	        }
//	        if qerr != nil { return api.Agent{}, qerr }
//	        return mapAgent(row), nil
//	    })
//
//	// In a write handler, AFTER tx.Commit:
//	if derr := c.Del(ctx, key); derr != nil {
//	    logger.WarnContext(ctx, "cache del", "err", derr) // log+continue
//	}
//
// Locked contract reference: see cache.go header for D-49..D-60 details.
// Threat surface notes: see .planning/phases/03-catalog-crud-go/03-04-PLAN.md
// §threat_model (T-3-12..T-3-16).
package cache
