-- +goose Up
ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS check_in_token TEXT;

ALTER TABLE registrations
ADD COLUMN IF NOT EXISTS checked_in_at TIMESTAMPTZ NULL;

UPDATE registrations
SET check_in_token = md5(id::text || ':' || created_at::text)
WHERE check_in_token IS NULL;

ALTER TABLE registrations
ALTER COLUMN check_in_token SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_registrations_event_check_in_token
  ON registrations(event_id, check_in_token);

CREATE INDEX IF NOT EXISTS idx_registrations_event_checked_in_at
  ON registrations(event_id, checked_in_at);

-- +goose Down
DROP INDEX IF EXISTS idx_registrations_event_checked_in_at;
DROP INDEX IF EXISTS idx_registrations_event_check_in_token;
ALTER TABLE registrations DROP COLUMN IF EXISTS checked_in_at;
ALTER TABLE registrations DROP COLUMN IF EXISTS check_in_token;
