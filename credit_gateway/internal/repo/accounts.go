package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Account struct {
	ID               string
	CreditLimitCents int64
	BalanceCents     int64
	AttemptCount     int64
	SpentCents       int64
}

func GetAccountByID(ctx context.Context, db *pgxpool.Pool, id string) (*Account, error) {
	const q = `
SELECT id, credit_limit_cents, balance_cents, attempt_count, spent_cents
FROM accounts
WHERE id = $1
`
	row := db.QueryRow(ctx, q, id)

	var a Account
	if err := row.Scan(&a.ID, &a.CreditLimitCents, &a.BalanceCents, &a.AttemptCount, &a.SpentCents); err != nil {
		return nil, err
	}

	return &a, nil
}
