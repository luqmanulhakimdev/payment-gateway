# Database design

PostgreSQL is the system of record. The initial migration creates merchants, customers, payment intents, payment methods, payment attempts, webhook events, refunds, idempotency keys, and audit logs. Migration 000002 adds refund idempotency request hashes and the `(payment_intent_id, idempotency_key)` uniqueness constraint.

Amounts use integer minor units with an explicit currency code. Payment lifecycle and webhook-processing states are constrained. Idempotency keys are unique within a merchant and retain a request hash and response. Webhook delivery is unique by merchant, provider, and event ID; failed events can be selected through a partial retry index. Foreign keys preserve financial history.

The schema stores only provider references and safe display metadata for payment methods. It has no columns for card numbers or CVV. Merchant API keys are represented as hashes. Webhook verification secrets must be stored encrypted at rest because signature verification needs the original secret; decryption keys belong in deployment secret storage. Timestamps are `TIMESTAMPTZ`; JSONB is reserved for metadata, webhook payloads, and audit details.

Apply locally using `docker compose exec -T postgres psql -U app -d payment_gateway < migrations/000001_initial_schema.up.sql`. See [ERD](erd.md).
