-- +goose Up
ALTER TABLE events
ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT '';

ALTER TABLE events
ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}'::text[];

CREATE INDEX IF NOT EXISTS idx_events_category_active
  ON events(category)
  WHERE deleted_at IS NULL AND category <> '';

CREATE INDEX IF NOT EXISTS idx_events_tags_active
  ON events
  USING GIN(tags)
  WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_events_tags_active;
DROP INDEX IF EXISTS idx_events_category_active;
ALTER TABLE events DROP COLUMN IF EXISTS tags;
ALTER TABLE events DROP COLUMN IF EXISTS category;
