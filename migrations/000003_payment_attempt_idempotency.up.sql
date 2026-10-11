ALTER TABLE payment_attempts ADD COLUMN idempotency_key TEXT;
CREATE UNIQUE INDEX payment_attempts_intent_idempotency_unique_idx ON payment_attempts(payment_intent_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
