-- +goose Up

ALTER TABLE merchant_requests
  RENAME COLUMN merchant_request_id TO merchant_request_reference;

ALTER TABLE merchant_requests
  ADD CONSTRAINT uniq_merchant_reference
  UNIQUE (merchant_id, merchant_request_reference);

CREATE INDEX idx_merchant_requests_reference
  ON merchant_requests (merchant_id, merchant_request_reference);


-- +goose Down

DROP INDEX IF EXISTS idx_merchant_requests_reference;

ALTER TABLE merchant_requests
  DROP CONSTRAINT IF EXISTS uniq_merchant_reference;

ALTER TABLE merchant_requests
  RENAME COLUMN merchant_request_reference TO merchant_request_id;
