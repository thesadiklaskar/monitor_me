# Native Monitoring Agent

The native Go agent runs directly on each monitored host. Linux host collection has prior verification; the initial Windows collector and Windows Service Control Manager entry point are implemented but still require runtime acceptance. Target bootstrap installers download the matching artifact; Docker is not required on the monitored machine.

## Current implementation

- Foreground mode for development and native systemd/Windows Service lifecycle support for managed installation, with JSON structured logs, environment validation, and graceful stop handling.
- Configurable 100ms–1m collection interval (500ms default).
- Linux procfs aggregate/per-core CPU, memory, swap, disk throughput/IOPS, network throughput/packets/errors, uptime, process count, and up to 128 readable processes by RSS.
- Initial Windows collector using native operating-system counters through gopsutil: aggregate/per-core CPU, memory and swap, mounted-volume capacity and usage, disk throughput/IOPS, network throughput/packets/errors, uptime, host identity/version, process count, and up to 128 readable processes by RSS.
- TCP connection state counts and listening-port inventory from `/proc/net/tcp` and `/proc/net/tcp6`.
- Process cgroup selectors, Systemd unit identification, and Docker/container cgroup IDs where those paths are exposed by the host.
- Counter-derived rates use actual monotonic elapsed time; counter resets and invalid intervals produce missing rates, not negative values.
- Bounded append-only JSONL spool, fsync on append, private 0700 directory/0600 WAL file, quota enforcement, recovery of a truncated final record, and atomic acknowledgement compaction.
- Optional TLS gRPC enrollment, persisted session credentials, heartbeat, reconnect/backoff, and local batch delivery. A batch is removed from the WAL only after the backend reports a durable commit.
- Agent-health telemetry includes process CPU utilization, collector errors per interval, persistent WAL byte size and unacknowledged batch depth. Last successful send is omitted until a matching durable ACK has been received.
- A hardened Linux systemd unit is provided at `agent/deploy/systemd/monitor-me-agent.service`; the downloadable Linux installer creates the service. The Windows build includes a Service Control Manager entry point and a restart-enabled PowerShell installer. Monitored machines do not need Docker.

macOS and other unsupported operating systems currently report only agent health/spool capabilities. Windows does not currently expose Linux TCP-state/listening-port inventory, cgroup/Systemd workload attribution, or per-process CPU through this collector. Windows Service lifecycle code and target-side Windows Service installer are now present. They still require runtime acceptance on supported Windows releases and publisher code-signing; Windows TCP/listening-port inventory and workload attribution are unsupported.

## Build and test

From the repository root on Linux:

```sh
cd agent
go mod tidy
go test -race ./...
go vet ./...
go build -trimpath ./cmd/monitor-agent
```

To cross-build the Windows x64 agent from Windows PowerShell using the Docker Go builder:

```powershell
.\\scripts\\build-windows-agent.ps1
```

The generated executable is written to the ignored `.data/windows-agent/` directory. The script generates the versioned protobuf sources, resolves Go module dependencies, and builds with `CGO_ENABLED=0`.

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
| `MONITOR_LOG_FILE` | stdout | Optional bounded rotating JSON log output for managed services |
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
- TCP state and listening-port inventory is currently Linux-only and does not provide HTTP request counts, latency, or status codes.
- Windows disk capacity is reported per mounted volume. Disk and network rates use the operating-system counters and measured elapsed interval. Protected or inaccessible Windows processes may be omitted when the agent runs without elevation.
- Windows process RSS is not unique physical memory; shared pages can be present in multiple process working sets.

## Backpressure and delivery

When the spool reaches its configured quota, new appends fail with an observable error; already persisted batches remain intact. Delivery uses batches of up to 16 pending records per stream. Backend acknowledgements are sent only after PostgreSQL commits; a lost acknowledgement is safe because retries use the same batch ID and content hash.

Redis live events are best-effort notifications. They are never treated as proof of durability. PostgreSQL history is the recovery path for reconnecting dashboards.

## Linux systemd service

Build and install the agent binary and unit:

```sh
cd agent
go build -trimpath -o /tmp/monitor-agent ./cmd/monitor-agent
sudo install -m 0755 /tmp/monitor-agent /usr/local/bin/monitor-agent
sudo install -d -m 0700 /etc/monitor-me
sudo install -m 0644 deploy/systemd/monitor-me-agent.service /etc/systemd/system/monitor-me-agent.service
sudo install -m 0600 deploy/systemd/agent.env.example /etc/monitor-me/agent.env
sudo install -m 0644 /path/to/server-ca.crt /etc/monitor-me/server-ca.crt
sudo systemctl daemon-reload
sudo systemctl enable --now monitor-me-agent
sudo systemctl status monitor-me-agent
```

Edit `/etc/monitor-me/agent.env` with the organization's gRPC address and trusted CA path. Put a one-use enrollment token there only for initial enrollment, then remove that line after the private agent state file has been created. The unit runs as root because host-wide process inventory and RSS collection require access to process metadata across users; filesystem writes are confined to `/var/lib/monitor-me`. The WAL and session state inherit restrictive directory/file permissions.

Follow logs with `journalctl -u monitor-me-agent -f`. Use `systemctl restart monitor-me-agent` to restart and `systemctl disable --now monitor-me-agent` to disable and stop the service.


## Windows service installation

1. Open the dashboard, choose **Add Machine**, and create a one-time enrollment token.
2. On the monitored Windows x64 host, open PowerShell as Administrator and run the copied command:

   ```powershell
   & ([scriptblock]::Create((Invoke-RestMethod -Uri 'https://monitor.example/install/agent-windows.ps1'))) -ServerUrl 'https://monitor.example' -GrpcAddress 'monitor.example:8443'
   ```

3. Paste the token at the hidden prompt. The installer downloads the Windows binary and public control-plane CA, verifies SHA-256, installs a restart-enabled Windows Service, stores the private session under ProgramData and removes the one-time token after enrollment.
4. Confirm the service with `Get-Service MonitorMeAgent`. Diagnostic output is stored in the bounded rotating log `%ProgramData%\MonitorMe\agent.log`.

No Docker, Go toolchain or repository checkout is required on the monitored Windows host. Use `-ForceEnrollment` only after creating a replacement token and when you deliberately want to discard the current session. Windows x64 support is the current release target; code-signing/publisher verification and runtime acceptance remain release gates.

