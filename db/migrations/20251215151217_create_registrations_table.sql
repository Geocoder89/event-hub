-- +goose Up
CREATE TABLE IF NOT EXISTS registrations (
  id UUID PRIMARY KEY,
  event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  email TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  CONSTRAINT registrations_event_email_uniq UNIQUE (event_id, email)
);
CREATE INDEX IF NOT EXISTS idx_registrations_event_id ON registrations(event_id);

-- +goose Down
DROP TABLE IF EXISTS registrations;



