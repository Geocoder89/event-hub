-- +goose Up
-- requires pgcrypto for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE notification_deliveries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kind TEXT NOT NULL,
  registration_id UUID NOT NULL,
  job_id UUID NOT NULL,
  recipient TEXT NOT NULL,

  status TEXT NOT NULL DEFAULT 'sending', -- sending | sent
  sent_at TIMESTAMPTZ NULL,
  provider_message_id TEXT NULL,
  last_error TEXT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The send-once rule for registration confirmation:
-- one confirmation per registration_id (regardless of job retries or new job ids)
CREATE UNIQUE INDEX notification_deliveries_kind_registration_uniq
  ON notification_deliveries(kind, registration_id);

CREATE INDEX notification_deliveries_status_idx ON notification_deliveries(status);
CREATE INDEX notification_deliveries_sent_at_idx ON notification_deliveries(sent_at);

-- +goose Down
DROP TABLE IF EXISTS notification_deliveries;
