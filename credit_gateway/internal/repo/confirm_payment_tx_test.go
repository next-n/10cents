package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"gateway/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// minimal DB interface so we can use both *pgxpool.Pool and pgx.Tx
type dbExecQuery interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func seedAccount(t *testing.T, db dbExecQuery, id uuid.UUID, creditLimit int64, status string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, `
INSERT INTO accounts (id, status, credit_limit_cents, balance_cents, spent_cents, attempt_count)
VALUES ($1, $2, $3, 0, 0, 0)
`, id, status, creditLimit)
	if err != nil {
		t.Fatalf("seedAccount: %v", err)
	}
}

func getAccountState(t *testing.T, db dbExecQuery, id uuid.UUID) (status string, balance, spent, attempts int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.QueryRow(ctx, `
SELECT status, balance_cents, spent_cents, attempt_count
FROM accounts
WHERE id = $1
`, id).Scan(&status, &balance, &spent, &attempts); err != nil {
		t.Fatalf("getAccountState: %v", err)
	}
	return
}

func countLedgerByIntent(t *testing.T, db dbExecQuery, intentID uuid.UUID) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var n int64
	if err := db.QueryRow(ctx, `
SELECT count(*)
FROM ledger_entries
WHERE payment_intent_id = $1
`, intentID).Scan(&n); err != nil {
		t.Fatalf("countLedgerByIntent: %v", err)
	}
	return n
}

func getIntentStatus(t *testing.T, db dbExecQuery, intentID uuid.UUID) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var s string
	if err := db.QueryRow(ctx, `
SELECT status
FROM payment_intents
WHERE id = $1
`, intentID).Scan(&s); err != nil {
		t.Fatalf("getIntentStatus: %v", err)
	}
	return s
}

func TestConfirmPayment_NormalSuccess_UpdatesLedgerAndAccount(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "active")

	pi, err := CreatePaymentIntent(context.Background(), db, accountID, 5)
	if err != nil {
		t.Fatalf("CreatePaymentIntent: %v", err)
	}

	if err := ConfirmPayment(context.Background(), db, pi.ID, domain.DefaultPolicy()); err != nil {
		t.Fatalf("ConfirmPayment: %v", err)
	}

	if got := getIntentStatus(t, db, pi.ID); got != "succeeded" {
		t.Fatalf("intent status = %q, want %q", got, "succeeded")
	}

	if got := countLedgerByIntent(t, db, pi.ID); got != 2 {
		t.Fatalf("ledger rows = %d, want 2", got)
	}

	st, balance, spent, attempts := getAccountState(t, db, accountID)
	if st != "active" {
		t.Fatalf("account status = %q, want active", st)
	}
	if attempts != 1 {
		t.Fatalf("attempt_count = %d, want 1", attempts)
	}
	if spent != 5 {
		t.Fatalf("spent_cents = %d, want 5", spent)
	}

	// attempt=1 => 101% => interest=floor(5*1.01)=5
	// total balance increment = 5 + 5 = 10
	if balance != 10 {
		t.Fatalf("balance_cents = %d, want 10", balance)
	}
}

func TestConfirmPayment_InvalidAmount_11_RefusedPenaltyAndErr(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "active")

	pi, err := CreatePaymentIntent(context.Background(), db, accountID, 11)
	if err != nil {
		t.Fatalf("CreatePaymentIntent: %v", err)
	}

	err = ConfirmPayment(context.Background(), db, pi.ID, domain.DefaultPolicy())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrMoreThan10Cents) {
		t.Fatalf("err = %v, want ErrMoreThan10Cents", err)
	}

	if got := getIntentStatus(t, db, pi.ID); got != "refused" {
		t.Fatalf("intent status = %q, want refused", got)
	}

	if got := countLedgerByIntent(t, db, pi.ID); got != 1 {
		t.Fatalf("ledger rows = %d, want 1 (penalty only)", got)
	}

	_, balance, spent, attempts := getAccountState(t, db, accountID)
	if attempts != 1 {
		t.Fatalf("attempt_count = %d, want 1", attempts)
	}
	if spent != 0 {
		t.Fatalf("spent_cents = %d, want 0", spent)
	}
	if balance != domain.InvalidAmountFineCents {
		t.Fatalf("balance_cents = %d, want %d", balance, domain.InvalidAmountFineCents)
	}
}

