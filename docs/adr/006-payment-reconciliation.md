# ADR 006: Reconcile stale attempts through provider lookup

## Status

Accepted

## Decision

Reconciliation runs as a scheduled one-shot CLI job. It selects a bounded batch of old `PENDING` attempts for the configured provider and asks that provider for the result using the original stable attempt idempotency key. The provider adapter returns only recognized statuses and its reference; no card data is involved.

For each result, the PostgreSQL adapter locks the attempt and payment intent, rechecks that both remain `PENDING`, updates them together, and writes an audit record. This makes concurrent webhook delivery or another worker safe: a row that has already changed is left alone. Lookup or persistence failures are counted and produce a nonzero CLI exit so the scheduler can retry and alert.

## Consequences

- Scheduling cadence and alerts are provided by the deployment environment.
- Each provider adapter must support lookup by the stable idempotency key.
- The mock provider models a previously accepted authorization whose original response was lost; it does not model settlement.
- A future provider adapter can map additional terminal states without changing the job or HTTP API.
