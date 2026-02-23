-- +goose Up
CREATE TABLE IF NOT EXISTS registration_csv_exports (
  job_id UUID PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
  event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  requested_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
  file_name TEXT NOT NULL,
  content_type TEXT NOT NULL DEFAULT 'text/csv',
  row_count INT NOT NULL DEFAULT 0,
  csv_data BYTEA NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_registration_csv_exports_event_created
  ON registration_csv_exports(event_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_registration_csv_exports_event_created;
DROP TABLE IF EXISTS registration_csv_exports;