func TestConfirmPayment_InsufficientCredit_LocksAccount_RefusesIntent_Persists(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	// attempt=1 => interest=floor(10*1.01)=10 => total=20
	seedAccount(t, db, accountID, 19, "active")

	pi, err := CreatePaymentIntent(context.Background(), db, accountID, 10)
	if err != nil {
		t.Fatalf("CreatePaymentIntent: %v", err)
	}

	err = ConfirmPayment(context.Background(), db, pi.ID, domain.DefaultPolicy())
	if err == nil {
		t.Fatalf("expected ErrInsufficientCredit, got nil")
	}
	if !errors.Is(err, ErrInsufficientCredit) {
		t.Fatalf("err = %v, want ErrInsufficientCredit", err)
	}

	st, balance, spent, attempts := getAccountState(t, db, accountID)
	if st != "locked" {
		t.Fatalf("account status = %q, want locked", st)
	}
	if balance != 0 || spent != 0 {
		t.Fatalf("account changed unexpectedly balance=%d spent=%d", balance, spent)
	}
	if attempts != 1 {
		t.Fatalf("attempt_count = %d, want 1", attempts)
	}

	if got := getIntentStatus(t, db, pi.ID); got != "refused" {
		t.Fatalf("intent status = %q, want refused", got)
	}

	if got := countLedgerByIntent(t, db, pi.ID); got != 0 {
		t.Fatalf("ledger rows = %d, want 0", got)
	}
}

func TestConfirmPayment_LockedAccount_Rejects(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "locked")

	pi, err := CreatePaymentIntent(context.Background(), db, accountID, 5)
	if err != nil {
		t.Fatalf("CreatePaymentIntent: %v", err)
	}

	err = ConfirmPayment(context.Background(), db, pi.ID, domain.DefaultPolicy())
	if err == nil {
		t.Fatalf("expected ErrAccountLocked, got nil")
	}
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("err=%v, want ErrAccountLocked", err)
	}

	if got := countLedgerByIntent(t, db, pi.ID); got != 0 {
		t.Fatalf("ledger rows = %d, want 0", got)
	}
}

func TestConfirmPayment_Idempotent_DoubleConfirm_NoDoubleLedger(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "active")

	pi, err := CreatePaymentIntent(context.Background(), db, accountID, 5)
	if err != nil {
		t.Fatalf("CreatePaymentIntent: %v", err)
	}

	if err := ConfirmPayment(context.Background(), db, pi.ID, domain.DefaultPolicy()); err != nil {
		t.Fatalf("first ConfirmPayment: %v", err)
	}
	if err := ConfirmPayment(context.Background(), db, pi.ID, domain.DefaultPolicy()); err != nil {
		t.Fatalf("second ConfirmPayment should be no-op, got: %v", err)
	}

	if got := countLedgerByIntent(t, db, pi.ID); got != 2 {
		t.Fatalf("ledger rows = %d, want 2", got)
	}
}

func TestConfirmPayment_Concurrency_SameIntent_OnlyOnce(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "active")

	pi, err := CreatePaymentIntent(context.Background(), db, accountID, 5)
	if err != nil {
		t.Fatalf("CreatePaymentIntent: %v", err)
	}

	errCh := make(chan error, 2)
	go func() { errCh <- ConfirmPayment(context.Background(), db, pi.ID, domain.DefaultPolicy()) }()
	go func() { errCh <- ConfirmPayment(context.Background(), db, pi.ID, domain.DefaultPolicy()) }()

	e1 := <-errCh
	e2 := <-errCh
	if e1 != nil || e2 != nil {
		t.Fatalf("expected both nil (one does work, other no-op), got e1=%v e2=%v", e1, e2)
	}

	if got := countLedgerByIntent(t, db, pi.ID); got != 2 {
		t.Fatalf("ledger rows = %d, want 2", got)
	}
}

// compile-time check that *pgxpool.Pool matches dbExecQuery
var _ dbExecQuery = (*pgxpool.Pool)(nil)
