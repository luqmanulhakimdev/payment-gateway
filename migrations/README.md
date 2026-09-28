# Database migrations

Migrations are ordered SQL files with `.up.sql` and `.down.sql` suffixes. The service embeds up scripts, applies pending migrations transactionally at startup, and serializes migration runners with a PostgreSQL advisory lock. Applied versions are recorded in `schema_migrations`.

Inspect the local schema with `docker compose exec postgres psql -U app -d payment_gateway`. Down migrations are for controlled operator use and are never run automatically.
