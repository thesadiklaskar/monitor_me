# Native monitoring agent

Monitor Me's native Go agent runs on each monitored machine. It collects real host telemetry and supported host logs, stores unacknowledged batches in a bounded local write-ahead log, and sends them to the owner's self-hosted control plane over authenticated TLS gRPC. Docker is not required on monitored hosts.

## What it collects

### Linux

- Aggregate and per-core CPU utilisation.
- Memory used/available/cache and swap.
- Disk capacity, read/write throughput and IOPS.
- Mounted filesystem total/used/free capacity and usage percentage are sampled once per minute per readable mount; samples include mountpoint, device and filesystem labels.
- Network bytes, packets and errors per second.
- Host uptime, process count, top readable processes by RSS.
- TCP connection state and listening-port metadata.
- systemd journal events where the agent has permission to read them.
- Process, systemd unit, cgroup and container identifiers as host metadata only. These are not customer projects or access scopes.

### Windows x64

- CPU, memory/swap, mounted-volume capacity, disk/network throughput, uptime and process inventory.
- Initial Application/System Event Log reader source is implemented, but Windows service installation, event ingestion and restart recovery still need end-to-end runtime acceptance.
- Linux-only TCP, journald, cgroup and systemd collectors are not available on Windows.

### Unsupported targets

macOS, Windows ARM64 and BSD are not released agent targets. Do not present the source stubs as supported collection.

## Installation

1. Open the self-hosted Monitor Me dashboard.
2. Select **Add machine** and copy the one-time enrolment command for the target OS.
3. Run it on the machine with the required administrator/root privileges.
4. Confirm the machine appears under **Servers** and starts reporting fresh metrics.

The installer detects supported OS/architecture, downloads the matching versioned artifact and public CA, verifies the artifact checksum, installs the native service and enrols it with a short-lived credential. Monitored nodes connect outbound to the control plane (HTTPS for bootstrap/artifacts and TLS gRPC, default TCP 8443, for telemetry); they do not need an inbound Monitor Me firewall rule.

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `MONITOR_AGENT_VERSION` | `0.1.0-dev` | Agent build/version |
| `MONITOR_COLLECTION_INTERVAL` | `500ms` | Collection interval; minimum 100ms |
| `MONITOR_PROC_ROOT` | `/proc` | Procfs mount or test fixture root |
| `MONITOR_SPOOL_DIR` | `./.monitor-me/spool` | Durable local queue directory |
| `MONITOR_SPOOL_MAX_BYTES` | `67108864` | Maximum local spool size (64 MiB) |
| `MONITOR_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `MONITOR_LOG_FILE` | stdout | Optional bounded rotating JSON log file |
| `MONITOR_GRPC_ADDRESS` | unset | TLS gRPC control-plane address |
| `MONITOR_TLS_CA_FILE` | unset | CA certificate used to verify the control plane |
| `MONITOR_ENROLLMENT_TOKEN` | unset | Short-lived, one-use enrolment token |
| `MONITOR_AGENT_STATE_FILE` | spool directory + `agent-state.json` | Private persisted agent session |

Transport stays disabled unless both the gRPC address and CA file are configured. Do not store secrets in source control, service manifests or diagnostic logs. The one-time token is removed from the process environment after initial enrolment and is not persisted by the agent.

## Build and test

Use the Go version specified by `go.mod` (currently Go 1.27.4). From the repository root:

```bash
cd agent
go test -race ./...
go vet ./...
go build -trimpath ./cmd/monitor-agent
```

Run dependency maintenance such as `go mod tidy` deliberately in a networked developer environment, review the resulting `go.mod`/`go.sum` diff, and commit both files. Do not run module tidy as an incidental Docker image build step.

Linux/Windows release artifacts are produced through `deploy/agent-artifacts/Dockerfile` and the corresponding target-side installers. Windows Service and Linux systemd lifecycle tests must be run on real supported operating systems before declaring a release supported.

## Reliability and data safety

- Batches are appended to a bounded JSONL write-ahead log and flushed before delivery.
- Files/directories use restrictive permissions (0600/0700).
- Retries use bounded backoff; unacknowledged batches survive process restart.
- A batch is removed from the spool only after the backend acknowledges durable acceptance.
- Retries reuse the batch identifier/content hash, so a lost acknowledgement does not create duplicate visible data.
- Quota exhaustion is surfaced as an error; the agent must not discard older unacknowledged data silently.

## Development notes

- Main entry point: `cmd/monitor-agent`.
- Platform collectors: `internal/collector/` and `internal/logcollector/`.
- Durable delivery: `internal/spool/` and `internal/transport/`.
- systemd unit and example environment: `deploy/systemd/`.
- Keep platform-specific code behind OS-specific files and report unsupported capabilities explicitly.
- The agent is read-only monitoring software; do not add remote shell, process stop or service restart commands.
- Keep this file as the only agent-level Markdown guide.
