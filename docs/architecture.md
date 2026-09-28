# Architecture

## Goals

An educational payment gateway backend demonstrating payment lifecycle, idempotency, webhooks, refunds, and reconciliation.

## Layers

- `internal/domain`: entities, invariants, and domain services; no infrastructure dependencies.
- `internal/application`: use cases and ports.
- `internal/infrastructure`: PostgreSQL, provider, and other adapters.
- `internal/interfaces`: HTTP handlers, middleware, and API transport.
- `cmd/api`: process wiring and startup.

Dependencies point inward. PostgreSQL is the primary datastore.

## Scope

Merchants and customers; payment intents and attempts; payment methods; idempotency; signed webhooks; refunds; reconciliation; audit logs; rate limiting.

## Runtime

The initial HTTP process exposes `GET /healthz`. Business endpoints are added in feature branches as their use cases and persistence rules are implemented.
