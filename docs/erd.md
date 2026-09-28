# Entity relationship diagram

The diagram shows the planned core relationships. Exact columns and constraints will be defined by migrations as each domain is implemented.

```mermaid
erDiagram
  MERCHANTS ||--o{ CUSTOMERS : serves
  MERCHANTS ||--o{ PAYMENT_INTENTS : owns
  CUSTOMERS ||--o{ PAYMENT_INTENTS : requests
  PAYMENT_INTENTS ||--o{ PAYMENT_ATTEMPTS : attempts
  PAYMENT_INTENTS ||--o{ REFUNDS : refunds
  MERCHANTS ||--o{ PAYMENT_METHODS : configures
  MERCHANTS ||--o{ WEBHOOK_EVENTS : receives
  MERCHANTS ||--o{ IDEMPOTENCY_KEYS : scopes
  MERCHANTS ||--o{ AUDIT_LOGS : records
```
