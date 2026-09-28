# Database migrations

Migrations are ordered SQL files with `.up.sql` and `.down.sql` suffixes. The initial schema is `000001_initial_schema`. Apply the up migration to the local Compose database from the repository root:

```sh
docker compose up -d postgres
docker compose exec -T postgres psql -U app -d payment_gateway < migrations/000001_initial_schema.up.sql
```

Rollback the initial migration with the matching `.down.sql` file. Apply each migration once and in numeric order. A versioned migration runner and automated migration integration tests will be introduced with the database adapters.
