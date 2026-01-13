package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TryMarkMerchantPayProgressedTx marks a merchant_pay_intents row as progressed exactly once.
// Returns true only for the first caller (idempotency gate).
func TryMarkMerchantPayProgressedTx(
	ctx context.Context,
	tx pgx.Tx,
	intentID uuid.UUID,
) (bool, error) {

	ct, err := tx.Exec(ctx, `
UPDATE merchant_pay_intents
SET progressed_at = now()
WHERE payment_intent_id = $1
  AND progressed_at IS NULL
`, intentID)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 1, nil
}
