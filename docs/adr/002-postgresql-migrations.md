# ADR 002: PostgreSQL migration runner

- Status: Accepted
- Date: 2026-09-28

## Context

Local startup and CI need a consistent schema, and multiple application instances may start concurrently.

## Decision

Embed ordered SQL up migrations, apply each unapplied migration in a transaction, and use a transaction-scoped PostgreSQL advisory lock to serialize runners. Record applied versions in `schema_migrations`; never run down migrations on startup.

## Consequences

The runtime database role needs schema DDL privileges during startup. A dedicated migration role can replace this if deployment policy later separates schema and runtime permissions.
