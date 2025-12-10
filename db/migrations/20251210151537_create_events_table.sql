-- +goose Up
CREATE TABLE IF NOT EXISTS events (
  id UUID PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  city TEXT NOT NULL,
  start_at TIMESTAMPTZ NOT NULL,
  capacity INT NOT NULL CHECK (capacity >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_start_at on events(start_at);
-- +goose down
DROP TABLE IF EXISTS events;

