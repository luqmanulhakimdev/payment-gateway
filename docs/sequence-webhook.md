# Webhook processing sequence

```mermaid
sequenceDiagram
  participant Provider
  participant API as Webhook endpoint
  participant DB as PostgreSQL
  participant Worker as Retry command
  Provider->>API: Event + signature
  API->>DB: Load encrypted merchant secret
  API->>API: Decrypt secret and verify exact signed body
  API->>API: Validate allow-listed event fields
  API->>DB: Commit verified event as RECEIVED
  API->>DB: Lock event; update payment, audit, and event status
  alt Processing succeeds
    DB-->>API: Commit PROCESSED event
    API-->>Provider: 200 acknowledgement
  else Processing fails
    DB-->>API: Roll back state transition
    API->>DB: Persist FAILED status and next_attempt_at
    API-->>Provider: Error; provider may retry
    Worker->>DB: Select due RECEIVED/FAILED events
    Worker->>DB: Retry payment transition transactionally
    DB-->>Worker: Mark PROCESSED or schedule next attempt
  end
  Note over API,DB: Event ID uniqueness and row locks make concurrent provider and worker retries safe.
```
