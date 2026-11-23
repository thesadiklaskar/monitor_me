# Local Development

This guide covers the Phase 1–3 backend/agent foundations and the current Phase 4 dashboard implementation. It also points to the separate Linux production deployment runbook; the default Compose stack remains a local development topology.

## Toolchain Baseline

| Tool | Baseline |
| --- | --- |
| Go | 1.27.4 |
| Node.js | 26.13.0 |
| Next.js | 16.4.6 |
| React | 19.3.1 |
| Tailwind CSS | 4.3.5 |
| PostgreSQL container | 18.7 |
| Redis container | 8.10.3 |
| Docker Compose | Compose v2 |

## Prerequisites and setup

- Go 1.27.4
- Node.js 26.13.0 and npm
- Docker Engine or Docker Desktop with Compose v2
- Protocol Buffers compiler (`protoc`) for local generation
- OpenSSL for a local development certificate

From the repository root on Linux, macOS, or Windows with WSL2 + Docker Desktop:

```sh
git pull origin main
cp .env.example .env
# Change the local PostgreSQL and Redis passwords if this machine is shared.
make certs
docker compose up --build -d
docker compose ps --all
```

The first build downloads the container images and compiles the API/web images. Compose starts PostgreSQL and Redis, runs the one-shot migration container, then starts the API and Next.js dashboard. Wait until all long-running services report healthy before opening the browser. The generated self-signed key is deliberately readable by the local non-root API container; it is for localhost development only and must never be reused on a public host.

## Local endpoints

- Web application: http://localhost:3000
- API liveness: http://localhost:8080/health/live
- API readiness: http://localhost:8080/health/ready
- Agent gRPC/TLS: localhost:8443
- PostgreSQL: localhost:5432
- Redis: localhost:6379

The API readiness endpoint requires PostgreSQL and Redis. Liveness is independent of these dependencies. Compose CI stops each dependency separately, verifies readiness becomes unhealthy, restarts the dependency, and verifies readiness recovers. The dashboard includes authenticated server views, historical/live charts, project/mapping administration, alerts/in-app notifications, and allowlisted remote operations. Owner acceptance remains required. The local stack binds published ports to 127.0.0.1; access it through localhost on the same machine.

## First owner and server enrollment

The initial owner is created once per database. Register and log in using the API:

```sh
curl -sS http://localhost:8080/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","display_name":"Workspace Owner","password":"replace-with-a-long-password","organization_name":"Example Team","organization_slug":"example-team"}'

curl -sS http://localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"owner@example.com","password":"replace-with-a-long-password"}'
```

Create a server with the returned bearer token:

```sh
curl -sS http://localhost:8080/v1/servers \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"production-node-01","hostname":"production-node-01"}'
```

The server creation response contains a scoped, single-use enrollment token with a 15-minute lifetime. Treat it as a secret and do not commit it or expose it in logs.

## Run the native Linux agent

The agent runs directly on the monitored Linux host; Docker is not required on that host.

```sh
cd agent
go mod tidy
go test -race ./...
go vet ./...
go build -trimpath ./cmd/monitor-agent
```

To enroll against the local control plane, build the native binary and use the one-time token and development CA. Run this from the repository root in a second terminal; set ENROLLMENT_TOKEN to the one-time value shown when you add the test server:

```sh
cd agent
go build -trimpath -o ./monitor-agent ./cmd/monitor-agent
MONITOR_GRPC_ADDRESS=localhost:8443 \
MONITOR_TLS_CA_FILE=../.data/certs/server.crt \
MONITOR_ENROLLMENT_TOKEN="$ENROLLMENT_TOKEN" \
MONITOR_SPOOL_DIR=./.monitor-me/spool \
MONITOR_AGENT_STATE_FILE=./.monitor-me/agent-state.json \
./monitor-agent
```

Keep this terminal running while testing dashboard telemetry. The Linux collector runs on a Linux host. On Windows, the web/database/API stack can run with Docker Desktop, but real host metrics require a separate Linux VM or WSL2 environment with procfs support and network access to the host's `localhost:8443` gRPC port.

The agent writes its session credential into a mode-0600 state file within a mode-0700 directory. On restarts it reuses the stored session and does not need the original enrollment token until that session expires or is revoked. If the agent has no valid local session, supply a fresh enrollment token.

Check local service readiness and logs:

```sh
curl -fsS http://localhost:8080/health/live
curl -fsS http://localhost:8080/health/ready
# When troubleshooting:
docker compose logs --tail=150 postgres redis migrate api web
```

Stop the stack without deleting data using `docker compose down`. To remove all local test data and start over, use `docker compose down -v` (this permanently deletes the local PostgreSQL and Redis volumes).

## Current Linux collector contract

