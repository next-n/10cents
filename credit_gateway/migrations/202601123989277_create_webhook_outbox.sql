-- +goose Up
CREATE TABLE webhook_outbox (
  id              BIGSERIAL PRIMARY KEY,

  -- what happened
  event_type      TEXT NOT NULL,

  -- what it refers to (merchant_requests.id for now)
  aggregate_type  TEXT NOT NULL DEFAULT 'merchant_request',
  aggregate_id    BIGINT NOT NULL,

  -- where to send it
  target_url      TEXT NOT NULL,

  -- what to send
  payload         JSONB NOT NULL,

  -- delivery state
  status          TEXT NOT NULL DEFAULT 'pending', -- pending | sent | failed
  attempt_count   INT  NOT NULL DEFAULT 0,
  next_retry_at   TIMESTAMPTZ,
  last_error      TEXT,

  sent_at         TIMESTAMPTZ,

  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_outbox_pending
  ON webhook_outbox (status, next_retry_at, created_at);

CREATE INDEX idx_webhook_outbox_aggregate
  ON webhook_outbox (aggregate_type, aggregate_id);

-- +goose Down
DROP TABLE webhook_outbox;
