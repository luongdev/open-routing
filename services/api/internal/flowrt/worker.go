package flowrt

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
func ptrStr(s string) *string           { return &s }

// ProcessDueContinuations claims due continuations across orgs (raw pool + FOR
// UPDATE SKIP LOCKED so replicas claim disjoint rows) and dispatches each. The
// claim is intentionally NOT org-scoped (worker is cross-org); per-continuation
// processing is org-scoped via the OrgDB. Returns the count handled. cmd/runtime
// calls this on a tick.
func (e *Endpoints) ProcessDueContinuations(ctx context.Context, pool *pgxpool.Pool, workerID string, now time.Time, lease time.Duration, limit int) (int, error) {
	rawq := generated.New(pool)
	wid := workerID
	claimed, err := rawq.ClaimDueContinuations(ctx, generated.ClaimDueContinuationsParams{
		ClaimedAt: ts(now), ClaimedBy: &wid, ClaimExpiresAt: ts(now.Add(lease)), Limit: int32(limit),
	})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range claimed {
		if perr := e.processContinuation(ctx, wid, c); perr != nil {
			e.deps.Logger.ErrorContext(ctx, "continuation process failed", "id", apiUUID(c.ID), "kind", c.Kind, "err", perr)
			_, _ = rawq.FailContinuation(ctx, generated.FailContinuationParams{ID: c.ID, OrgID: c.OrgID, ClaimedBy: &wid, LastError: ptrStr(perr.Error())})
			continue
		}
		n++
	}
	return n, nil
}

func (e *Endpoints) processContinuation(ctx context.Context, workerID string, c generated.Continuation) error {
	switch c.Kind {
	case "reservation_timeout":
		return e.fireReservationTimeout(ctx, workerID, c)
	case "wait":
		return e.fireWait(ctx, workerID, c)
	case "wrapup_expiry":
		return e.fireWrapUpExpiry(ctx, workerID, c)
	default:
		return nil
	}
}

func (e *Endpoints) resolveDone(ctx context.Context, qtx *generated.Queries, c generated.Continuation, workerID string) error {
	wid := workerID
	_, err := qtx.ResolveContinuation(ctx, generated.ResolveContinuationParams{ID: c.ID, OrgID: c.OrgID, ClaimedBy: &wid, Status: "done"})
	return err
}

// fireReservationTimeout: acquire the route lock FIRST (serializes against the
// API's accept/reject), then guarded-timeout THE specific offered reservation
// and resume the flow with the "timeout" signal. If the route isn't resumable
// or the reservation was already resolved, it's a no-op (release + mark done).
func (e *Endpoints) fireReservationTimeout(ctx context.Context, workerID string, c generated.Continuation) error {
	orgID := apiUUID(c.OrgID)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)

	route, err := qtx.AcquireRouteForRun(ctx, generated.AcquireRouteForRunParams{ID: c.RouteRequestID, OrgID: c.OrgID})
	if errors.Is(err, pgx.ErrNoRows) {
		if rErr := e.resolveDone(ctx, qtx, c, workerID); rErr != nil {
			return rErr
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if _, tErr := qtx.TimeoutReservation(ctx, generated.TimeoutReservationParams{ID: c.ReservationID, OrgID: c.OrgID}); errors.Is(tErr, pgx.ErrNoRows) {
		// reservation already accepted/rejected/cancelled → release the lock
		if rErr := qtx.ReleaseRoute(ctx, generated.ReleaseRouteParams{ID: route.ID, OrgID: c.OrgID}); rErr != nil {
			return rErr
		}
		if rErr := e.resolveDone(ctx, qtx, c, workerID); rErr != nil {
			return rErr
		}
		return tx.Commit(ctx)
	} else if tErr != nil {
		return tErr
	}
	e.appendEvent(ctx, qtx, orgID, apiUUID(route.ID), "reservation.timeout", map[string]any{"reservation_id": apiUUID(c.ReservationID).String()})
	if err := e.resumeRoute(ctx, tx, qtx, orgID, route, "timeout"); err != nil {
		return err
	}
	if err := e.resolveDone(ctx, qtx, c, workerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (e *Endpoints) fireWait(ctx context.Context, workerID string, c generated.Continuation) error {
	orgID := apiUUID(c.OrgID)
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)
	route, err := qtx.AcquireRouteForRun(ctx, generated.AcquireRouteForRunParams{ID: c.RouteRequestID, OrgID: c.OrgID})
	if errors.Is(err, pgx.ErrNoRows) {
		if rErr := e.resolveDone(ctx, qtx, c, workerID); rErr != nil {
			return rErr
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if err := e.resumeRoute(ctx, tx, qtx, orgID, route, ""); err != nil {
		return err
	}
	if err := e.resolveDone(ctx, qtx, c, workerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (e *Endpoints) fireWrapUpExpiry(ctx context.Context, workerID string, c generated.Continuation) error {
	tx, err := e.deps.OrgDB.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := generated.New(tx)
	_, _ = qtx.ExpireWrapUp(ctx, generated.ExpireWrapUpParams{AgentID: c.AgentID, OrgID: c.OrgID}) // idempotent
	if err := e.resolveDone(ctx, qtx, c, workerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
