-- +goose Up
CREATE TABLE payment_intents (
  id              UUID PRIMARY KEY,
  account_id      UUID NOT NULL REFERENCES accounts(id),
  amount_cents    BIGINT NOT NULL,
  status          TEXT NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_intents_account_id ON payment_intents(account_id);

-- +goose Down
DROP TABLE payment_intents;
