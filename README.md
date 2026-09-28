# Payment Gateway

An educational payment gateway backend demonstrating payment lifecycle, idempotency, webhooks, refunds, and reconciliation.

> Portfolio and educational project. The payment gateway does not process real card data. Never submit real card numbers, CVV, or credentials.

## Architecture

Pragmatic hexagonal architecture separates domain rules, application use cases, infrastructure adapters, and HTTP interfaces. See [architecture](docs/architecture.md) and the [architecture decision record](docs/adr/001-architecture.md).

## Features

- Go HTTP service with health endpoint
- PostgreSQL local development environment
- Domain and persistence structure prepared for the planned capabilities: Merchants and customers; payment intents and attempts; payment methods; idempotency; signed webhooks; refunds; reconciliation; audit logs; rate limiting.

Business flows are added incrementally; the current baseline does not claim these features are implemented.

## Tech stack

Go 1.23, PostgreSQL 16, Docker Compose, GitHub Actions.

## ERD

See [docs/erd.md](docs/erd.md) for the Mermaid diagram.

## Local setup

```sh
cp .env.example .env
docker compose up --build
# in another terminal
curl -i http://localhost:8081/healthz
```

The Compose database credentials are for local development only. Do not reuse them outside local development.

## Environment variables

| Variable | Purpose | Default |
| --- | --- | --- |
| `APP_ENV` | Runtime environment | `development` |
| `HTTP_ADDR` | HTTP listen address | `:8081` |
| `DATABASE_URL` | PostgreSQL connection | local Compose database |
| `LOG_LEVEL` | Log verbosity | `debug` |

## Migrations

Ordered up/down SQL migrations will live in `migrations/`. Schema and migration runner are introduced with the first persistence feature. See [database design](docs/database.md).

## API example

```sh
curl -i http://localhost:8081/healthz
```

Planned routes are documented in [docs/api.md](docs/api.md).

## Testing

```sh
go test ./...
go vet ./...
go build ./...
```

Database integration tests will be added with persistence flows. CI currently runs formatting, vet, tests, and build.

## Design decisions

- Define a PaymentProvider port in the application boundary and begin with a deterministic MockPaymentProvider adapter. This demonstrates provider integration without processing or storing real card numbers, CVV, or sensitive credentials. PostgreSQL is the source of truth. Idempotency keys are unique per merchant and persisted transactionally with payment creation; webhook events are persisted and deduplicated before processing.
- Detailed state transitions: `CREATED → PENDING → AUTHORIZED → PAID; PENDING → FAILED or EXPIRED; CREATED or PENDING → CANCELLED; PAID → REFUNDED`.

## Future improvements

Implement schema migrations and use cases incrementally, publish an OpenAPI contract, add unit and PostgreSQL integration tests, and add operational metrics and tracing.
