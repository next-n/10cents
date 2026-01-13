package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func RefusePaymentIntentTx(
	ctx context.Context,
	tx pgx.Tx,
	intentID uuid.UUID,
) error {
	_, err := tx.Exec(
		ctx,
		`update payment_intents
		   set status = 'refused'
		 where id = $1
		   and status = 'pending'`,
		intentID,
	)
	return err
}
