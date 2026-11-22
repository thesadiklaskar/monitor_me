# Local Development

This guide describes the initial Phase 2 development environment. It is intended for local development only, not as a production hardening guide.

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

## Start the Control Plane

\`\`\`sh
docker compose up --build -d
docker compose ps
docker compose logs -f --tail=100
\`\`\`

The local endpoints are bound to loopback by default:

- Web application: http://localhost:3000
- API liveness: http://localhost:8080/health/live
- API readiness: http://localhost:8080/health/ready
- PostgreSQL: localhost:5432
- Redis: localhost:6379

PostgreSQL and Redis ports are exposed only on loopback for local tooling. Do not expose these ports publicly.

The API health endpoint currently reports process-level readiness only. Dependency-aware readiness will be added when the database and cache clients are wired into the service.

## Apply the Initial Schema

The initial migration is checked into \`backend/migrations\`. Until a migration runner is selected and integrated, apply it explicitly:

\`\`\`sh
set -a
. ./.env
set +a
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" < backend/migrations/000001_initial_schema.up.sql
\`\`\`

To reverse this initial schema in a disposable local database:

\`\`\`sh
set -a
. ./.env
set +a
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" < backend/migrations/000001_initial_schema.down.sql
\`\`\`

Do not run the down migration against data that must be retained.

## Run Services Without Docker

Backend:

\`\`\`sh
cd backend
go test ./...
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

The CI workflow runs these checks and builds the local Compose services. A green CI run is required before the corresponding foundation slice is marked verified.

## Current Limitations

- The API currently implements only liveness and process-level readiness endpoints.
- PostgreSQL and Redis containers start with health checks, but application clients are not yet connected to them.
- The initial schema is a foundation migration; application repositories and migration automation are not implemented yet.
- The frontend is a static foundation preview. It does not display real telemetry and does not yet implement authentication or enrollment.
- Protocol Buffer definitions exist, but generated code and a running gRPC server are not implemented yet.
- The current Compose stack is a local development environment, not a production deployment topology.
