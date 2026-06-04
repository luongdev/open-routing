-- Wave 3 live routing runtime. The route_requests row is the single execution-
-- exclusion authority: a process CAS-acquires it (status -> running) before
-- running the executor, parks the cursor on suspend, and releases it. Storing
-- the cursor here (not on continuations) avoids the API<->worker deadlock when
-- accept races a timeout (the API never locks the continuation row).
-- 000001/000002 are already applied; this is a forward migration.

BEGIN;

ALTER TABLE route_requests
    -- Execution position to resume from (node_id + vars + reservation phase).
    -- Set on suspend, read after acquiring the run-lock. NULL when not suspended.
    ADD COLUMN resume_cursor          JSONB,
    -- The reservation the route is currently waiting on (offered), for fast
    -- lookup + cancellation on route cancel.
    ADD COLUMN current_reservation_id UUID,
    -- Bumped on every suspend — a monotonic fence for cursor staleness/debug.
    ADD COLUMN run_seq                INT NOT NULL DEFAULT 0;

COMMIT;
