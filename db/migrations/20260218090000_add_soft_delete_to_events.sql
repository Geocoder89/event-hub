-- +goose Up
ALTER TABLE events
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_events_deleted_at ON events(deleted_at);

CREATE INDEX IF NOT EXISTS idx_events_active_start_at_id
  ON events(start_at, id)
  WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_events_active_start_at_id;
DROP INDEX IF EXISTS idx_events_deleted_at;
ALTER TABLE events DROP COLUMN IF EXISTS deleted_at;
