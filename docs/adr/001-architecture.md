# ADR 001: Architecture and persistence boundaries

- Status: Accepted
- Date: 2026-09-28

## Context

The project should demonstrate maintainable Go backend design and preserve business invariants under concurrent requests.

## Decision

Define a PaymentProvider port in the application boundary and begin with a deterministic MockPaymentProvider adapter. This demonstrates provider integration without processing or storing real card numbers, CVV, or sensitive credentials. PostgreSQL is the source of truth. Idempotency keys are unique per merchant and persisted transactionally with payment creation; webhook events are persisted and deduplicated before processing.

## Consequences

Use cases can be tested without PostgreSQL or external providers. Adapter and transaction integration tests will verify database behavior as these flows are implemented.
