-- +goose Up
create table if not exists merchant_requests (
  id bigserial primary key,

  merchant_id text not null,
  merchant_request_id text null,

  payer_account_id uuid not null references accounts(id),

  target_cents bigint not null check (target_cents > 0),
  paid_cents bigint not null default 0 check (paid_cents >= 0),

  status text not null default 'pending'
    check (status in ('pending','completed','canceled')),

  webhook_url text null,

  completed_at timestamptz null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists ux_merchant_requests_merchant_req
  on merchant_requests (merchant_id, merchant_request_id)
  where merchant_request_id is not null;

create index if not exists ix_merchant_requests_merchant_id
  on merchant_requests (merchant_id);

create index if not exists ix_merchant_requests_payer_account_id
  on merchant_requests (payer_account_id);

create index if not exists ix_merchant_requests_status
  on merchant_requests (status);

-- +goose StatementBegin
create or replace function merchant_requests_set_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;
-- +goose StatementEnd

drop trigger if exists trg_merchant_requests_updated_at on merchant_requests;

create trigger trg_merchant_requests_updated_at
before update on merchant_requests
for each row
execute function merchant_requests_set_updated_at();

-- +goose Down
drop trigger if exists trg_merchant_requests_updated_at on merchant_requests;

-- +goose StatementBegin
drop function if exists merchant_requests_set_updated_at();
-- +goose StatementEnd

drop index if exists ix_merchant_requests_status;
drop index if exists ix_merchant_requests_payer_account_id;
drop index if exists ix_merchant_requests_merchant_id;
drop index if exists ux_merchant_requests_merchant_req;

drop table if exists merchant_requests;
