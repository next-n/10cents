-- +goose Up
ALTER TABLE merchant_pay_intents
  ADD COLUMN IF NOT EXISTS progressed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_merchant_pay_intents_progressed_at
  ON merchant_pay_intents(progressed_at);

-- +goose Down
DROP INDEX IF EXISTS idx_merchant_pay_intents_progressed_at;
ALTER TABLE merchant_pay_intents DROP COLUMN IF EXISTS progressed_at;
