# Native Monitoring Agent

The Phase 3 native Go agent runs directly on a monitored Linux host. Docker is not required on the monitored machine.

## Current implementation

- Native foreground process with JSON structured logs and signal-aware shutdown.
- Validated environment configuration with a bounded 100ms–1m collection interval.
- Linux procfs collection for aggregate/per-core CPU, memory, swap, disk counters, network counters, uptime, process count, and the top 128 visible processes by RSS.
- Counter-derived rates use monotonic elapsed time. Counter resets and invalid intervals produce missing samples rather than negative values.
- Append-only JSONL telemetry spool with fsync, private directory/file permissions, bounded disk quota, restart recovery, and atomic acknowledgement compaction.
- Explicit capability reporting. Unsupported operating systems do not claim Linux procfs capabilities.

The agent currently persists collected batches locally. Secure enrollment, gRPC delivery, and durable backend ingestion are the next Phase 3 implementation slice; local spooling must not be mistaken for a connected or centrally monitored host.

## Build and test

Run from the repository root:

    cd agent
    go test -race ./...
    go vet ./...
    go build -trimpath ./cmd/monitor-agent

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| MONITOR_AGENT_VERSION | 0.1.0-dev | Agent build/version string |
| MONITOR_COLLECTION_INTERVAL | 500ms | Collection interval; minimum 100ms |
| MONITOR_PROC_ROOT | /proc | Procfs mount or fixture root |
| MONITOR_SPOOL_DIR | ./.monitor-me/spool | Local durable queue directory |
| MONITOR_SPOOL_MAX_BYTES | 67108864 | Maximum local spool size (64MiB) |
| MONITOR_LOG_LEVEL | info | debug, info, warn, or error |

The spool directory is created with mode 0700 and the WAL file with mode 0600. Protect the agent state directory with host-level access controls. A full spool applies backpressure by rejecting new batches; it does not silently discard already persisted batches.

## Metric semantics and limitations

Linux CPU and device/network rates are calculated from cumulative counters using the actual monotonic elapsed interval. The initial snapshot intentionally omits rates because no previous counter exists. Device/interface identities are labels; a newly appearing device has no rate until a second sample is available. Linux disk sectors are converted using the kernel's 512-byte accounting unit.

Memory available uses MemAvailable when the kernel provides it. Cache is an estimate from Cached + SReclaimable + Buffers - Shmem, clamped at zero. Process RSS is per-process resident memory and must not be summed as unique physical memory. At most 128 readable processes are emitted per collection; process count is independent of RSS visibility.

The initial collector reads procfs and does not yet discover container/project attribution, Systemd units, listening sockets, or application request metrics. Those capabilities must be implemented and tested before being advertised.
