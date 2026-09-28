DROP INDEX IF EXISTS refunds_intent_idempotency_unique_idx;
ALTER TABLE refunds DROP COLUMN IF EXISTS request_hash;
ALTER TABLE refunds DROP COLUMN IF EXISTS idempotency_key;
