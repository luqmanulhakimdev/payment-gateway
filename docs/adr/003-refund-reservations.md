# ADR 003: Reserve refund amounts before provider calls

## Status

Accepted

## Context

Provider calls happen outside database transactions. Concurrent partial refunds must not exceed the captured payment, and a timeout must remain safely retryable.

## Decision

Lock the payment intent, verify the merchant and paid state, and persist a `PENDING` refund before calling the provider. Sum `PENDING` and `SUCCEEDED` refunds when validating a new amount. Persist an idempotency key and request hash on the refund; derive a stable provider key from its generated public ID. Retry a pending refund with the same key and request. Mark the intent `REFUNDED` only when succeeded refunds equal the captured amount.

## Consequences

Provider network timeouts leave a durable pending reservation that the merchant can retry. Operators will need a future reconciliation workflow for refunds that remain pending for too long. Provider calls never run while holding database locks.
