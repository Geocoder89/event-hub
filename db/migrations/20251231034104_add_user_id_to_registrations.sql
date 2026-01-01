-- +goose Up
ALTER TABLE registrations
ADD COLUMN user_id UUID;

CREATE INDEX IF NOT EXISTS idx_registrations_user_id ON registrations(user_id);

ALTER TABLE registrations
ADD CONSTRAINT registrations_user_id_fk
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE registrations DROP CONSTRAINT IF EXISTS registrations_user_id_fk;
DROP INDEX IF EXISTS idx_registrations_user_id;
ALTER TABLE registrations DROP COLUMN IF EXISTS user_id;
