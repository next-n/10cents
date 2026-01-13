-- +goose Up
CREATE TABLE ledger_entries (
  id            UUID PRIMARY KEY,
  account_id    UUID NOT NULL REFERENCES accounts(id),
  payment_intent_id UUID,
  entry_type    TEXT NOT NULL, -- principal | interest | penalty
  amount_cents  BIGINT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ledger_account ON ledger_entries(account_id);

-- +goose Down
DROP TABLE ledger_entries;
