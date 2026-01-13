-- +goose Up
INSERT INTO accounts (id, credit_limit_cents, balance_cents, attempt_count, spent_cents)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  100000000, -- $1,000,000.00 = 100,000,000 cents
  0,
  0,
  0
)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM accounts WHERE id = '00000000-0000-0000-0000-000000000001';
