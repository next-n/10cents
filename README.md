# 10CENTS — Stupid-but-Serious Payment Gateway (Go)

**10CENTS** is a deliberately over-engineered payment gateway built to demonstrate **money correctness, transactional safety, idempotency, and reliable webhook delivery**.

This is **not** a real financial product.
It is a backend systems showcase focused on how payment systems must behave under retries, partial failures, and concurrency.

---

## Why This Project Exists

Most payment demos stop at:

* simple inserts
* happy-path updates
* no retry logic

This project explores:

* cents-only money handling
* deterministic interest calculation
* idempotent payment confirmation
* incremental merchant settlement
* outbox-based webhook delivery

It answers the question:

> *What actually happens when a payment is retried, confirmed twice, or partially fails?*

---

## Core Principles

### 1. No Floats. Ever.

All money is represented as **cents (`int64`)**.
No floating-point arithmetic anywhere in the system.

### 2. Two-Step Payments

Payments are split into:

* **Intent creation** (no money moves)
* **Confirmation** (money moves)

Confirm operations are **idempotent**.

### 3. Incremental Merchant Settlement

A merchant can request **$1**, and the system fulfills it via:

* **10 separate 10-cent payments**
* each payment accrues interest independently

### 4. Deterministic Interest Rules

Interest is calculated using **basis points (BPS)** and explicit rounding rules.

### 5. Outbox Pattern for Webhooks

Webhooks are:

* enqueued inside DB transactions
* delivered asynchronously
* retried safely
* never sent directly from payment logic

---

## Interest Model (Intentionally “Stupid”)

* Base rate: **100%**
* Step: **+1% per global attempt**
* Formula:

  ```
  rate_bps = 10000 + (attempt_count × 100)
  interest = floor(amount_cents × rate_bps / 10000)
  ```

Example:

* Attempt #10
* Payment: 10 cents
* Rate: 110%
* Interest: 11 cents
* Total charged: 21 cents

Rounding uses **floor**, intentionally favoring predictability over realism.

---

## High-Level Flow

### Normal Payment

```
POST /v1/payment_intents
POST /v1/payment_intents/{id}/confirm
```

### Merchant Payment (Two-Step)

```
POST /v1/merchant_requests
POST /v1/merchant_requests/{id}/pay
POST /v1/merchant_requests/{intent_id}/confirm
(repeat until fulfilled)
```

When the merchant request completes:

```
merchant_requests.pending → completed
→ webhook event enqueued
→ delivered asynchronously
```

---

## Webhooks

* Delivered via **outbox worker**
* Signed using **HMAC-SHA256**
* Replay-safe (timestamp + event_id)
* Exactly-once semantics at business level

Example payload:

```json
{
  "event": "merchant_request.completed",
  "event_id": "uuid",
  "merchant_id": "merchant_test",
  "merchant_request_reference": "order_001",
  "paid_cents": 100,
  "target_cents": 100
}
```

---

## Project Structure

```
credit_gateway/
├─ cmd/
│  ├─ gateway/            # API server + outbox worker
│  └─ webhook_receiver/  # local webhook demo receiver
├─ internal/
│  ├─ http/               # HTTP handlers
│  ├─ repo/               # DB + transaction logic
│  ├─ domain/             # money & interest rules
│  ├─ outbox/             # webhook outbox + worker
│  └─ config/
├─ migrations/            # goose SQL migrations
└─ tests/ (co-located)    # banking-level tests
```

---

## Quick Start (Run & Test)

### 0) Start Postgres

From repo root:

```bash
docker compose up -d
```

### 1) Run migrations

From `credit_gateway/`:

```bash
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="postgres://credit_gateway:credit_gateway@localhost:5432/credit_gateway?sslmode=disable"
goose -dir migrations up
```

### 2) Run the gateway

From `credit_gateway/`:

```bash
go run ./cmd/gateway
```

### 3) (Optional) Run the webhook receiver

In another terminal, from `credit_gateway/`:

```bash
go run ./cmd/webhook_receiver
```

---

## Manual Test Recipe (Curl + SQL)

### A) Reset DB to a clean state

Run this in `psql` (connected to `credit_gateway` DB):

```sql
TRUNCATE TABLE
  webhook_outbox,
  merchant_pay_intents,
  ledger_entries,
  payment_intents,
  merchant_requests,
  accounts
RESTART IDENTITY
CASCADE;
```

### B) Create an account

```sql
INSERT INTO accounts (
  id,
  status,
  credit_limit_cents,
  balance_cents,
  spent_cents,
  attempt_count
)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  'active',
  5000,
  0,
  0,
  0
);
```

---

## Normal Payment Flow

### 1) Create a payment intent (example: 5 cents)

```bash
curl -s -X POST http://localhost:8083/v1/payment_intents \
  -H "Content-Type: application/json" \
  -d '{"account_id":"00000000-0000-0000-0000-000000000001","amount_cents":5}'
```

Copy the returned `id`.

### 2) Confirm the intent

```bash
curl -s -X POST http://localhost:8083/v1/payment_intents/<INTENT_ID>/confirm
```

### 3) Try an invalid amount (example: 11 cents)

This should refuse and apply the flat $10 fine.

```bash
curl -s -X POST http://localhost:8083/v1/payment_intents \
  -H "Content-Type: application/json" \
  -d '{"account_id":"00000000-0000-0000-0000-000000000001","amount_cents":11}'

curl -s -X POST http://localhost:8083/v1/payment_intents/<INTENT_ID>/confirm
```

---

## Merchant Payment Flow (Two-Step)

### 1) Create merchant request

```bash
curl -s -X POST http://localhost:8083/v1/merchant_requests \
  -H "Content-Type: application/json" \
  -d '{
    "merchant_id": "merchant_test",
    "merchant_request_reference": "order_001",
    "payer_account_id": "00000000-0000-0000-0000-000000000001",
    "target_cents": 20,
    "webhook_url": "http://localhost:8090/webhook"
  }'
```

Copy the returned gateway `id` (example: `1`).

### 2) Create a merchant pay intent (fixed 10 cents)

```bash
curl -s -X POST http://localhost:8083/v1/merchant_requests/1/pay
```

Copy the returned `payment_intent_id`.

### 3) Confirm the merchant pay intent

```bash
curl -s -X POST http://localhost:8083/v1/merchant_requests/payment_intents/<PAYMENT_INTENT_ID>/confirm
```

Repeat steps (2) + (3) until `paid_cents == target_cents`.

When completed, the gateway enqueues an outbox event and the webhook receiver prints the delivered payload.

---

## Run Tests (with separate test DB)

### 1) Create test DB

Create `credit_gateway_test` in Postgres (one-time):

```sql
CREATE DATABASE credit_gateway_test;
```

### 2) Run migrations on test DB

```bash
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="postgres://credit_gateway:credit_gateway@localhost:5432/credit_gateway_test?sslmode=disable"
goose -dir migrations up
```

### 3) Run tests

```bash
export TEST_DB_DSN="postgres://credit_gateway:credit_gateway@localhost:5432/credit_gateway_test?sslmode=disable"
go test ./... -count=1
```

---

## What This Is NOT

* ❌ Not a real payment processor
* ❌ Not PCI compliant
* ❌ Not production ready

This is a **backend engineering showcase**, not a fintech product.

---

## Summary

1CENT is intentionally “stupid” in its rules, but **serious in execution**.

It demonstrates how to build systems that remain correct when:

* requests are retried
* confirmations are duplicated
* failures happen mid-transaction
* side effects must be delayed safely

If you care about correctness more than CRUD speed, this project is for you.
