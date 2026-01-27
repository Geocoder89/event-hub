-- +goose Up

ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0;

-- Replace old index with priority-aware claim index
DROP INDEX IF EXISTS idx_jobs_status_runat;

CREATE INDEX IF NOT EXISTS idx_jobs_claim_priority
  ON jobs (status, priority DESC, run_at, created_at);

-- +goose Down

DROP INDEX IF EXISTS idx_jobs_claim_priority;

ALTER TABLE jobs
  DROP COLUMN IF EXISTS priority;
