# ADR 004: Give each payment attempt a stable idempotency key

## Status

Accepted

## Context

Provider authorization calls can time out after the provider has accepted them. A merchant retry must not create an additional provider-side payment.

## Decision

Persist the merchant's attempt key under the payment intent before calling the provider. Derive a provider idempotency key from the persisted attempt ID and pass it to the provider port. A timeout keeps the attempt `PENDING`; retrying the same merchant key reuses that attempt. Persist the provider reference and `AUTHORIZED` state after a successful response.

## Consequences

Provider calls run outside database transactions and can be safely repeated if the provider honors its idempotency contract. The attempt table's partial unique index prevents duplicate attempts for the same intent and key. Authorization is distinct from capture; only a verified event should move the intent to `PAID`.
