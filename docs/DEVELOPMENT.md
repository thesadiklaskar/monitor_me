# Local Development

This guide describes the Phase 2 local development environment. It is intended for local development only, not as a production hardening guide.

## Toolchain Baseline

| Tool | Baseline |
| --- | --- |
| Go | 1.27.4 |
| Node.js | 26.13.0 LTS |
| Next.js | 16.4.6 |
| React | 19.3.1 |
| Tailwind CSS | 4.3.5 |
| PostgreSQL container | 18.7 |
| Redis container | 8.10.3 |
| Docker Compose | Compose v2 |

Runtime versions are pinned in the project files. Review compatibility and security updates before changing them.

## Prerequisites

- Git
- Go 1.27.4
- Node.js 26.13.0 and npm
- Docker Engine or Docker Desktop with Docker Compose v2
- A terminal with Make (optional; equivalent commands are shown below)

## Configure Local Environment

From the repository root:

\`\`\`sh
cp .env.example .env
\`\`\`

Change \`POSTGRES_PASSWORD\` and \`REDIS_PASSWORD\` in \`.env\` before starting the stack. The sample values are for isolated local development only and must never be reused in shared or production environments.

Database URLs must be valid PostgreSQL URLs. If a password contains reserved URL characters, percent-encode it before including it in \`DATABASE_URL\`.

## Start the Control Plane

\`\`\`sh
docker compose up --build -d
docker compose ps --all
docker compose logs -f --tail=100
\`\`\`

Compose starts PostgreSQL and Redis, runs the one-shot migration service, and starts the API only after the migration service exits successfully. The API checks PostgreSQL and Redis during startup.

The local endpoints are bound to loopback by default:

- Web application: http://localhost:3000
- API liveness: http://localhost:8080/health/live
- API readiness: http://localhost:8080/health/ready
- PostgreSQL: localhost:5432
- Redis: localhost:6379

PostgreSQL and Redis ports are exposed only on loopback for local tooling. Do not expose these ports publicly.

\`/health/live\` reports whether the API process can answer requests. \`/health/ready\` checks PostgreSQL and Redis and returns HTTP 503 when either dependency is unavailable. It reports only dependency state and never returns connection strings or driver errors.

## Database Migrations

Migrations are embedded in the migration runner binary and applied by the one-shot \`migrate\` Compose service before the API starts.

To run the migration service manually:

\`\`\`sh
docker compose run --rm migrate
\`\`\`

To inspect the migration version:

\`\`\`sh
docker compose exec -T postgres psql -U monitor_me -d monitor_me -c "SELECT * FROM schema_migrations;"
\`\`\`

The down migration exists for disposable development databases. Do not reverse migrations in an environment containing data that must be retained.

## Run Services Without Docker

PostgreSQL and Redis must be available locally. Set \`DATABASE_URL\`, \`REDIS_ADDR\`, and \`REDIS_PASSWORD\` before starting the API.

Backend:

\`\`\`sh
cd backend
go mod tidy
go test ./...
go run ./cmd/migrate
go run ./cmd/api
\`\`\`

Frontend, in a separate terminal:

\`\`\`sh
cd web
npm install
npm run dev
\`\`\`

## Quality Checks

From the repository root:

\`\`\`sh
make test
make vet
make web-lint
make web-typecheck
make web-build
\`\`\`

The CI workflow runs backend, frontend, and Compose integration checks. A green CI run is required before the corresponding foundation slice is marked verified.

## Current Limitations

- The API currently implements health endpoints only; server inventory and monitoring APIs are not implemented yet.
- PostgreSQL and Redis clients are connected for startup and readiness checks, but application repositories, authentication, and live telemetry are not implemented yet.
- The initial migration creates identity and inventory foundations; metric tables and time-series retention are planned for Phase 3.
- The frontend is a static foundation preview. It does not display real telemetry and does not yet implement authentication or enrollment.
- Protocol Buffer definitions exist, but generated code, a running gRPC server, and agent enrollment are not implemented yet.
- The current Compose stack is a local development environment, not a production deployment topology.
