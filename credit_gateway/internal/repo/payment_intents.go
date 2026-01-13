package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentIntent struct {
	ID        uuid.UUID
	AccountID uuid.UUID
	Amount    int64
	Status    string
}

func CreatePaymentIntent(
	ctx context.Context,
	db *pgxpool.Pool,
	accountID uuid.UUID,
	amountCents int64,
) (*PaymentIntent, error) {
	id := uuid.New()

	const q = `
INSERT INTO payment_intents (id, account_id, amount_cents, status)
VALUES ($1, $2, $3, 'pending')
`
	if _, err := db.Exec(ctx, q, id, accountID, amountCents); err != nil {
		return nil, err
	}

	return &PaymentIntent{
		ID:        id,
		AccountID: accountID,
		Amount:    amountCents,
		Status:    "pending",
	}, nil
}
