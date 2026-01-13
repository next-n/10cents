package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func CreatePaymentIntentTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID uuid.UUID,
	amountCents int64,
) (*PaymentIntent, error) {

	const q = `
insert into payment_intents (id, account_id, amount_cents, status)
values ($1, $2, $3, 'pending')
returning id, account_id, amount_cents, status;
`
	var pi PaymentIntent
	pi.ID = uuid.New()

	if err := tx.QueryRow(ctx, q,
		pi.ID,
		accountID,
		amountCents,
	).Scan(
		&pi.ID,
		&pi.AccountID,
		&pi.Amount,
		&pi.Status,
	); err != nil {
		return nil, err
	}

	return &pi, nil
}
