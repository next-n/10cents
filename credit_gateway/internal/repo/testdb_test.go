package repo

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Fatalf("missing TEST_DB_DSN")
	}

	// safety: refuse running tests on dev DB
	if strings.Contains(dsn, "/credit_gateway?") || strings.Contains(dsn, "/credit_gateway?sslmode") {
		t.Fatalf("refusing to run tests on dev database DSN: %s", dsn)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("db ping: %v", err)
	}

	return pool
}

func resetDB(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, `
TRUNCATE TABLE
  webhook_outbox,
  merchant_pay_intents,
  ledger_entries,
  payment_intents,
  merchant_requests,
  accounts
RESTART IDENTITY
CASCADE;
`)
	if err != nil {
		t.Fatalf("resetDB truncate: %v", err)
	}
}
