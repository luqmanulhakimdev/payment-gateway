# Webhook processing sequence

```mermaid
sequenceDiagram
  participant Provider
  participant API as Webhook endpoint
  participant DB as PostgreSQL
  Provider->>API: Event + signature
  API->>DB: Load encrypted merchant secret
  API->>API: Decrypt secret and verify exact signed body
  API->>API: Validate allow-listed event fields
  API->>DB: Begin transaction and insert event with unique provider event ID
  alt Duplicate event
    DB-->>API: Lock existing event
    alt Already processed with same content
      API-->>Provider: 200 duplicate acknowledgement
    else Retryable delivery
      API->>DB: Retry payment transition
    end
  else New event
    API->>DB: Update attempt, intent, audit, and event status
    API->>DB: Commit transaction
  end
  API-->>Provider: 200 acknowledgement
  Note over API,DB: Invalid state or DB failure rolls back; provider can retry.
```
