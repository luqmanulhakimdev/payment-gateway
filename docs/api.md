# API

Base URL: `http://localhost:8081`. The baseline exposes `GET /healthz`, returning `200 OK` with `ok` when the HTTP process is running. The machine-readable contract is [OpenAPI](openapi.yaml).

Business API routes will be added alongside implemented use cases. Error responses will use a consistent JSON envelope and request validation will happen at the transport boundary.
