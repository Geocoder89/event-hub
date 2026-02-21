-- +goose Up

CREATE TABLE IF NOT EXISTS admin_action_audits (
  id UUID PRIMARY KEY,
  actor_user_id TEXT NULL,
  actor_email TEXT NULL,
  actor_role TEXT NOT NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT NULL,
  request_id TEXT NULL,
  status_code INT NOT NULL,
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_action_audits_created_at
  ON admin_action_audits(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_action_audits_actor_created_at
  ON admin_action_audits(actor_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_action_audits_action_created_at
  ON admin_action_audits(action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_action_audits_resource_created_at
  ON admin_action_audits(resource_type, resource_id, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_admin_action_audits_resource_created_at;
DROP INDEX IF EXISTS idx_admin_action_audits_action_created_at;
DROP INDEX IF EXISTS idx_admin_action_audits_actor_created_at;
DROP INDEX IF EXISTS idx_admin_action_audits_created_at;

DROP TABLE IF EXISTS admin_action_audits;