The native Linux agent reads procfs and Go runtime process-CPU metrics. Its health samples include per-interval collector error count, durable WAL size and pending-batch count, and the most recent accepted send timestamp. The timestamp is omitted after process restart until a new matching backend ACK arrives. A hardened systemd unit is provided in `agent/deploy/systemd/monitor-me-agent.service`; follow `agent/README.md` for setup and environment-file permissions.

The initial native collector reads Linux procfs:

| Area | Initial metrics or inventory |
| --- | --- |
| CPU | Aggregate and logical-core busy percentage; user, system, and iowait |
| Memory | Total, available, used, cache estimate, swap total/used |
| Disk | Read/write bytes/sec and operations/sec by device |
| Network | Receive/transmit bytes/sec, packets/sec, errors/sec by interface |
| Host/agent | Host uptime, agent uptime, process CPU, collector error count, WAL bytes/pending batches, last successful ACK timestamp, process count and agent RSS when visible |
| Processes | Up to 128 readable processes, per-process RSS, cgroup labels |
| TCP | Connection state counts by address family and listening-port inventory |
| Workloads | cgroup path, Systemd unit names, Docker/container cgroup IDs where exposed |

CPU, disk, and network rates are calculated from counter deltas using monotonic elapsed time. The first snapshot intentionally omits rates. Counter reset and invalid intervals produce missing samples rather than negative values. No claim is made that a 100ms sample interval implies 1ms dashboard freshness.

## Historical query and live WebSocket

Authenticated server history:

```text
GET /v1/servers/{serverID}/metrics?metric=host.cpu.utilization&from=2026-11-22T10:00:00Z&to=2026-11-22T10:05:00Z&step=1s&limit=300
```

The query requires organization and server context from the bearer session, enforces a 100ms minimum aggregation step, a maximum 30-day raw range, and at most 5,000 output points. Range/step combinations that exceed the limit are rejected rather than silently truncated.

A browser first obtains a 30-second single-use ticket:

```text
POST /v1/live/ticket
GET  /v1/live/ws?ticket={single-use-ticket}
```

The ticket is hash-only persisted and bound to the authenticated user session and organization. A WebSocket subscription receives only the current organization channel. The server uses a bounded 32-event output queue and disconnects a slow client; reconnect using a fresh ticket and query PostgreSQL history to catch up. Redis Pub/Sub is ephemeral and never replaces the durable PostgreSQL record.

## Data durability and retention

- PostgreSQL is the durable source for telemetry and identity state.
- Redis transports ephemeral organization-scoped live notifications.
- The agent WAL is append-only JSONL, fsyncs each batch, applies a configurable disk quota, and retains batches until the backend ACKs the committed transaction.
- An incomplete trailing WAL record is recovered at startup without deleting previously committed records.
- Default raw metric retention is 30 days; batch idempotency tombstones are kept for 90 days; expired WebSocket tickets are cleaned in bounded batches.
- Query aggregation is implemented with PostgreSQL `date_bin`; persistent rollup tables and background downsampling are not yet implemented.

## Backend checks

```sh
cd backend
go mod tidy
go test -race -cover ./...
go vet ./...
go build -trimpath ./cmd/api
go build -trimpath ./cmd/migrate
```

Generate versioned Protobuf sources for both modules:

```sh
make proto
```

## Benchmark commands

Run the native collector and WAL benchmarks from the repository root:

```sh
cd agent
go test -run '^$' -bench 'Benchmark(ProcCollectorCollect|CounterRate|SpoolAppend)' -benchmem -benchtime=3s ./internal/collector ./internal/spool
```

The Compose CI job also runs a TLS gRPC-to-PostgreSQL ingestion benchmark with 64 samples per batch. Benchmark results are environment-specific and must be reported with the runner/host information; they are not universal capacity guarantees.

## Frontend checks

```sh
cd web
npm install
npm run lint
npm run typecheck
npm run build
```

## Current limitations

- The native collector is Linux-only. Windows and macOS collectors are Phase 5 work.
- Docker container metadata is currently inferred from exposed cgroup paths; a full Docker Engine API integration is not yet implemented.
- Process RSS is collected, but per-process CPU attribution is not yet implemented.
- Application HTTP request rate, latency, and status metrics require application instrumentation, a supported reverse-proxy integration, or tracing; they are not inferred from open ports.
- eBPF and kernel-level event tracing are not yet implemented.
- Historical queries aggregate raw samples; persistent materialized downsampling/rollups remain future work.
- Windows/macOS collectors, full Docker Engine discovery, per-process CPU/lifecycle data, application request instrumentation, persistent rollups, signed installers and fleet-scale testing remain unfinished.
- The local Compose deployment is a development topology. Production Compose and operator scripts are in deploy/production/README.md, but no VPS deployment/restore rehearsal is claimed.

## Production deployment preparation

See [deploy/production/README.md](../deploy/production/README.md) for the version-tagged production Compose file, Nginx reverse-proxy example, firewall helper, backup/restore, health verification and application-image rollback procedure. Do not commit the production environment file.
