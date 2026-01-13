package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func CreateMerchantPayIntentTx(
	ctx context.Context,
	tx pgx.Tx,
	merchantRequestID int64,
	accountID uuid.UUID,
	amountCents int64,
) (*PaymentIntent, error) {

	pi, err := CreatePaymentIntentTx(ctx, tx, accountID, amountCents)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx,
		`insert into merchant_pay_intents (payment_intent_id, merchant_request_id)
		 values ($1, $2)`,
		pi.ID, merchantRequestID,
	)
	if err != nil {
		return nil, err
	}

	return pi, nil
}

func GetMerchantRequestIDByPaymentIntentForUpdate(
	ctx context.Context,
	tx pgx.Tx,
	intentID uuid.UUID,
) (int64, error) {

	const q = `
select merchant_request_id
from merchant_pay_intents
where payment_intent_id = $1
for update
`
	var mrID int64
	if err := tx.QueryRow(ctx, q, intentID).Scan(&mrID); err != nil {
		return 0, err
	}
	return mrID, nil
}
