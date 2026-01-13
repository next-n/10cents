-- +goose Up
CREATE TABLE accounts (
  id              UUID PRIMARY KEY,
  credit_limit_cents BIGINT NOT NULL CHECK (credit_limit_cents >= 0),
  balance_cents      BIGINT NOT NULL CHECK (balance_cents >= 0),

  attempt_count      BIGINT NOT NULL CHECK (attempt_count >= 0),
  spent_cents        BIGINT NOT NULL CHECK (spent_cents >= 0),

  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_accounts_created_at ON accounts(created_at);

-- +goose Down
DROP TABLE accounts;
