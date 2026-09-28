CREATE TABLE merchants (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    api_key_hash TEXT NOT NULL UNIQUE,
    webhook_secret_encrypted TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE customers (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    external_id TEXT,
    email TEXT,
    name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, external_id),
    CHECK (email IS NOT NULL OR external_id IS NOT NULL)
);
CREATE INDEX customers_merchant_email_idx ON customers(merchant_id, lower(email)) WHERE email IS NOT NULL;

CREATE TABLE payment_intents (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id BIGINT REFERENCES customers(id) ON DELETE RESTRICT,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency CHAR(3) NOT NULL,
    status TEXT NOT NULL DEFAULT 'CREATED' CHECK (status IN ('CREATED','PENDING','AUTHORIZED','PAID','FAILED','EXPIRED','CANCELLED','REFUNDED')),
    description TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX payment_intents_merchant_created_idx ON payment_intents(merchant_id, created_at DESC);
CREATE INDEX payment_intents_status_created_idx ON payment_intents(status, created_at DESC);
CREATE INDEX payment_intents_expiration_idx ON payment_intents(expires_at) WHERE status IN ('CREATED','PENDING');

CREATE TABLE payment_methods (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id BIGINT REFERENCES customers(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    provider_method_ref TEXT NOT NULL,
    method_type TEXT NOT NULL,
    display_label TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, provider, provider_method_ref)
);
CREATE INDEX payment_methods_customer_idx ON payment_methods(customer_id);

CREATE TABLE payment_attempts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    payment_intent_id BIGINT NOT NULL REFERENCES payment_intents(id) ON DELETE RESTRICT,
    attempt_number INTEGER NOT NULL CHECK (attempt_number > 0),
    provider TEXT NOT NULL,
    provider_reference TEXT,
    status TEXT NOT NULL CHECK (status IN ('PENDING','AUTHORIZED','PAID','FAILED')),
    failure_code TEXT,
    failure_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (payment_intent_id, attempt_number),
    UNIQUE (provider, provider_reference)
);
CREATE INDEX payment_attempts_intent_created_idx ON payment_attempts(payment_intent_id, created_at DESC);

CREATE TABLE idempotency_keys (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    idempotency_key TEXT NOT NULL,
    request_hash BYTEA NOT NULL,
    response_status INTEGER,
    response_body JSONB,
    payment_intent_id BIGINT REFERENCES payment_intents(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE (merchant_id, idempotency_key),
    CHECK ((response_status IS NULL) = (response_body IS NULL))
);
CREATE INDEX idempotency_keys_expiration_idx ON idempotency_keys(expires_at);

CREATE TABLE webhook_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL,
    provider_event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    signature_valid BOOLEAN NOT NULL,
    status TEXT NOT NULL DEFAULT 'RECEIVED' CHECK (status IN ('RECEIVED','PROCESSING','PROCESSED','FAILED')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ,
    last_error TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    UNIQUE (merchant_id, provider, provider_event_id)
);
CREATE INDEX webhook_events_retry_idx ON webhook_events(next_attempt_at) WHERE status = 'FAILED';
CREATE INDEX webhook_events_merchant_received_idx ON webhook_events(merchant_id, received_at DESC);

CREATE TABLE refunds (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    payment_intent_id BIGINT NOT NULL REFERENCES payment_intents(id) ON DELETE RESTRICT,
    payment_attempt_id BIGINT REFERENCES payment_attempts(id) ON DELETE RESTRICT,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency CHAR(3) NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING','SUCCEEDED','FAILED')),
    reason TEXT NOT NULL DEFAULT '',
    provider_reference TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (payment_attempt_id, provider_reference)
);
CREATE INDEX refunds_intent_created_idx ON refunds(payment_intent_id, created_at DESC);

CREATE TABLE audit_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    merchant_id BIGINT REFERENCES merchants(id) ON DELETE SET NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_merchant_created_idx ON audit_logs(merchant_id, created_at DESC);
CREATE INDEX audit_logs_entity_created_idx ON audit_logs(entity_type, entity_id, created_at DESC);
