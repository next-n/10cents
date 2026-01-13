package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WebhookOutboxRow struct {
	ID            int64
	EventID       uuid.UUID
	EventType     string
	AggregateType string
	AggregateID   int64
	TargetURL     string
	PayloadJSON   []byte

	AttemptCount int32
	Status       string
}

func ClaimPendingOutboxTx(ctx context.Context, tx pgx.Tx, limit int) ([]WebhookOutboxRow, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := tx.Query(ctx, `
SELECT id, event_id, event_type, aggregate_type, aggregate_id, target_url, payload, attempt_count, status
FROM webhook_outbox
WHERE status = 'pending'
  AND (next_retry_at IS NULL OR next_retry_at <= now())
ORDER BY created_at ASC
FOR UPDATE SKIP LOCKED
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WebhookOutboxRow
	for rows.Next() {
		var r WebhookOutboxRow
		if err := rows.Scan(
			&r.ID,
			&r.EventID,
			&r.EventType,
			&r.AggregateType,
			&r.AggregateID,
			&r.TargetURL,
			&r.PayloadJSON,
			&r.AttemptCount,
			&r.Status,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func ClaimPendingOutbox(ctx context.Context, db *pgxpool.Pool, limit int) ([]WebhookOutboxRow, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	evts, err := ClaimPendingOutboxTx(ctx, tx, limit)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return evts, nil
}

func MarkOutboxSentTx(ctx context.Context, tx pgx.Tx, id int64) error {
	_, err := tx.Exec(ctx, `
UPDATE webhook_outbox
SET status='sent', sent_at=now(), updated_at=now()
WHERE id=$1
`, id)
	return err
}

func MarkOutboxFailedTx(ctx context.Context, tx pgx.Tx, id int64, attempt int32, lastErr string, retryAfter time.Duration) error {
	next := time.Now().UTC().Add(retryAfter)
	_, err := tx.Exec(ctx, `
UPDATE webhook_outbox
SET status='pending',
    attempt_count=$2,
    last_error=$3,
    next_retry_at=$4,
    updated_at=now()
WHERE id=$1
`, id, attempt, lastErr, next)
	return err
}
