-- +goose Up

CREATE INDEX IF NOT EXISTS idx_events_fts
  ON events
  USING GIN (to_tsvector('simple', coalesce(title,'') || ' ' || coalesce(description,'') || ' ' || coalesce(city,'')));

-- +goose Down

DROP INDEX IF EXISTS idx_events_fts;
