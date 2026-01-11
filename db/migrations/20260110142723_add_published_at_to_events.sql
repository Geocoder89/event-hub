-- +goose Up
ALTER TABLE events
ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_events_published_at ON events(published_at);

-- +goose Down
DROP INDEX IF EXISTS idx_events_published_at;
ALTER TABLE events DROP COLUMN IF EXISTS published_at;
