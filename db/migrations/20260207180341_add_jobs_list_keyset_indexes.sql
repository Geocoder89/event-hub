-- +goose Up

-- Admin list: filter by status + stable keyset ordering
CREATE INDEX IF NOT EXISTS idx_jobs_status_updated_at_id
  ON jobs(status, updated_at DESC, id DESC);

-- Admin list: no status filter + stable keyset ordering
CREATE INDEX IF NOT EXISTS idx_jobs_updated_at_id
  ON jobs(updated_at DESC, id DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_jobs_updated_at_id;
DROP INDEX IF EXISTS idx_jobs_status_updated_at_id;
