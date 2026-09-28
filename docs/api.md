# API

Base URL: `http://localhost:8081`.

- `GET /healthz` returns `200 OK` when the HTTP process is running.
- `GET /readyz` returns `200 OK` when PostgreSQL is reachable, otherwise `503 Service Unavailable`.
- `POST /v1/payment-intents` requires a merchant bearer API key and an `Idempotency-Key` header. It accepts the amount, currency, optional merchant customer reference, description, and metadata. Unknown fields are rejected, so card data is never accepted.
- `POST /v1/payment-intents/{intentID}/attempts` starts an idempotent mock provider attempt and returns `AUTHORIZED`; later provider events move it to `PAID`.
- `POST /v1/payment-intents/{intentID}/refunds` requires the merchant bearer API key and an `Idempotency-Key`. Only paid intents with a captured provider attempt can be refunded; partial refunds are supported and concurrent reservations cannot exceed the captured amount.
- `POST /v1/webhooks/{merchantID}/{provider}` receives the `payment.paid` and `payment.failed` events. It requires `Payment-Signature: t=<unix>,v1=<hex>` signed over `<timestamp>.<exact raw body>`. The event body permits only `id`, `type`, and `payment_reference`; successful duplicate deliveries receive `200` with `Idempotent-Replay: true`.

Create a local merchant with `MERCHANT_NAME="Demo" DATABASE_URL=... WEBHOOK_ENCRYPTION_KEY=... go run ./cmd/create-merchant`. Save the printed API key and webhook signing secret securely; the command prints them only once.

Reconciliation is a scheduled CLI job rather than a merchant HTTP endpoint: `DATABASE_URL=... RECONCILIATION_OLDER_THAN=5m RECONCILIATION_BATCH_SIZE=100 go run ./cmd/reconcile`. Its JSON report includes checked, updated, unchanged, and failed counts. A nonzero exit signals that a run had provider or persistence failures and should be retried/alerted.

```sh
curl -X POST http://localhost:8081/v1/payment-intents \
  -H 'Authorization: Bearer YOUR_MERCHANT_API_KEY' \
  -H 'Idempotency-Key: order-123-attempt-1' \
  -H 'Content-Type: application/json' \
  -d '{"amount_minor":12500,"currency":"IDR","description":"Order 123"}'
```

Merchant API keys are generated as random 256-bit values; store the plaintext only with the merchant and persist the SHA-256 hash. Reusing a key with the same request returns the original response; reusing it with a different request returns `409 Conflict`. See the complete [OpenAPI contract](openapi.yaml).

```sh
curl -X POST http://localhost:8081/v1/payment-intents/pi_example/attempts \
  -H 'Authorization: Bearer YOUR_MERCHANT_API_KEY' \
  -H 'Idempotency-Key: attempt-order-123-1'
```

```sh
curl -X POST http://localhost:8081/v1/payment-intents/pi_example/refunds \
  -H 'Authorization: Bearer YOUR_MERCHANT_API_KEY' \
  -H 'Idempotency-Key: refund-order-123-part-1' \
  -H 'Content-Type: application/json' \
  -d '{"amount_minor":2500,"reason":"returned item"}'
```
