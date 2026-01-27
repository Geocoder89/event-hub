-- +goose Up
ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS user_id UUID NULL;

CREATE INDEX IF NOT EXISTS idx_jobs_user_id ON jobs(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_user_id;
ALTER TABLE jobs DROP COLUMN IF EXISTS user_id;
