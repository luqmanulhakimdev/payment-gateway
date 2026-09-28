# API

Base URL: `http://localhost:8081`.

- `GET /healthz` returns `200 OK` when the HTTP process is running.
- `GET /readyz` returns `200 OK` when PostgreSQL is reachable, otherwise `503 Service Unavailable`.

The machine-readable contract is [OpenAPI](openapi.yaml). Payment, webhook, and refund routes will be added alongside their application use cases.
