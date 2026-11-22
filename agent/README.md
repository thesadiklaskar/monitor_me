# Native Monitoring Agent

The native Go agent runs directly on a monitored Linux host. Docker is not required on the monitored machine.

## Current implementation

- Foreground process with JSON structured logs, environment validation, and signal-aware shutdown.
- Configurable 100ms–1m collection interval (500ms default).
- Linux procfs aggregate/per-core CPU, memory, swap, disk throughput/IOPS, network throughput/packets/errors, uptime, process count, and up to 128 readable processes by RSS.
- TCP connection state counts and listening-port inventory from `/proc/net/tcp` and `/proc/net/tcp6`.
- Process cgroup selectors, Systemd unit identification, and Docker/container cgroup IDs where those paths are exposed by the host.
- Counter-derived rates use actual monotonic elapsed time; counter resets and invalid intervals produce missing rates, not negative values.
- Bounded append-only JSONL spool, fsync on append, private 0700 directory/0600 WAL file, quota enforcement, recovery of a truncated final record, and atomic acknowledgement compaction.
- Optional TLS gRPC enrollment, persisted session credentials, heartbeat, reconnect/backoff, and local batch delivery. A batch is removed from the WAL only after the backend reports a durable commit.

Unsupported operating systems report only agent health/spool capabilities. Linux procfs collectors are not claimed on Windows or macOS.

## Build and test

From the repository root:

```sh
cd agent
go mod tidy
go test -race ./...
go vet ./...
go build -trimpath ./cmd/monitor-agent
```

Run local benchmarks:

```sh
go test -run '^$' -bench 'Benchmark(ProcCollectorCollect|CounterRate|SpoolAppend)' -benchmem -benchtime=3s ./internal/collector ./internal/spool
```

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `MONITOR_AGENT_VERSION` | `0.1.0-dev` | Agent build/version string |
| `MONITOR_COLLECTION_INTERVAL` | `500ms` | Collection interval; minimum 100ms |
| `MONITOR_PROC_ROOT` | `/proc` | Procfs mount or fixture root |
| `MONITOR_SPOOL_DIR` | `./.monitor-me/spool` | Local durable queue directory |
| `MONITOR_SPOOL_MAX_BYTES` | `67108864` | Maximum local spool size (64MiB) |
| `MONITOR_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `MONITOR_GRPC_ADDRESS` | unset | TLS gRPC endpoint, e.g. `localhost:8443` |
| `MONITOR_TLS_CA_FILE` | unset | Explicit CA certificate used to verify the backend |
| `MONITOR_ENROLLMENT_TOKEN` | unset | One-use token for first enrollment or session renewal |
| `MONITOR_AGENT_STATE_FILE` | spool directory + `agent-state.json` | Private persisted agent session |

Transport is disabled unless both gRPC address and CA path are configured. Enrollment tokens cannot be supplied in offline mode. Do not put secrets in the repository, process manager manifests, or diagnostic logs.

## Run against a local control plane

Generate the development certificate and start the control plane from the repository root. Create a server with the owner API and use the returned one-time enrollment token:

```sh
cd agent
MONITOR_GRPC_ADDRESS=localhost:8443 \
MONITOR_TLS_CA_FILE=../.data/certs/server.crt \
MONITOR_ENROLLMENT_TOKEN="$ENROLLMENT_TOKEN" \
MONITOR_SPOOL_DIR=./.monitor-me/spool \
MONITOR_AGENT_STATE_FILE=./.monitor-me/agent-state.json \
./monitor-agent
```

The returned session is written once to the local state file. The parent directory is mode 0700 and the file is mode 0600. On restart, the agent reuses its current session; if that session is revoked or expires, provide a new enrollment token to enroll again. The token itself is never persisted by the agent.

## Metric semantics and limitations

- CPU busy/user/system/iowait percentages use cumulative Linux `/proc/stat` counters. A rate is omitted for the initial snapshot and when counters reset.
- Memory available uses `MemAvailable` when present. Cache is estimated from `Cached + SReclaimable + Buffers - Shmem`, clamped at zero.
- Linux disk sectors are converted using the kernel's 512-byte accounting unit. IOPS are read/write operation counter deltas per second.
- Network byte, packet, and error rates use `/proc/net/dev` counters and the measured elapsed interval.
- Process RSS is resident memory per process, not unique physical memory. At most 128 readable processes are emitted per collection; the process count is independent of RSS visibility.
- Cgroup selectors are descriptive metadata. A project is assigned only when a server/project workload mapping matches; otherwise the sample remains explicitly unclassified or host-total.
- TCP state and listening-port inventory does not provide HTTP request counts, latency, or status codes.

## Backpressure and delivery

When the spool reaches its configured quota, new appends fail with an observable error; already persisted batches remain intact. Delivery uses batches of up to 16 pending records per stream. Backend acknowledgements are sent only after PostgreSQL commits; a lost acknowledgement is safe because retries use the same batch ID and content hash.

Redis live events are best-effort notifications. They are never treated as proof of durability. PostgreSQL history is the recovery path for reconnecting dashboards.
