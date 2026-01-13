package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"gateway/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func createMerchantRequestRow(t *testing.T, db dbExecQuery, merchantID string, merchantRef *string, payerAccountID string, target int64) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id int64
	if err := db.QueryRow(ctx, `
insert into merchant_requests (merchant_id, merchant_request_reference, payer_account_id, target_cents, webhook_url)
values ($1, $2, $3, $4, $5)
returning id;
`, merchantID, merchantRef, payerAccountID, target, "http://example.test/webhook").Scan(&id); err != nil {
		t.Fatalf("createMerchantRequestRow: %v", err)
	}
	return id
}

func getMerchantRequestState(t *testing.T, db dbExecQuery, mrID int64) (paid, target int64, status string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.QueryRow(ctx, `
select paid_cents, target_cents, status
from merchant_requests
where id = $1
`, mrID).Scan(&paid, &target, &status); err != nil {
		t.Fatalf("getMerchantRequestState: %v", err)
	}
	return
}

func TestMerchantPay_TwoStep_Fulfills1DollarIn10Confirms(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "active")

	mrID := createMerchantRequestRow(t, db, "merchant_test", ptr("order_001"), accountID.String(), 100)

	for i := 0; i < 10; i++ {
		ctx := context.Background()
		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		pi, err := CreateMerchantPayIntentTx(ctx, tx, mrID, accountID, 10)
		if err != nil {
			t.Fatalf("CreateMerchantPayIntentTx: %v", err)
		}

		if err := ConfirmPaymentTx(ctx, tx, pi.ID, domain.DefaultPolicy()); err != nil {
			t.Fatalf("ConfirmPaymentTx: %v", err)
		}

		_, _, _, err = IncrementMerchantRequestProgress(ctx, tx, mrID, 10)
		if err != nil {
			t.Fatalf("IncrementMerchantRequestProgress: %v", err)
		}

		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	paid, target, st := getMerchantRequestState(t, db, mrID)
	if target != 100 {
		t.Fatalf("target=%d want 100", target)
	}
	if paid != 100 {
		t.Fatalf("paid=%d want 100", paid)
	}
	if st != "completed" {
		t.Fatalf("status=%q want completed", st)
	}
}

func TestMerchantPay_InsufficientCreditMidway_LocksAndStopsProgress(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	// attempt=1 => total=20; attempt=2 => interest=floor(10*1.02)=10; total=20 again
	// so with limit 30: first confirm succeeds (balance=20), second should fail (20+20>30).
	seedAccount(t, db, accountID, 30, "active")

	mrID := createMerchantRequestRow(t, db, "merchant_test", ptr("order_002"), accountID.String(), 100)

	makeAndConfirm := func() error {
		ctx := context.Background()
		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		pi, err := CreateMerchantPayIntentTx(ctx, tx, mrID, accountID, 10)
		if err != nil {
			return err
		}

		err = ConfirmPaymentTx(ctx, tx, pi.ID, domain.DefaultPolicy())
		if err != nil {
			// business errors should commit to persist refused/locked
			if errors.Is(err, ErrInsufficientCredit) || errors.Is(err, ErrMoreThan10Cents) || errors.Is(err, ErrAccountLocked) {
				_ = tx.Commit(ctx)
			}
			return err
		}

		_, _, _, err = IncrementMerchantRequestProgress(ctx, tx, mrID, 10)
		if err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	// First should succeed
	if err := makeAndConfirm(); err != nil {
		t.Fatalf("first merchant confirm: %v", err)
	}

	paid1, _, _ := getMerchantRequestState(t, db, mrID)
	if paid1 != 10 {
		t.Fatalf("paid after first = %d, want 10", paid1)
	}

	// Second should fail with insufficient and lock
	err := makeAndConfirm()
	if err == nil {
		t.Fatalf("expected ErrInsufficientCredit on second, got nil")
	}
	if !errors.Is(err, ErrInsufficientCredit) {
		t.Fatalf("err=%v, want ErrInsufficientCredit", err)
	}

	paid2, _, _ := getMerchantRequestState(t, db, mrID)
	if paid2 != 10 {
		t.Fatalf("paid after fail = %d, want still 10", paid2)
	}

	st, _, _, _ := getAccountState(t, db, accountID)
	if st != "locked" {
		t.Fatalf("account status=%q want locked", st)
	}
}

func TestMerchantPay_MappingRequired_ConfirmWithoutMappingNotFound(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "active")

	pi, err := CreatePaymentIntent(context.Background(), db, accountID, 10)
	if err != nil {
		t.Fatalf("CreatePaymentIntent: %v", err)
	}

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	_, err = GetMerchantRequestIDByPaymentIntentForUpdate(ctx, tx, pi.ID)
	if err == nil {
		t.Fatalf("expected mapping lookup error, got nil")
	}
}

