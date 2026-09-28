# Database design

PostgreSQL is the system of record. Planned core tables: `merchants, customers, payment_intents, payment_attempts, payment_methods, webhook_events, refunds, idempotency_keys, audit_logs`.

Schema work will add foreign keys, check constraints, uniqueness rules, and indexes alongside the migrations that introduce each feature. Money values use integer minor units plus an explicit currency. Timestamped records use UTC.

Critical invariants: stock changes are append-only movements; checkout is transactional and locks inventory; order item prices are snapshots; idempotency uniqueness is merchant-scoped; webhook event identifiers are deduplicated.
