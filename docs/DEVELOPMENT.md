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

For the quick local control-plane setup, start Docker Desktop / Docker Engine, pull the latest `main`, then run one helper from the repository root:

```powershell
git pull origin main
.\scripts\start-local.ps1
```

```sh
git pull origin main
./scripts/start-local.sh
```

The helpers create `.env` from `.env.example` when absent, generate a localhost-only development certificate using a temporary Docker container, run `docker compose up --build -d`, and probe the dashboard. They preserve existing volumes. The certificate/private key are for local development only; never reuse them on a public host.

The commands above require Docker only. Go, Node.js, npm and `protoc` are needed when running the individual backend, agent or frontend development/test commands below.

## Local endpoints

- Web application: http://localhost:3000
- API liveness: http://localhost:8080/health/live
- API readiness: http://localhost:8080/health/ready
- Agent gRPC/TLS: localhost:8443
- PostgreSQL: localhost:5432
- Redis: localhost:6379

The API readiness endpoint requires PostgreSQL and Redis. Liveness is independent of these dependencies. Compose CI stops each dependency separately, verifies readiness becomes unhealthy, restarts the dependency, and verifies readiness recovers. The dashboard includes server views, historical/live charts, project/mapping administration, alerts/in-app notifications, and allowlisted remote operations. The default local Compose profile enables anonymous access for localhost convenience. Keep the local stack bound to 127.0.0.1; never expose anonymous mode to a LAN, reverse proxy or the public internet.

## Single-install dashboard and machine enrollment

The default local Compose profile opens without login on `http://localhost:3000`. It creates an internal local workspace on first boot; there is no dashboard signup or organization-creation step in this profile. The API still retains internal tenancy identifiers for data isolation and compatibility.

From the dashboard, choose **Add Machine** and create a one-time enrollment token. On the target machine, run the one-command installer for the shell available there. The installer detects the target OS/architecture itself; do not pick Windows/Linux in the browser.

Windows x64 (PowerShell as Administrator):

```powershell
& ([scriptblock]::Create((Invoke-RestMethod -Uri 'https://YOUR_MONITOR_HOST/install/agent-windows.ps1'))) -ServerUrl 'https://YOUR_MONITOR_HOST' -GrpcAddress 'YOUR_MONITOR_HOST:8443'
```

Linux amd64/arm64 (systemd, root shell):

```sh
curl -fsSL 'https://YOUR_MONITOR_HOST/install/agent-linux.sh' | sudo bash -s -- --server-url 'https://YOUR_MONITOR_HOST' --grpc-address 'YOUR_MONITOR_HOST:8443'
```

Both installers fetch the matching native artifact and public control-plane CA over HTTPS, verify the artifact SHA-256, prompt for the one-time enrollment token without putting it in shell history, and remove the token from the service configuration after the session is persisted. Linux creates and enables a systemd service; Windows installs an automatic-start Windows Service with restart recovery. Windows agent service diagnostics are written to a bounded rotating log under ProgramData; Linux logs go to journald.

The monitored machine does not need Docker, Go, Node.js or a source checkout. Each target needs outbound reachability to the control plane's HTTPS installer and TLS gRPC endpoint. `localhost:8443` is valid only when the agent shares the control-plane host. The current release supports Linux systemd amd64/arm64 and Windows x64; macOS, non-systemd Linux, code-signing/publisher verification, upgrade rollback and unattended token rotation remain incomplete.

The existing low-level authenticated API is still available for development/integration testing, but it is not the default local operator workflow. The local anonymous profile must remain loopback-bound. Remote no-login deployments are fail-closed behind a separate firewall gate; see the production deployment guide and ADR 0004.

The agent stores a private session credential and bounded local spool. On restart it reuses the persisted session without the original enrollment token; use the Add Machine rotation/re-enrollment workflow only after a session expires or is revoked.

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

- Native host collection currently supports Linux and an initial Windows slice. Windows reports CPU, memory/swap, disk, network, uptime, process inventory and OS/virtualization metadata; Linux also reports TCP, cgroup and Systemd workload context. macOS collection remains Phase 5 work.
- Docker container metadata is currently inferred from exposed cgroup paths; a full Docker Engine API integration is not yet implemented.
- Process RSS is collected, but per-process CPU attribution is not yet implemented.
- Application HTTP request rate, latency, and status metrics require application instrumentation, a supported reverse-proxy integration, or tracing; they are not inferred from open ports.
- eBPF and kernel-level event tracing are not yet implemented.
- Historical queries aggregate raw samples; persistent materialized downsampling/rollups remain future work.
- Windows TCP/listening-port metrics, per-process CPU, workload attribution, automatic Windows service lifecycle and signed installers remain unfinished. macOS collection, complete Docker Engine discovery, application request instrumentation, persistent rollups and fleet-scale testing also remain unfinished.
- The local Compose deployment is a development topology. Production Compose and operator scripts are in deploy/production/README.md, but no VPS deployment/restore rehearsal is claimed.

## Production deployment preparation

See [deploy/production/README.md](../deploy/production/README.md) for the version-tagged production Compose file, Nginx reverse-proxy example, firewall helper, backup/restore, health verification and application-image rollback procedure. Do not commit the production environment file.
