# ADR 0005: ClickHouse as the high-volume log/event store

- **Status:** Accepted for the initial Linux-only slice; runtime, backup and benchmark acceptance pending
- **Date:** 2026-11-24

## Context

The current pipeline collects host metrics, persists metric batches through a bounded agent spool, writes metric history to PostgreSQL and uses Redis for best-effort live notifications. An initial Linux journald log path now uses a separate bounded spool, authenticated TLS gRPC batch delivery, ClickHouse MergeTree storage and bounded search.

The product goal includes searchable Linux and Windows logs at high event volumes without browser, agent, API or storage crashes. Keeping raw high-volume logs in the current transactional PostgreSQL tables would couple control-plane and metric workloads to a different write/query profile.

## Decision

Evaluate a separate ClickHouse MergeTree data plane for raw logs and high-volume events while retaining:
- PostgreSQL for installation, node, enrollment, alert, audit and current metric state/history initially.
- Redis for ephemeral live fan-out only, never as the durable source of truth.
- Per-agent durable WAL/spool for disconnected delivery and retry.

The log event envelope should carry event time, observed time, installation/node/source identity, severity, body, optional trace/span IDs, a stable event ID and bounded structured attributes.

The ingestion path must:
1. Collect source logs on the native host agent (Linux journald/files; Windows Event Logs/files).
2. Batch and compress events before transport.
3. Bound memory queues and persistent spool size; expose drop, throttle, queue and disk-full health metrics.
4. Apply back-pressure and retry with jitter.
5. Acknowledge an idempotent batch only after ClickHouse reports the configured durable insert completion.
6. Query only bounded time ranges and bounded keyset pages with server-side cancellation, memory and execution limits. The initial API limits each request to a seven-day range and returns at most 200 visible rows per page using `(event_time,event_id)` cursors.

The initial ClickHouse table should use a MergeTree-family engine, time-based partitioning appropriate to measured retention/volume, an ordering key that matches common node/time/source/severity queries, compression codecs and selective skipping/text indexes. Keep common fields as typed columns and bound arbitrary attribute cardinality in a map/raw JSON field. Use TTL/partition cleanup for retention. Avoid one-row inserts and avoid creating a storage column for every arbitrary log attribute.

## Performance and reliability gates

The initial proposed reference gate is 10,000 structured log events/sec sustained for 60 minutes and a separately measured 100-million-event search/retention test. Both are acceptance targets, not current performance claims. Reports must include hardware, software versions, event shape/size, source count, ingestion acknowledgement latency, sustained rate, p50/p95/p99 query latency, CPU/RSS, disk growth, retention, restart/replay and data-loss/duplicate behavior.

## Consequences

- ClickHouse is provisioned in local and production Compose profiles and the first Linux journald ingestion/search path is implemented. Windows Event Log/file readers, ClickHouse backup/restore, index tuning and realistic resource profiling remain open.
- Metrics and raw logs retain separate query paths and retention controls.
- A bounded browser Log Explorer must virtualize results and never materialize millions of events in the DOM.
- Backups, restore, security updates, disk sizing and health checks for ClickHouse become mandatory before production release.
