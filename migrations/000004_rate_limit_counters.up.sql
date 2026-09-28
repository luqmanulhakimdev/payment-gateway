CREATE TABLE rate_limit_counters (
    scope_key TEXT PRIMARY KEY,
    window_started_at TIMESTAMPTZ NOT NULL,
    request_count INTEGER NOT NULL CHECK (request_count > 0)
);
