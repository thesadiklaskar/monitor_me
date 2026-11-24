# Monitor Me — Plug-and-Play Observability Product Contract

**Status:** Target product architecture, not a claim of shipped functionality.  
**Date:** 2026-11-24

## Product outcome

Monitor Me is a self-hosted, single-install observability control plane intended to replace the normal day-to-day Grafana-style host monitoring and log exploration workflow. An operator installs one control plane, opens one dashboard, creates a short-lived node enrollment token, and runs a single bootstrap command on each target machine. The node installer detects the target OS and architecture itself, installs the matching native collector, enables its platform service, enrolls with the control plane and starts telemetry delivery.

The operator must not need to create a user, sign in, create an organization, choose the target OS in a wizard, install Go/Node/protoc, hand-edit service environment files, generate certificates manually, or configure per-node collector settings for the supported default path.

## Non-negotiable operator experience

### Control plane

- One installation command per supported control-plane OS. The installer detects Linux distribution/architecture, checks Docker Engine and Compose v2, creates strong per-install secrets and a local TLS identity, writes a private .env, starts the supported Compose profile, waits for database migrations and readiness, and prints/opens the dashboard URL.
- Provide a first-run path for a local install and a public/VPS install. Local installs remain loopback-only by default. Public installs require HTTPS and an explicit, safe operator-network policy.
- Preserve existing data when upgrading. Never delete Compose volumes, reset the database, or overwrite operator secrets as part of an upgrade.
- Create one installation metadata record automatically. There is no hosted control plane, SaaS tenant, login/signup, customer profile, user administration, company/organization membership, organization selector, or organization-based API workflow. The target schema removes legacy users, organizations, organization_memberships, user_sessions, user actor references, and organization_id columns, filters, and foreign keys. Existing installations require an explicit, tested forward migration that preserves customer data; never drop records or Compose volumes to achieve the new schema.

### Monitored-node bootstrap

- The dashboard creates a single-use, short-expiry node token and exposes two copyable launcher commands solely because Windows PowerShell and POSIX shells are different execution environments. There is no manual OS selector.
- The launcher running on the target detects OS, architecture, service manager, init system, virtualization metadata and available cloud metadata locally. It chooses the matching signed native artifact and platform service automatically.
- Linux installs a systemd service where available. Windows installs a Windows Service with recovery/restart policy. macOS/launchd is a later supported target; do not advertise it before the collector and service are tested.
- A target agent can be installed without Docker, Go, Node.js, a repository checkout or manual configuration. It needs only outbound TLS to the control plane. A monitored host does not need an inbound Monitor Me firewall rule.
- The target reports its actual OS, version, architecture, virtualization/container classification and cloud-provider metadata where available. Unavailable metadata is unknown, never a guessed provider.
- The token is used once to exchange for a device-bound session credential. Never place a reusable credential in shell history, process arguments, URLs or logs. Support revocation, rotation, uninstall and upgrade.

### No-login single-install access

- Do not show login, signup, create-organization or user-management screens in the default single-install UI.
- The installation descriptor is generated automatically at first boot; it is not a user, profile, organization, or hosted tenant.
- No-login is a network access mode, not a claim that an internet-facing dashboard is safe. The local profile binds to loopback. A remote profile must use HTTPS and restrict operator UI access to an explicitly configured private/VPN/admin network. The public agent enrollment endpoint is separate and accepts only short-lived single-use enrollment tokens and established agent sessions.
- Keep PostgreSQL, Redis, ClickHouse, internal API and internal web ports private. Never publish data-store ports.
- Do not automatically alter cloud security groups or network ACLs without provider IAM credentials and an explicit policy. Host firewall automation must be idempotent, source-aware, additive by default, verified and reversible. Agent-only hosts require outbound connectivity only.

## Automatic host discovery contract

The agent—not the browser, Compose container or manually selected dropdown—must report the target machine's facts. Discovery fields include:

