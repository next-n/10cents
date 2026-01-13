package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDuplicateMerchantRequest = errors.New("duplicate merchant_request_reference for merchant")

type MerchantRequest struct {
	ID                       int64
	MerchantID               string
	MerchantRequestReference *string
	TargetCents              int64
	PaidCents                int64
	Status                   string
	WebhookURL               *string
	CompletedAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
	PayerAccountID           string
}

func CreateMerchantRequest(ctx context.Context, db *pgxpool.Pool,
	merchantID string,
	merchantRequestRefrence *string,
	payerAccountID string,
	targetCents int64,
	webhookURL *string,
) (*MerchantRequest, error) {

	const q = `
insert into merchant_requests
  (merchant_id, merchant_request_reference, payer_account_id, target_cents, webhook_url)
values
  ($1, $2, $3, $4, $5)
returning
  id, merchant_id, merchant_request_reference, payer_account_id, target_cents, paid_cents, status, webhook_url,
  completed_at, created_at, updated_at;
`
	row := db.QueryRow(ctx, q, merchantID, merchantRequestRefrence, payerAccountID, targetCents, webhookURL)

	var mr MerchantRequest
	if err := row.Scan(
		&mr.ID,
		&mr.MerchantID,
		&mr.MerchantRequestReference,
		&mr.PayerAccountID,
		&mr.TargetCents,
		&mr.PaidCents,
		&mr.Status,
		&mr.WebhookURL,
		&mr.CompletedAt,
		&mr.CreatedAt,
		&mr.UpdatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateMerchantRequest
		}
		return nil, err
	}

	return &mr, nil
}

func GetMerchantRequestByID(ctx context.Context, db *pgxpool.Pool, id int64) (*MerchantRequest, error) {
	const q = `
select
  id, merchant_id, merchant_request_reference, payer_account_id, target_cents, paid_cents, status, webhook_url,
  completed_at, created_at, updated_at
from merchant_requests
where id = $1
limit 1;
`
	row := db.QueryRow(ctx, q, id)

	var mr MerchantRequest
	if err := row.Scan(
		&mr.ID,
		&mr.MerchantID,
		&mr.MerchantRequestReference,
		&mr.PayerAccountID,
		&mr.TargetCents,
		&mr.PaidCents,
		&mr.Status,
		&mr.WebhookURL,
		&mr.CompletedAt,
		&mr.CreatedAt,
		&mr.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}

	return &mr, nil
}
