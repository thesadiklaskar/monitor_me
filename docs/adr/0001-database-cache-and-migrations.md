# ADR 0001: PostgreSQL, Redis, and Database Migrations

- Status: Accepted
- Date: 2026-11-22
- Phase: 2 — Platform Foundation and Secure Connectivity

## Context

The control plane requires durable application state, a responsive live-state cache, bounded resource use, and a safe way to apply schema changes before the API starts.

Database migrations must be repeatable, versioned, auditable, and protected from concurrent execution when multiple application instances are deployed.

## Decisions

### PostgreSQL client

Use \`github.com/jackc/pgx/v5\` v5.11.0 through its \`database/sql\` adapter for the API's primary database pool.

Reasons:
- Native PostgreSQL protocol support.
- Familiar \`database/sql\` integration and pool controls.
- The v5.11.0 release explicitly includes Go 1.27 compatibility work.
- PostgreSQL remains the durable source of truth for application state and historical metrics.

### Redis client

Use \`github.com/redis/go-redis/v9\` v9.23.0 for explicitly ephemeral cache and coordination use cases.

Reasons:
- Go-native client with configurable connection-pool, timeout, and retry behavior.
- Redis is not the durable source of truth.
- API readiness checks Redis availability, but durable PostgreSQL records must remain valid if Redis is unavailable.

### Migration runner

Use \`github.com/golang-migrate/migrate/v4\` v4.20.1 with its pgx/v5 database driver and embedded \`io/fs\` migration source.

Reasons:
- Versioned up/down migrations.
- Dirty-version tracking when a migration fails.
- PostgreSQL advisory locking to prevent multiple migrators from applying schema changes concurrently.
- Embedded migrations make the migration runner independent of the container's working directory.

The separate migration executable runs once as a Compose one-shot service. The API starts only after that service exits successfully. API replicas do not apply migrations independently.

## Operational Boundaries

- PostgreSQL connection pools are bounded and use context-aware startup pings.
- Redis uses bounded pool settings and explicit dial/read/write timeouts.
- Readiness reports dependency state without exposing connection strings or driver errors.
- Liveness remains independent from database and cache availability.
- Redis outage must not be treated as loss of durable telemetry.
- Local Compose credentials are for isolated development only; production secrets must be injected securely.
- Migration SQL is transaction-wrapped where the underlying statements support transactional execution.

## References

- pgx v5 releases: https://github.com/jackc/pgx/releases
- go-redis releases: https://github.com/redis/go-redis/releases
- golang-migrate v4 releases: https://github.com/golang-migrate/migrate/releases
- golang-migrate pgx/v5 driver: https://pkg.go.dev/github.com/golang-migrate/migrate/v4/database/pgx/v5
- golang-migrate embedded io/fs source: https://pkg.go.dev/github.com/golang-migrate/migrate/v4/source/iofs

## Consequences

The backend now has explicit database/cache clients and a separate migration command. CI must run a full Compose smoke test to verify migration execution, API readiness, and Redis/PostgreSQL connectivity. Additional integration tests are required before the database/cache portion of Phase 2 is marked complete.
