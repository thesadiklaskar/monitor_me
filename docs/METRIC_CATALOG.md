# Metric Catalog

This catalog defines the initial metric contract for Monitor Me. Metric names and semantics must remain stable or change through a documented compatibility decision.

## 1. Timestamp Model

Every sample or batch must preserve the relevant timestamps:

| Timestamp | Meaning |
| --- | --- |
| `collected_at` | Wall-clock time assigned by the agent when the observation is made |
| `monotonic_elapsed` | Monotonic elapsed duration used for rate calculations |
| `received_at` | Time the ingestion service receives the batch |
| `persisted_at` | Time the durable storage operation completes |
| `rendered_at` | Browser-side time when a value is rendered, when measured |

Wall-clock timestamps are useful for correlation across systems. Monotonic clocks must be used for local elapsed-time calculations because wall clocks can jump.

## 2. Host CPU

| Metric | Unit | Type | Initial source / method |
| --- | --- | --- | --- |
| `host.cpu.utilization` | percent | Gauge | Busy counter delta divided by total counter delta |
| `host.cpu.core.utilization` | percent | Gauge | Per-logical-core counter deltas |
| `host.cpu.user` | percent | Gauge | User-time delta |
| `host.cpu.system` | percent | Gauge | Kernel/system-time delta |
| `host.cpu.iowait` | percent | Gauge, where supported | OS-specific wait counter |
| `host.cpu.load_1m` | dimensionless | Gauge, where supported | OS load average |

CPU utilization is computed over the actual elapsed sampling interval. Counter resets, hot-plugged cores, and unsupported fields must be handled explicitly.

## 3. Host Memory

| Metric | Unit | Type | Initial source / method |
| --- | --- | --- | --- |
| `host.memory.total` | bytes | Gauge | OS memory total |
| `host.memory.available` | bytes | Gauge | OS-reported available memory where available |
| `host.memory.used` | bytes | Gauge | Derived from documented OS semantics |
| `host.memory.cache` | bytes | Gauge, where available | OS cache/buffer accounting |
| `host.memory.swap.total` | bytes | Gauge | OS swap total |
| `host.memory.swap.used` | bytes | Gauge | OS swap used |

Memory accounting differs by operating system. The agent must preserve native field semantics and document how derived values are calculated. It must not label a value as universally exact when the OS exposes an estimate or different accounting model.

## 4. Process Memory and CPU

| Metric | Unit | Type | Initial source / method |
| --- | --- | --- | --- |
| `process.cpu.utilization` | percent of one logical CPU, unless explicitly normalized | Gauge | Process CPU counter delta |
| `process.memory.rss` | bytes | Gauge | Resident set size from native process API |
| `process.memory.virtual` | bytes | Gauge, where supported | Virtual address space |
| `process.memory.shared` | bytes | Gauge, where supported | OS-provided shared-memory field |

Process RSS values must not be summed and treated as unique physical memory because shared pages may be counted in multiple processes.

## 5. Disk and Filesystem

| Metric | Unit | Type | Initial source / method |
| --- | --- | --- | --- |
| `host.disk.read.bytes_per_second` | bytes/second | Rate | Read-byte counter delta / elapsed seconds |
| `host.disk.write.bytes_per_second` | bytes/second | Rate | Write-byte counter delta / elapsed seconds |
| `host.disk.read.operations_per_second` | operations/second | Rate | Read-operation counter delta / elapsed seconds |
| `host.disk.write.operations_per_second` | operations/second | Rate | Write-operation counter delta / elapsed seconds |
| `host.disk.utilization` | percent, where supported | Gauge | OS/device busy-time semantics |
| `host.filesystem.used` | bytes | Gauge | Filesystem capacity minus available capacity |
| `host.filesystem.available` | bytes | Gauge | OS filesystem availability |

Device counters and filesystem capacity are different measurements and must not be conflated. Device identity and mount-point identity must be represented separately.

## 6. Network

