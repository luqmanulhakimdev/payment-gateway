ALTER TABLE refunds ADD COLUMN idempotency_key TEXT;
ALTER TABLE refunds ADD COLUMN request_hash BYTEA;
CREATE UNIQUE INDEX refunds_intent_idempotency_unique_idx ON refunds(payment_intent_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
