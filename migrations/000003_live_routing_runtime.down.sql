BEGIN;

ALTER TABLE route_requests
    DROP COLUMN IF EXISTS resume_cursor,
    DROP COLUMN IF EXISTS current_reservation_id,
    DROP COLUMN IF EXISTS run_seq;

COMMIT;