- OS and distribution/product, version, kernel and architecture.
- Bare-metal vs virtual-machine vs container where reliable native signals exist.
- Hypervisor/vendor and cloud provider where metadata services or DMI/SMBIOS identify them.
- CPU topology, memory, swap, volumes/mounts, disks, network interfaces, uptime and process inventory.
- Agent version, capabilities, service state, last heartbeat and collector health.

Discovery is capability-based. Windows does not claim Linux cgroup/Systemd/TCP metrics; Linux does not claim unimplemented Windows Event Log or service metrics. Every panel distinguishes unsupported, unavailable, stale, permission-denied and zero.

## Observability data architecture

The current implementation has a host-metrics pipeline with a persistent agent spool, TLS gRPC ingestion, PostgreSQL history and Redis live notifications. An initial Linux journald → bounded log spool → TLS gRPC → ClickHouse path and a keyset-paginated Log Explorer have been implemented. Windows Event Log, file-tail sources, indexed full-text search, native push log streaming and trace ingestion remain incomplete and require explicit acceptance.

Recommended target data planes:

| Data | Target storage / transport | Design purpose |
| --- | --- | --- |
| Installation, nodes, tokens, alert rules and audit metadata | PostgreSQL | Transactional control-plane state |
| Host metrics and bounded metric history | Existing PostgreSQL path initially; benchmark before changing | Time-series queries and dashboards |
| Raw logs and high-volume events | ClickHouse MergeTree | Compressed columnar storage, bounded time-range search, filtering and retention at high write volume |
| Live status and ephemeral fan-out | Redis | Non-authoritative live notifications only |
| Agent delivery protection | Per-agent persistent WAL/spool with idempotent batch IDs | Offline buffering, retries and acknowledgement only after durable acceptance |

Do not send one insert per log event. Batch on agents and/or server-side ingestion; bound memory and queues; apply back-pressure; acknowledge a batch only after ClickHouse confirms the configured durable insert policy. Use idempotent event/batch IDs so retries after lost acknowledgements do not duplicate visible rows. Query APIs must require a time range, page/cap result size, enforce query timeouts and protect the storage layer from unbounded scans.

### Log collection scope

- Linux: journald and configured syslog/file sources, with cursor/inode+offset checkpointing and log rotation support.
- Windows: Windows Event Logs (Application/System/Security as permitted) and configured file sources.
- Common structured envelope: event timestamp, observed timestamp, node ID, source, severity, body, trace/span IDs when present, stable event ID and bounded structured attributes.
- Support multiline parsing, structured JSON, filtering/exclusion, redaction, per-source rate and byte limits, disk quotas, compression and clear drop/overflow counters.
- Do not create an unbounded column per arbitrary log attribute. Promote common searchable fields and keep the remaining fields in a bounded map/raw JSON payload. Limit label/attribute cardinality.
- Add traces/OpenTelemetry ingestion only after logs and metrics pipelines have a durable shared identity/correlation model.

ClickHouse is installed in the Compose profiles and an initial MergeTree schema is used by the Linux journald path. The current 30-day TTL and 90-day PostgreSQL idempotency ledger are implemented, but production backup/restore, index tuning, Windows/file source coverage and resource budgeting must be validated with realistic log distributions before any throughput claim.

## Professional monitoring-console UI

- A collapsible/expandable icon-led sidebar with accessible tooltips and persisted preference.
- Consistent icon system, compact typography, clear information hierarchy, precise units, timestamps and timezone.
- Global node selector, time-range picker, refresh/live state, health indicators and search.
- Purpose-built overview, node details, metrics, processes, network, services, alerts, operations, log explorer and settings pages. Remove placeholder menu items and irrelevant phase/status copy.
- Log Explorer: time picker, severity filter, source/node filters, full-text query, structured attribute filters, polling live tail with pause/resume, JSON detail, context around an event, keyset pagination/virtualized rendering and saved queries. Initial severity/source/text filters, event JSON detail and bounded 200-row keyset pages are implemented; context search and saved queries remain.
- Useful no-data/unsupported/permission-denied/stale/error states; never show fabricated zeroes or pretend unavailable capabilities are active.
- Dense tables and virtualized lists for large data. Never render millions of log rows into the browser at once; query and page results server-side.
- Keyboard navigation, visible focus, responsive layouts, no decorative controls that do not work.

