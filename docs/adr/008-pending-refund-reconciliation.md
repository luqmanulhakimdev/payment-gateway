# ADR 008: Reconcile pending refunds with stable provider keys

## Status

Accepted

## Context

A provider timeout can occur after it has accepted a refund but before the API persists success. The refund reservation remains `PENDING` and continues to count against the refundable balance. Retrying with the same key is safe only when the provider sees the same stable operation key.

## Decision

Add a scheduled bounded reconciliation command for stale pending refunds. It loads the persisted payment reference, amount, currency, and refund ID, then retries the provider operation with `refund-<refund ID>`, the same idempotency key used by the HTTP flow. A confirmed provider result finalizes the reservation and audit record inside the existing refund transaction. Failures remain pending and cause a nonzero command exit.

## Consequences

- Provider timeouts can be recovered without releasing the reserved amount or creating a second refund.
- Operators can schedule the command and alert on nonzero exits.
- A production provider adapter must honor the idempotency key and report a stable result for repeated calls.
