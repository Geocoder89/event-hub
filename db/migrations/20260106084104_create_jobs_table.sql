-- +goose Up
CREATE TABLE IF NOT EXISTS jobs (
  id UUID PRIMARY KEY,
  type TEXT NOT NULL,
  payload JSONB NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending','processing', 'done','failed')),
  attempts INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 25,
  run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMPTZ NULL,
  locked_by TEXT NULL,
  last_error TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  
);

-- Fast lookup by status + scheduling
CREATE INDEX IF NOT EXISTS idx_jobs_status_runat ON jobs(status, run_at);
CREATE INDEX IF NOT EXISTS idx_jobs_locked_at ON jobs(locked_at);
  -- Optional: query by type
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);

-- +goose Down
DROP TABLE IF EXISTS jobs;