## Reliability, security and scale gates

- Bounded queues, memory budgets, disk quotas, CPU/network limits, timeouts, cancellation and retry jitter on agent, API, ingestion and UI paths.
- Load shedding and explicit drop/overflow metrics instead of process crashes or unbounded memory growth.
- Crash/restart tests for agent, API, ClickHouse and network interruption; WAL recovery and duplicate replay tests.
- Benchmarks must publish host hardware, versions, number of nodes, events/sec, bytes/sec, event-size distribution, retention, query patterns, p50/p95/p99 latency, CPU, RSS, disk growth and recovery behavior.
- Initial acceptance target: ingest and query at least 10,000 structured log events/second sustained for 60 minutes on a documented single-node reference profile, with a separately recorded 100-million-event search/retention test. This is a proposed gate, not a current capability or guarantee. Raise the target after measured profiling.
- Query a constrained time window and return bounded pages; cancellation and per-query memory/time limits are mandatory.
- Ensure the dashboard is never publicly anonymous by accident. Local no-login mode stays loopback-bound; remote no-login mode must be gated by an explicit network-access policy and HTTPS. Enrollment tokens are never substitutes for dashboard network security.

## Delivery sequence

1. Remove browser-OS selection and make Add Node issue one token plus OS-agnostic instructions; show PowerShell and POSIX launcher options only.
2. Replace source-checkout/Docker-based node helpers with versioned, signed native artifacts and universal target-side discovery/bootstrap. Add Windows Service lifecycle and Linux service lifecycle, upgrade/uninstall/rollback.
3. Convert the product to an implicit single-install workspace and remove login/signup/organization UI. Add safe network-only operator access for remote deployments; keep local no-login loopback-only.
4. Implement automatic control-plane bootstrap, secrets/TLS generation, migrations, readiness, supported Linux host firewall profiles and diagnostics. Document where cloud-provider APIs cannot be configured automatically without IAM.
5. Implement ClickHouse-backed durable log ingestion, native Linux/Windows log collection, indexing, filtering, live tail, retention and backup.
6. Replace the prototype dashboard shell with the complete monitoring-console design and a virtualized Log Explorer.
7. Run fault-injection, security, cross-platform and sustained load tests. Do not claim Grafana replacement or million-log reliability until gates pass.


## Strict single-install architecture clarification — 2026-11-24

This clarification supersedes any earlier wording that allowed organization tenancy keys to remain indefinitely as an invisible implementation detail. One installation is one owner's isolated product boundary and datastore. Monitor Me is not a hosted service and does not ask an operator to create a customer profile, user, company, organization, or workspace. Servers, metrics, logs, dashboards, alerts and operations belong directly to that installation. Optional grouping may exist only as a non-security label, never as a tenant or access-control boundary.

The current source still contains legacy user/organization tables and organization_id query, key and foreign-key paths because the original relational schema and dependent resource tables rely on them. These are migration debt, not an accepted final design. Removal requires a numbered forward migration covering every dependent table, index, query, API payload, test fixture and retention path. It must preserve existing servers, agent identities, metric history, alert history, dashboards, audit rows and log idempotency records. It must stop safely on ambiguous legacy data instead of silently selecting an organization or deleting records. Do not use destructive volume resets as a migration strategy.

A move to a different GitHub owner or repository name must not require source edits. Go module paths are stable internal import namespaces, not published repository URLs. Artifact origins, release channels and public download bases must be operator-configurable; source must not bake in a personal account, repository URL, deployment hostname or example customer URL.
