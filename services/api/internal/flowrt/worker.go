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

// ronaCooldown is how long a non-answering agent is held out of the matcher after
// a missed offer. Short + self-healing (ListAvailableAgentsForMatch treats an
// expired cooldown as routable), so a wrongly-sidelined re-readied agent recovers
// fast even before the state machine writes last_ready_at to activate the fence.
const ronaCooldown = 10 * time.Second

func (e *Endpoints) ProcessDueContinuations(ctx context.Context, pool *pgxpool.Pool, workerID string, now time.Time, lease time.Duration, limit int) (int, error) {
	rawq := generated.New(pool)
	wid := workerID
	// Dead-letter rows that exhausted their attempts but were never cleanly failed
	// (e.g. a worker crashed mid-process), so a poison row can't sit unclaimable
	// forever once its lease expires (re-review H5).
	_, _ = rawq.CancelExhaustedContinuations(ctx, generated.CancelExhaustedContinuationsParams{UpdatedAt: ts(now), AttemptCount: maxContinuationAttempts})
	claimed, err := rawq.ClaimDueContinuations(ctx, generated.ClaimDueContinuationsParams{
		ClaimedAt: ts(now), ClaimedBy: &wid, ClaimExpiresAt: ts(now.Add(lease)), Limit: int32(limit), //nolint:gosec // worker batch limit is small + bounded
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

// SweepCapacity reclaims leaked capacity slots cross-org on the raw pool (like
// the continuation worker): PENDING holds whose timer elapsed (RONA before
// accept) and slots whose reservation already went terminal. Returns the rows
// freed. A confirmed slot for an agent who crashed mid-call is NOT freed here —
// that abandonment is presence-loss driven (W5).
func (e *Endpoints) SweepCapacity(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	rawq := generated.New(pool)
	swept, err := rawq.SweepExpiredCapacityHolds(ctx)
	if err != nil {
		return 0, err
	}
	reclaimed, err := rawq.ReconcileOrphanedSlotsAllOrgs(ctx)
	if err != nil {
		return swept, err
	}
	return swept + reclaimed, nil
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

	// run_seq-fenced acquire: the offer that armed this timer pinned the route's
	// run_seq (SuspendRoute on the inline path, CommitMatchOffer on the matcher
	// path). If the route has since advanced (accepted/rejected → a newer suspend
	// bumped run_seq, or it completed), AtSeq gets 0 rows and the stale timer is a
	// no-op — it can't seize a later wait/offer (cross-AI review MED).
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
	timedOut, tErr := qtx.TimeoutReservation(ctx, generated.TimeoutReservationParams{ID: c.ReservationID, OrgID: c.OrgID})
	if errors.Is(tErr, pgx.ErrNoRows) {
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
	if e.deps.Capacity != nil { // free the slot (gated on the timeout transition above)
		if rErr := e.deps.Capacity.ReleaseInTx(ctx, qtx, orgID, apiUUID(c.ReservationID)); rErr != nil {
			return rErr
		}
	}
	// RONA cooldown: the offered agent didn't answer → keep the matcher from
	// re-ringing them for another route briefly, fenced so a Ready after the offer
	// isn't clobbered. excluded_agent_ids already fences the SAME route on re-queue.
	if _, rErr := qtx.MarkAgentMissed(ctx, generated.MarkAgentMissedParams{
		OrgID: c.OrgID, AgentID: timedOut.AgentID,
		StateExpiresAt: ts(time.Now().Add(ronaCooldown)), LastReadyAt: timedOut.OfferedAt,
	}); rErr != nil {
		return rErr
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
	expired, err := qtx.ExpireWrapUp(ctx, generated.ExpireWrapUpParams{AgentID: c.AgentID, OrgID: c.OrgID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// WrapUp → Ready (post-interaction auto-ready) stamps last_ready_at so the RONA
	// fence sees this fresh availability; NotReady does not.
	if err == nil && expired.Status == "Ready" {
		if rErr := qtx.MarkAgentReady(ctx, generated.MarkAgentReadyParams{OrgID: c.OrgID, AgentID: c.AgentID}); rErr != nil {
			return rErr
		}
	}
	if err := e.resolveDone(ctx, qtx, c, workerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
