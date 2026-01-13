-- +goose Up

-- enable pgcrypto for gen_random_uuid() (safe if already enabled)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE webhook_outbox
  ADD COLUMN IF NOT EXISTS event_id UUID;

-- backfill existing rows (if any)
UPDATE webhook_outbox
SET event_id = gen_random_uuid()
WHERE event_id IS NULL;

ALTER TABLE webhook_outbox
  ALTER COLUMN event_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_webhook_outbox_event_id
  ON webhook_outbox(event_id);

-- +goose Down
DROP INDEX IF EXISTS ux_webhook_outbox_event_id;
ALTER TABLE webhook_outbox DROP COLUMN IF EXISTS event_id;
