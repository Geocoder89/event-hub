-- +goose Up
ALTER TABLE jobs
ADD COLUMN IF NOT EXISTS idempotency_key TEXT NULL;

-- Unique for pending/processing/done jobs
-- simple: one idempotency key maps to one job ever.
CREATE UNIQUE INDEX IF NOT EXISTS jobs_idempotency_key_uniq
ON jobs(idempotency_key)
WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS jobs_idempotency_key_uniq;
ALTER TABLE jobs DROP COLUMN IF EXISTS idempotency_key;
