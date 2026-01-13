package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrMerchantRequestNotPayable = errors.New("merchant request not payable")

func GetMerchantRequestByIDForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	id int64,
) (*MerchantRequest, error) {

	const q = `
select
  id,
  merchant_id,
  merchant_request_id,
  payer_account_id,
  target_cents,
  paid_cents,
  status,
  webhook_url,
  completed_at,
  created_at,
  updated_at
from merchant_requests
where id = $1
for update;
`
	var mr MerchantRequest
	if err := tx.QueryRow(ctx, q, id).Scan(
		&mr.ID,
		&mr.MerchantID,
		&mr.MerchantRequestID,
		&mr.PayerAccountID,
		&mr.TargetCents,
		&mr.PaidCents,
		&mr.Status,
		&mr.WebhookURL,
		&mr.CompletedAt,
		&mr.CreatedAt,
		&mr.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &mr, nil
}

func IncrementMerchantRequestProgress(
	ctx context.Context,
	tx pgx.Tx,
	merchantRequestID int64,
	deltaCents int64,
) (paid int64, target int64, completedNow bool, err error) {

	// lock + load fields needed for completion + webhook
	const lockQ = `
select paid_cents, target_cents, status, merchant_id, merchant_request_id, webhook_url, payer_account_id
from merchant_requests
where id = $1
for update;
`
	var (
		status       string
		merchantID   string
		merchantRef  *string
		webhookURL   *string
		payerAccount string
	)
	if err = tx.QueryRow(ctx, lockQ, merchantRequestID).Scan(
		&paid, &target, &status,
		&merchantID, &merchantRef, &webhookURL, &payerAccount,
	); err != nil {
		return
	}

	if status != "pending" {
		err = ErrMerchantRequestNotPayable
		return
	}

	// update progress
	const updateQ = `
update merchant_requests
set paid_cents = paid_cents + $2
where id = $1;
`
	if _, err = tx.Exec(ctx, updateQ, merchantRequestID, deltaCents); err != nil {
		return
	}

	// mark completed + capture timestamp only if crossing threshold
	const completeQ = `
update merchant_requests
set status = 'completed',
    completed_at = now()
where id = $1
  and status = 'pending'
  and paid_cents >= target_cents
returning completed_at;
`
	var completedAt *time.Time
	if err2 := tx.QueryRow(ctx, completeQ, merchantRequestID).Scan(&completedAt); err2 == nil {
		completedNow = true
	} else {
		// if no row returned, it means not completed yet
		completedNow = false
	}

	// read back latest paid/target
	const readBackQ = `
select paid_cents, target_cents, status
from merchant_requests
where id = $1;
`
	if err = tx.QueryRow(ctx, readBackQ, merchantRequestID).Scan(&paid, &target, &status); err != nil {
		return
	}

	// enqueue webhook only on transition to completed
	if completedNow {
		if webhookURL == nil || *webhookURL == "" {
			// decide policy: either error OR just skip.
			// For "banking level", I'd error so you notice configuration issues.
			err = ErrMissingWebhookURL
			return
		}

		payload := map[string]any{
			"event":                      "merchant_request.completed",
			"gateway_request_id":         merchantRequestID,
			"merchant_id":                merchantID,
			"merchant_request_reference": merchantRef, // pointer => null if missing
			"payer_account_id":           payerAccount,
			"target_cents":               target,
			"paid_cents":                 paid,
			"completed_at":               completedAt,
		}

		_, err = InsertOutboxEventTx(ctx, tx, OutboxEvent{
			EventType:     "merchant_request.completed",
			AggregateType: "merchant_request",
			AggregateID:   merchantRequestID,
			TargetURL:     *webhookURL,
			Payload:       payload,
		})
		if err != nil {
			return
		}
	}

	return
}
