# Go backend

This folder contains the Monitor Me control-plane API and SQL migration runner.

## Responsibilities

- Accept authenticated native-agent enrollment and telemetry over TLS gRPC.
- Store machine, credential, alert and dashboard state in PostgreSQL.
- Validate, deduplicate and persist metric batches before acknowledging an agent.
- Publish bounded latest-value metric events through Redis for live dashboard updates.
- Store structured host logs in ClickHouse and expose bounded, time-range log search.
- Expose readiness/liveness endpoints and same-install operator APIs.

The operator product is single-install and has no browser account flow. The database and internal Go implementation still contain legacy organization/user/session/project compatibility fields and routes. These are migration debt, not product features; remove them through forward migrations that preserve machine, metric, log, dashboard and alert data. Never wipe Compose volumes to simplify a migration.

## Storage boundaries

| Component | Role |
| --- | --- |
| PostgreSQL | Durable machine metadata, enrollment credentials, dashboards, alert state, metric history and idempotency |
| Redis | Ephemeral cache/pub-sub for latest-value updates; not the durable source of truth |
| ClickHouse | Structured log events and bounded time-range search |
| Agent local WAL | Durable local queue until backend acknowledgement |

The ingestion path must acknowledge a batch only after durable storage has accepted it. Bound message sizes, queues, query windows and result counts. Treat agent labels and log attributes as untrusted input.

## Build and test

Requires Go 1.27.4 as declared by `go.mod`.

From this folder:

```bash
go test ./...
go vet ./...
go build ./cmd/monitor-me-api
go build ./cmd/monitor-me-migrate
```

Integration tests require the documented PostgreSQL/Redis/ClickHouse services and their environment configuration. Keep migration tests isolated and do not use production volumes.

## Migrations and configuration

- Numbered up/down SQL files live in `migrations/`.
- The migration runner is `cmd/monitor-me-migrate`.
- The API entry point is `cmd/monitor-me-api`.
- Runtime configuration is read from environment variables; production values are generated into the private `.env.production` file by the root installer.
- Keep credentials out of source, logs and error messages.

## Design rules

- No customer login, organization, workspace, project or remote-shell product capability.
- Installation-local ownership replaces tenant access control in the target architecture.
- Agent credentials remain short-lived, scoped to a machine, revocable and never logged.
- Keep SQL parameterized and use forward migrations for schema changes.
- Prefer bounded keyset pagination and database-side time aggregation.
- Redis events are notifications; always retain a durable history query for recovery.
- Do not return fake sample metrics when a collector is unavailable.
