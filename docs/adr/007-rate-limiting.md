# ADR 007: Store merchant rate limits in PostgreSQL

## Status

Accepted

## Decision

The API applies fixed-window counters per merchant and operation scope using an atomic PostgreSQL upsert. The count is capped at the first rejected request, so counter rows remain bounded to one per active merchant scope. The window uses the database clock so multiple API instances share the same limit.

Bearer-authenticated endpoints are limited after merchant authentication. Webhook events consume quota only after the merchant secret is loaded and the signature is verified; untrusted path IDs cannot create rate-limit rows. Requests above quota receive `429` and `Retry-After`. Rate-limit storage failures fail closed with `503`.

## Consequences

- PostgreSQL remains the only required runtime service.
- Limits are fixed-window and can allow a burst around a window boundary; a token bucket can replace this if traffic requires smoother shaping.
- The default is 60 requests per minute per merchant and endpoint scope, configurable through environment variables.
