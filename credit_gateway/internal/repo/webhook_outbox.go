package repo

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrMissingWebhookURL = errors.New("missing webhook_url")

type OutboxEvent struct {
	EventType     string
	AggregateType string
	AggregateID   int64
	TargetURL     string
	Payload       map[string]any
}

func InsertOutboxEventTx(ctx context.Context, tx pgx.Tx, e OutboxEvent) (uuid.UUID, error) {
	if e.AggregateType == "" {
		e.AggregateType = "merchant_request"
	}
	if e.TargetURL == "" {
		return uuid.Nil, ErrMissingWebhookURL
	}
	if e.Payload == nil {
		e.Payload = map[string]any{}
	}

	eventID := uuid.New()
	e.Payload["event_id"] = eventID.String()
	e.Payload["event_type"] = e.EventType

	b, err := json.Marshal(e.Payload)
	if err != nil {
		return uuid.Nil, err
	}

	_, err = tx.Exec(ctx, `
INSERT INTO webhook_outbox (event_id, event_type, aggregate_type, aggregate_id, target_url, payload, status)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'pending')
`, eventID, e.EventType, e.AggregateType, e.AggregateID, e.TargetURL, string(b))

	if err != nil {
		return uuid.Nil, err
	}

	return eventID, nil
}