| Metric | Unit | Type | Initial source / method |
| --- | --- | --- | --- |
| `host.network.receive.bytes_per_second` | bytes/second | Rate | RX byte counter delta / elapsed seconds |
| `host.network.transmit.bytes_per_second` | bytes/second | Rate | TX byte counter delta / elapsed seconds |
| `host.network.receive.packets_per_second` | packets/second | Rate | RX packet counter delta / elapsed seconds |
| `host.network.transmit.packets_per_second` | packets/second | Rate | TX packet counter delta / elapsed seconds |
| `host.network.receive.errors_per_second` | errors/second | Rate | RX error counter delta / elapsed seconds |
| `host.network.transmit.errors_per_second` | errors/second | Rate | TX error counter delta / elapsed seconds |

Interface identity, state, and counter reset behavior must be recorded. Loopback and virtual interfaces should be identifiable so dashboards can avoid misleading host totals.

## 7. Host and Agent Health

| Metric / field | Unit / type | Meaning |
| --- | --- | --- |
| `host.uptime.seconds` | seconds, gauge | Time since host boot |
| `agent.uptime.seconds` | seconds, gauge | Time since agent process start |
| `agent.cpu.utilization` | percent | Agent process CPU consumption |
| `agent.memory.rss` | bytes | Agent resident memory |
| `agent.collector.errors` | count | Collector errors by collector and interval |
| `agent.queue.depth` | samples or batches | Pending telemetry in memory |
| `agent.spool.bytes` | bytes | Persistent local spool size |
| `agent.last_successful_send` | timestamp | Last successful backend delivery |

Agent health metrics are required to distinguish a quiet host from a malfunctioning collector or disconnected agent.

## 8. Workload and Project Metrics

Where the host exposes enforceable workload boundaries, the product may report the same CPU, memory, disk, and network concepts with workload labels:

- `organization_id`
- `server_id`
- `project_id`
- `workload_id`
- `container_id`, when applicable
- `service_id`, when applicable
- `process_id`, when applicable
- `attribution_method`
- `attribution_confidence`, where inference is involved

Attribution methods must distinguish cgroup, container runtime, Systemd unit, explicit configuration, and inferred process-tree relationships. Host totals remain independent from project totals. Shared and unclassified resources must remain visible.

## 9. Application and API Metrics

Application request metrics are not derivable from port discovery alone. Supported sources may include application instrumentation, reverse-proxy integration, or OS tracing with sufficient protocol visibility.

| Metric | Unit | Type | Source |
| --- | --- | --- | --- |
| `application.http.requests_per_second` | requests/second | Rate | Instrumentation or proxy |
| `application.http.request_duration` | milliseconds | Histogram | Instrumentation or proxy |
| `application.http.responses_total` | requests | Counter | Instrumentation or proxy |
| `application.http.errors_total` | requests | Counter | Status-code classification |
| `application.http.error_rate` | percent | Derived | Error requests / total requests over a window |

Latency distributions must retain histogram buckets or another aggregation-friendly representation. Averages alone are insufficient for tail latency analysis. HTTP error-rate definitions must specify which status codes count as errors.

## 10. Sampling and Freshness

- The product target is a configurable 100ms–500ms collection range for supported, low-overhead metrics.
- The default interval must be selected only after collector overhead and system impact have been benchmarked.
- Every sample must include its effective collection interval or enough timing information to derive it.
- Missing samples must not be silently filled with zero.
- The dashboard must communicate stale data and agent disconnection.
- Sampling interval is not end-to-end latency. Ingestion and rendering delays are measured separately.

## 11. Counter Reset and Missing Data Rules

- If a cumulative counter decreases, treat it as a reset or identity change; do not emit a negative rate.
- If an interface, disk, process, or core disappears, close its previous identity series and record the lifecycle event.
- If a metric is unsupported, return a capability status rather than a numeric zero.
- If the elapsed interval is zero or invalid, do not calculate a rate.
- Rate calculations use monotonic elapsed time on the collecting host.

## 12. Platform Differences

Linux may expose detailed counters through `/proc`, `/sys`, cgroups, and optional eBPF. Windows and macOS require native APIs and may expose different fields, permissions, and sampling costs. The agent must report capabilities per operating system rather than pretending every metric is identical.

## 13. Versioning

Metric names, units, labels, and calculation semantics form a compatibility contract. Breaking changes require a documented schema/metric version change, migration plan, and dashboard compatibility review.
