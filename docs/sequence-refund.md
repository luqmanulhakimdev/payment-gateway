# Refund sequence

```mermaid
sequenceDiagram
  participant M as Merchant
  participant API as Gateway API
  participant DB as PostgreSQL
  participant P as PaymentProvider
  M->>API: POST refund + Idempotency-Key
  API->>DB: Lock paid intent and reserve amount
  DB-->>API: Existing refund or new PENDING refund + captured reference
  API->>P: Refund with stable refund ID key
  P-->>API: Provider refund reference
  API->>DB: Mark SUCCEEDED, audit, set intent REFUNDED if fully refunded
  DB-->>API: Commit
  API-->>M: Refund response
```

The payment intent row lock serializes refund reservations. Pending reservations count against the remaining captured amount, so a provider timeout followed by a retry cannot allow a second request to reserve the same funds.