func ptr(s string) *string { return &s }

func TestMerchantRequestCompletion_EnqueuesWebhookOutbox(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "active")

	// IMPORTANT: webhook_url must be non-null for completion enqueue
	mrID := createMerchantRequestRowWithWebhook(t, db, "merchant_test", ptr("order_outbox"), accountID.String(), 100, "http://example.test/webhook")

	for i := 0; i < 10; i++ {
		ctx := context.Background()
		tx, err := db.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback(ctx)

		pi, err := CreateMerchantPayIntentTx(ctx, tx, mrID, accountID, 10)
		if err != nil {
			t.Fatalf("CreateMerchantPayIntentTx: %v", err)
		}

		if err := ConfirmPaymentTx(ctx, tx, pi.ID, domain.DefaultPolicy()); err != nil {
			t.Fatalf("ConfirmPaymentTx: %v", err)
		}

		_, _, _, err = IncrementMerchantRequestProgress(ctx, tx, mrID, 10)
		if err != nil {
			t.Fatalf("IncrementMerchantRequestProgress: %v", err)
		}

		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	var n int64
	if err := db.QueryRow(context.Background(), `
SELECT count(*) FROM webhook_outbox WHERE aggregate_type='merchant_request' AND aggregate_id=$1
`, mrID).Scan(&n); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if n != 1 {
		t.Fatalf("outbox rows=%d want 1", n)
	}
}

func createMerchantRequestRowWithWebhook(t *testing.T, db dbExecQuery, merchantID string, merchantRef *string, payerAccountID string, target int64, webhookURL string) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id int64
	if err := db.QueryRow(ctx, `
insert into merchant_requests (merchant_id, merchant_request_reference, payer_account_id, target_cents, webhook_url)
values ($1, $2, $3, $4, $5)
returning id;
`, merchantID, merchantRef, payerAccountID, target, webhookURL).Scan(&id); err != nil {
		t.Fatalf("createMerchantRequestRowWithWebhook: %v", err)
	}
	return id
}

func TestMerchantConfirm_Idempotent_NoDoubleProgress(t *testing.T) {
	db := testPool(t)
	resetDB(t, db)

	accountID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	seedAccount(t, db, accountID, 5000, "active")

	mrID := createMerchantRequestRowWithWebhook(t, db, "merchant_test", ptr("order_idem"), accountID.String(), 20, "http://example.test/webhook")

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	pi, err := CreateMerchantPayIntentTx(ctx, tx, mrID, accountID, 10)
	if err != nil {
		t.Fatal(err)
	}

	// confirm once
	if err := ConfirmPaymentTx(ctx, tx, pi.ID, domain.DefaultPolicy()); err != nil {
		t.Fatal(err)
	}
	first, err := TryMarkMerchantPayProgressedTx(ctx, tx, pi.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !first {
		t.Fatalf("expected first progress claim true")
	}

	paid, _, _, err := IncrementMerchantRequestProgress(ctx, tx, mrID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if paid != 10 {
		t.Fatalf("paid=%d want 10", paid)
	}

	// "confirm again" same tx simulation: claim should fail now
	second, err := TryMarkMerchantPayProgressedTx(ctx, tx, pi.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second {
		t.Fatalf("expected second progress claim false")
	}

	// do NOT call IncrementMerchantRequestProgress again

	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// verify final paid is still 10
	mr, err := GetMerchantRequestByID(ctx, db, mrID) // adjust if your Get expects pool only
	if err != nil {
		t.Fatal(err)
	}
	if mr.PaidCents != 10 {
		t.Fatalf("paid_cents=%d want 10", mr.PaidCents)
	}
}
