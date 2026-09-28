# API

Base URL: `http://localhost:8081`.

- `GET /healthz` returns `200 OK` when the HTTP process is running.
- `GET /readyz` returns `200 OK` when PostgreSQL is reachable, otherwise `503 Service Unavailable`.
- `POST /v1/payment-intents` requires a merchant bearer API key and an `Idempotency-Key` header. It accepts the amount, currency, optional merchant customer reference, description, and metadata. Unknown fields are rejected, so card data is never accepted.

```sh
curl -X POST http://localhost:8081/v1/payment-intents \
  -H 'Authorization: Bearer YOUR_MERCHANT_API_KEY' \
  -H 'Idempotency-Key: order-123-attempt-1' \
  -H 'Content-Type: application/json' \
  -d '{"amount_minor":12500,"currency":"IDR","description":"Order 123"}'
```

Merchant API keys are generated as random 256-bit values; store the plaintext only with the merchant and persist the SHA-256 hash. Reusing a key with the same request returns the original response; reusing it with a different request returns `409 Conflict`. See the complete [OpenAPI contract](openapi.yaml).
