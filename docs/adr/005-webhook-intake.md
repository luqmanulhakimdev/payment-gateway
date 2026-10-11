# ADR 005: Verify webhooks and persist retryable processing

## Status

Accepted

## Decision

The HTTP API verifies a merchant-specific HMAC-SHA256 signature against the exact raw body and a five-minute timestamp window. Merchant secrets are encrypted at rest with AES-256-GCM using a deployment-provided 32-byte key. The event decoder accepts only the provider event ID, event type, and provider payment reference, so arbitrary provider fields are not stored.

The API persists a verified event under the unique `(merchant_id, provider, provider_event_id)` key before attempting payment processing. The payment attempt, intent, audit record, and `PROCESSED` status are updated in one PostgreSQL transaction. If processing fails, that transaction rolls back and a separate update records `FAILED`, a bounded error, and a capped exponential retry time. A scheduled bounded worker retries due records; provider retries can also trigger immediate retry. A repeated event with the same content is acknowledged without applying a completed state change again. Reusing an event ID for different content returns a conflict.

Supported event types are `payment.paid` and `payment.failed`. Both must match an attempt belonging to the route merchant and provider. The transition is accepted only from a pending or authorized payment state.

## Consequences

- The provider receives success only after the state update and event record commit.
- Provider deliveries and the retry worker can safely retry without partial payment updates.
- Merchant provisioning prints the API and webhook credentials once; operators must store them securely.
- A dead-letter policy and operational metrics are future work for events that fail repeatedly.
