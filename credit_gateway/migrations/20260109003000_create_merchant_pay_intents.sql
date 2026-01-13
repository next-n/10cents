-- +goose Up
CREATE TABLE merchant_pay_intents (
  payment_intent_id   UUID PRIMARY KEY
    REFERENCES payment_intents(id) ON DELETE CASCADE,

  merchant_request_id BIGINT NOT NULL
    REFERENCES merchant_requests(id) ON DELETE CASCADE,

  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_merchant_pay_intents_mr_id
  ON merchant_pay_intents(merchant_request_id);

-- +goose Down
DROP INDEX IF EXISTS idx_merchant_pay_intents_mr_id;
DROP TABLE IF EXISTS merchant_pay_intents;
