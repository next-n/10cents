package repo

import (
	"context"
	"errors"
	"log"

	"gateway/internal/domain"
	"gateway/internal/money"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrAccountLocked = errors.New("account locked")

func ConfirmPaymentTx(
	ctx context.Context,
	tx pgx.Tx,
	intentID uuid.UUID,
	policy domain.InterestPolicy,
) error {

	var (
		accountID     uuid.UUID
		amountCents   int64
		intentStatus  string
		attemptCount  int64
		spentCents    int64
		creditLimit   int64
		balanceCents  int64
		accountStatus string
	)

	const q = `
select
  pi.account_id, pi.amount_cents, pi.status,
  a.attempt_count, a.spent_cents, a.credit_limit_cents, a.balance_cents, a.status
from payment_intents pi
join accounts a on a.id = pi.account_id
where pi.id = $1
for update
`
	if err := tx.QueryRow(ctx, q, intentID).Scan(
		&accountID,
		&amountCents,
		&intentStatus,
		&attemptCount,
		&spentCents,
		&creditLimit,
		&balanceCents,
		&accountStatus,
	); err != nil {
		return err
	}

	// idempotency: only process pending
	if intentStatus != "pending" {
		return nil
	}

	// account lock check
	if accountStatus != "active" {
		_ = RefusePaymentIntentTx(ctx, tx, intentID)
		return ErrAccountLocked
	}

	// increment global attempts
	attemptCount++
	if _, err := tx.Exec(ctx,
		`update accounts set attempt_count = $1, updated_at = now() where id = $2`,
		attemptCount, accountID,
	); err != nil {
		return err
	}
	// calculate interest
	spent := money.Cents(amountCents)
	interest := policy.InterestDue(spent, attemptCount)

	// invalid amount -> penalty + refused
	if amountCents < 1 || amountCents > 10 {
		fine := money.Cents(domain.InvalidAmountFineCents)

		if balanceCents+int64(fine) > creditLimit {
			_ = RefusePaymentIntentTx(ctx, tx, intentID)
			_ = LockAccountTx(ctx, tx, accountID, "insufficient_credit")
			return ErrInsufficientCredit
		}

		if err := insertLedger(ctx, tx, accountID, intentID, "penalty", fine); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`update accounts
			   set balance_cents = balance_cents + $1,
			       updated_at = now()
			 where id = $2`,
			int64(fine), accountID,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`update payment_intents set status = 'refused' where id = $1`,
			intentID,
		); err != nil {
			return err
		}

		// keep your custom error
		return ErrMoreThan10Cents
	}

	// valid amount path
	total := amountCents + int64(interest)

	if balanceCents+total > creditLimit {
		_ = RefusePaymentIntentTx(ctx, tx, intentID)
		_ = LockAccountTx(ctx, tx, accountID, "insufficient_credit")
		return ErrInsufficientCredit
	}

	if err := insertLedger(ctx, tx, accountID, intentID, "principal", money.Cents(amountCents)); err != nil {
		return err
	}
	if err := insertLedger(ctx, tx, accountID, intentID, "interest", interest); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`update accounts
		   set balance_cents = balance_cents + $1,
		       spent_cents   = spent_cents + $2,
		       updated_at    = now()
		 where id = $3`,
		total, amountCents, accountID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`update payment_intents set status = 'succeeded' where id = $1`,
		intentID,
	); err != nil {
		return err
	}

	return nil
}

func insertLedger(
	ctx context.Context,
	tx pgx.Tx,
	accountID uuid.UUID,
	intentID uuid.UUID,
	entryType string,
	amount money.Cents,
) error {
	_, err := tx.Exec(
		ctx,
		`insert into ledger_entries (id, account_id, payment_intent_id, entry_type, amount_cents)
		 values ($1, $2, $3, $4, $5)`,
		uuid.New(), accountID, intentID, entryType, int64(amount),
	)
	return err
}
func LockAccountTx(
	ctx context.Context,
	tx pgx.Tx,
	accountID uuid.UUID,
	reason string, // optional, for future audit/logging
) error {
	log.Print(accountID)
	s, err := tx.Exec(ctx,
		`update accounts
		   set status = 'locked',
		       updated_at = now()
		 where id = $1
		   and status != 'locked'`,
		accountID,
	)
	log.Print(s)

	return err
}
