package repo

import (
	"context"
	"errors"

	"gateway/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInsufficientCredit = errors.New("insufficient credit")

var ErrMoreThan10Cents = errors.New("Refused payment over 10 cents and 10 dollars fined")

func ConfirmPayment(
	ctx context.Context,
	db *pgxpool.Pool,
	intentID uuid.UUID,
	policy domain.InterestPolicy,
) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = ConfirmPaymentTx(ctx, tx, intentID, policy)
	if err != nil {
		if errors.Is(err, ErrInsufficientCredit) || errors.Is(err, ErrMoreThan10Cents) || errors.Is(err, ErrAccountLocked) {
			// keep the lock / penalty / refused status
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return commitErr
			}
			return err
		}
		return err
	}
	return tx.Commit(ctx)
}
