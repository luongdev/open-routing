package flowrt

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/luongdev/open-routing/services/api/internal/db/generated"
	"github.com/luongdev/open-routing/services/api/internal/db/orgkey"
)

// errLostLease aborts continuation processing when the fenced resolve finds the
// row was re-claimed by another worker (lease expired mid-flight) — the tx rolls
// back so the reclaiming worker owns the side effects.
var errLostLease = errors.New("flowrt: continuation lease lost (reclaimed)")

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
func ptrStr(s string) *string           { return &s }

// ProcessDueContinuations claims due continuations across orgs (raw pool + FOR
// UPDATE SKIP LOCKED so replicas claim disjoint rows) and dispatches each. The
// claim is intentionally NOT org-scoped (worker is cross-org); per-continuation
// processing is org-scoped via the OrgDB. Returns the count handled. cmd/runtime
// calls this on a tick.
// maxContinuationAttempts bounds retries before a poison row is dead-lettered
// (cancelled) instead of being reclaimed at the head of the queue forever
// (review H6).
const maxContinuationAttempts = 5

func (e *Endpoints) ProcessDueContinuations(ctx context.Context, pool *pgxpool.Pool, workerID string, now time.Time, lease time.Duration, limit int) (int, error) {
	rawq := generated.New(pool)
	wid := workerID
	claimed, err := rawq.ClaimDueContinuations(ctx, generated.ClaimDueContinuationsParams{
		ClaimedAt: ts(now), ClaimedBy: &wid, ClaimExpiresAt: ts(now.Add(lease)), Limit: int32(limit),
		AttemptCount: maxContinuationAttempts,
	})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range claimed {
		perr := e.processContinuation(ctx, wid, c)
		if perr == nil {
			n++
			continue
		}
		// errLostLease means another worker reclaimed the row mid-flight — NOT a
		// failure of this row; leave it for that worker.
		if errors.Is(perr, errLostLease) {
			continue
		}
		e.deps.Logger.ErrorContext(ctx, "continuation process failed", "org_id", apiUUID(c.OrgID), "id", apiUUID(c.ID), "kind", c.Kind, "attempt", c.AttemptCount, "err", perr)
		// Dead-letter only once retries are exhausted; otherwise let the claim
		// lease expire so the row is retried (a transient DB error must not strand
		// the route forever — review H6).
		if c.AttemptCount >= maxContinuationAttempts {
			_, _ = rawq.FailContinuation(ctx, generated.FailContinuationParams{ID: c.ID, OrgID: c.OrgID, ClaimedBy: &wid, LastError: ptrStr(perr.Error())})
		}
	}
	return n, nil
}

func (e *Endpoints) processContinuation(ctx context.Context, workerID string, c generated.Continuation) error {
	// OrgDB preflight reads the org from ctx; the worker's ctx is cross-org, so
	// scope it to THIS continuation's org before any org-scoped query.
	ctx = orgkey.SetOrgID(ctx, apiUUID(c.OrgID))
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
	rows, err := qtx.ResolveContinuation(ctx, generated.ResolveContinuationParams{ID: c.ID, OrgID: c.OrgID, ClaimedBy: &wid, Status: "done"})
	if err != nil {
		return err
	}
	if rows != 1 {
		return errLostLease // another worker reclaimed this row mid-flight → roll back our side effects
	}
	return nil
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
	// run_seq-fenced acquire: a stale wait timer (the route advanced past this
	// suspend, or completed) gets 0 rows and is resolved as a no-op (review B1).
	route, err := qtx.AcquireRouteForRunAtSeq(ctx, generated.AcquireRouteForRunAtSeqParams{ID: c.RouteRequestID, OrgID: c.OrgID, RunSeq: c.RunSeq})
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
	// Idempotent, but a real DB error must surface for retry — only a guard-miss
	// (ErrNoRows) is the benign no-op (review M6).
	if _, err := qtx.ExpireWrapUp(ctx, generated.ExpireWrapUpParams{AgentID: c.AgentID, OrgID: c.OrgID}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err := e.resolveDone(ctx, qtx, c, workerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
