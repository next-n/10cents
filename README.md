# 1CENT — Stupid-but-Serious Payment Gateway (Go)

**1CENT** is a deliberately over-engineered payment gateway built to demonstrate **money correctness, transactional safety, idempotency, and reliable webhook delivery**.

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
└─ migrations/            # goose SQL migrations
```

---

## Testing Strategy

* Separate **test database**
* Real PostgreSQL (no mocks for money logic)
* Full transactional tests:

  * interest correctness
  * insufficient credit handling
  * account locking
  * idempotent confirms
  * merchant fulfillment
  * webhook enqueue guarantees

Run tests:

```bash
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
