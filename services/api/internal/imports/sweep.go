// sweep.go — D5-11 crash-recovery sweep.
//
// Three entry points mirror state/ttl.go verbatim, parameterised to
// import_jobs:
//
//   - safetySweep      — 1h-tick goroutine; for-select over ctx.Done()
//     vs ticker.Chan(); per-tick body calls runSweepPastDue.
//   - runSweepPastDue  — bounded 30s ctx; calls SweepCrashedImportJobs
//     under db.WithBypass(ctx, "import_crash_sweep"); emits one
//     slog.Warn per flipped row.
//   - startupSweep     — synchronous flavour called from Importer.Start
//     BEFORE the safetySweep goroutine spawns. Same SQL via the
//     ".startup" bypass marker for audit-trail distinction.
//
// SweepCrashedImportJobs is the SOLE org-agnostic query in the entire
// codebase. Its WHERE clause omits org_id; the SQLChecker would
// normally reject it. We pass through via db.WithBypass — and ONLY
// this call site does. The locked bypass markers are:
//
//   - "import_crash_sweep"          — 1h ticker calls.
//   - "import_crash_sweep.startup"  — startup-time synchronous call.
//
// Both markers surface in slog.Warn(event=orgdb_bypass) emitted by
// orgdb.preflight so the audit log distinguishes the two paths.
//
// Pattern source: services/api/internal/state/ttl.go safetySweep +
// runSweepPastDue + startupSweep. Phase 4 STATE-07. The structural
// mirror is intentional — both phases use a single-replica
// time.Ticker pattern (STATE.md "multi-replica TTL" blocker covers
// both call sites equally).
package imports

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/luongdev/open-routing/services/api/internal/db"
	"github.com/luongdev/open-routing/services/api/internal/db/generated"
)

// safetySweep is the long-running goroutine spawned by Start. Each
// tick on s.clock.NewTicker(s.sweepInterval) triggers runSweepPastDue.
// The select also watches s.ctx.Done() so Stop's cancel() drains the
// goroutine deterministically.
//
// Replaces the placeholder in handlers.go (Task 1) by virtue of the
// edit there that removes the //nolint:unused stub.
func (s *Importer) safetySweep() {
	defer s.wg.Done()
	ticker := s.clock.NewTicker(s.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.Chan():
			s.runSweepPastDue()
		}
	}
}

// runSweepPastDue is the per-tick body extracted so tests can call it
// deterministically without spinning the ticker.
//
// Bounded 30s ctx so a hung DB call cannot leak the goroutine; the
// db.WithBypass wrap installs the audit marker that orgdb.preflight
// recognises as the legitimate carve-out for the SOLE org-agnostic
// query in the codebase (SweepCrashedImportJobs).
func (s *Importer) runSweepPastDue() {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	ctx = db.WithBypass(ctx, "import_crash_sweep")

	q := generated.New(s.deps.OrgDB)
	rows, err := q.SweepCrashedImportJobs(ctx)
	if err != nil {
		s.deps.Logger.WarnContext(ctx, "import.crash_sweep.failed", "err", err)
		return
	}
	for _, r := range rows {
		s.deps.Logger.WarnContext(ctx, "import.crash_sweep",
			"event", "import_crash_sweep",
			"import_job_id", uuid.UUID(r.ID.Bytes),
			"org_id", uuid.UUID(r.OrgID.Bytes),
		)
	}
}

// startupSweep runs synchronously inside Start(ctx). Per D-95 / D5-11
// it blocks Start from returning so no stuck pending row survives a
// process restart unobserved.
//
// Distinct bypass marker (".startup" suffix) so the orgdb audit log
// distinguishes one-shot startup sweeps from the 1h tick sweeps.
// Failure propagates back up to Start which aborts the process —
// kubernetes restarts with a fresh attempt rather than serving with
// an unswept queue.
func (s *Importer) startupSweep(ctx context.Context) error {
	ctx = db.WithBypass(ctx, "import_crash_sweep.startup")
	q := generated.New(s.deps.OrgDB)
	rows, err := q.SweepCrashedImportJobs(ctx)
	if err != nil {
		return fmt.Errorf("imports: startup sweep: %w", err)
	}
	for _, r := range rows {
		s.deps.Logger.WarnContext(ctx, "import.crash_sweep.startup",
			"event", "import_crash_sweep.startup",
			"import_job_id", uuid.UUID(r.ID.Bytes),
			"org_id", uuid.UUID(r.OrgID.Bytes),
		)
	}
	return nil
}
