# ADR 005: Verify and apply webhooks synchronously

## Status

Accepted

## Decision

The HTTP API verifies a merchant-specific HMAC-SHA256 signature against the exact raw body and a five-minute timestamp window. Merchant secrets are encrypted at rest with AES-256-GCM using a deployment-provided 32-byte key. The event decoder accepts only the provider event ID, event type, and provider payment reference, so arbitrary provider fields are not stored.

The API inserts or locks the unique `(merchant_id, provider, provider_event_id)` event row, updates the payment attempt and intent, writes an audit record, and marks the event processed in one PostgreSQL transaction. A repeated event with the same content is acknowledged without applying the state change again. Reusing an event ID for different content returns a conflict. Transaction failures roll back, allowing the provider to retry.

Supported event types are `payment.paid` and `payment.failed`. Both must match an attempt belonging to the route merchant and provider. The transition is accepted only from a pending or authorized payment state.

## Consequences

- The provider receives success only after the state update and event record commit.
- Provider deliveries can retry after database failures without partial payment updates.
- Merchant provisioning prints the API and webhook credentials once; operators must store them securely.
- A future asynchronous worker may be needed for providers with strict response deadlines or for richer retry scheduling.
