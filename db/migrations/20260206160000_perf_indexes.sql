-- +goose Up

-- EVENTS: optimize filtered list + stable ordering + keyset pagination
CREATE INDEX IF NOT EXISTS idx_events_start_at_id
  ON events(start_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_events_city_start_at_id
  ON events(city, start_at ASC, id ASC);

-- REGISTRATIONS: optimize list-by-event stable ordering
CREATE INDEX IF NOT EXISTS idx_registrations_event_created_id
  ON registrations(event_id, created_at ASC, id ASC);

-- JOBS: speed up claim-next path (optional but useful at scale)
CREATE INDEX IF NOT EXISTS idx_jobs_pending_claim
  ON jobs(priority DESC, run_at ASC, created_at ASC)
  WHERE status = 'pending';

-- JOBS: speed up admin list sorting by updated_at
CREATE INDEX IF NOT EXISTS idx_jobs_status_updated_at
  ON jobs(status, updated_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_jobs_status_updated_at;
DROP INDEX IF EXISTS idx_jobs_pending_claim;

DROP INDEX IF EXISTS idx_registrations_event_created_id;

DROP INDEX IF EXISTS idx_events_city_start_at_id;
DROP INDEX IF EXISTS idx_events_start_at_id;
