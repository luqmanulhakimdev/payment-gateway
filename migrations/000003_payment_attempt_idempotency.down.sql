DROP INDEX IF EXISTS payment_attempts_intent_idempotency_unique_idx;
ALTER TABLE payment_attempts DROP COLUMN IF EXISTS idempotency_key;
