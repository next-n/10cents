-- +goose Up
ALTER TABLE accounts
ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
CHECK (status IN ('active', 'locked'));

-- +goose Down
ALTER TABLE accounts
DROP COLUMN status;
