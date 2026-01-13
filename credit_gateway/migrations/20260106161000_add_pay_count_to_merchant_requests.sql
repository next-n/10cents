-- +goose Up
alter table merchant_requests
add column if not exists pay_count bigint not null default 0 check (pay_count >= 0);

-- +goose Down
alter table merchant_requests
drop column if exists pay_count;
