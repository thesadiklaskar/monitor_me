# Architecture Overview

## 1. Status and Intent

This is the Phase 1 proposed architecture baseline, not a claim that the system has already been implemented. Detailed schemas, APIs, threat models, and deployment diagrams will be refined in later phases through code and Architecture Decision Records.

## 2. Technology Responsibilities

| Component | Technology | Responsibility |
| --- | --- | --- |
| Web application | Next.js, TypeScript, Tailwind CSS | Operator workflows, dashboards, inventory, projects, alerts |
| Backend | Go | Authentication, APIs, enrollment, ingestion, query, alert and action orchestration |
| Host agent | Native Go binary | Host collectors, workload discovery, buffering, secure communication |
| Agent protocol | gRPC, Protocol Buffers | Versioned and authenticated agent-to-backend messages |
| Browser live channel | WebSocket | Authorized delivery of live metric and event updates |
| Durable database | PostgreSQL | Application state, metadata, alert records, and historical telemetry |
| Ephemeral cache | Redis | Live cache, transient coordination, short-lived state |
| Local orchestration | Docker Compose | Repeatable local control-plane development and supported single-node deployments |
| Visualization | Custom Canvas/WebGL where justified | Efficient rendering of live and historical metric series |

No existing monitoring platform is part of the proposed architecture.

## 3. Logical Components

### 3.1 Host Agent

The agent is installed as a native service on the monitored machine. It is responsible for:

- Host resource collection.
- Process, workload, and capability discovery.
- Metric normalization and timestamping.
- Bounded in-memory queues.
- Persistent local spool for telemetry not yet acknowledged.
- Authenticated gRPC communication.
- Heartbeats, health reporting, reconnect, and credential rotation.

Collectors are isolated behind platform-specific interfaces. Linux-specific collectors may use `/proc`, `/sys`, cgroups, and optional eBPF. Windows and macOS use native APIs and report their own capability sets.

### 3.2 Backend Control Plane

The Go backend is logically organized around:

- User authentication and authorization.
- Organization, server, and project management.
- Agent enrollment, identity, and revocation.
- Telemetry ingestion and validation.
- Durable writes and historical metric queries.
- Live subscriber management.
- Alert evaluation and event processing.
- Approved remote-action orchestration.
- Health, readiness, configuration, and audit services.

The implementation should begin as a well-structured deployable backend. Components should only be split into separately deployed services when operational or measured scale requirements justify it.

### 3.3 Data Layer

- PostgreSQL is the durable source for application state and historical telemetry.
- Redis is used only for defined ephemeral workloads such as live cache and short-lived coordination.
- Agent persistent spool protects unsent telemetry during temporary disconnections.
- The design must define exactly when a telemetry batch is considered acknowledged and durable.

A custom time-series database is not required for the initial MVP. PostgreSQL partitioning, indexing, batch writes, retention, and downsampling should be measured before considering a separate storage engine.

### 3.4 Web Application

The Next.js application provides:

- Server inventory and Add Server workflow.
- Overall server resource view.
- Global All Projects / project selector.
- Project-specific dashboards.
- Historical charts and time-range controls.
- Process, workload, and application detail.
- Alerts, events, and audit history.
- Authorized remote-action controls.
- User and access management.

## 4. Primary Data Flow

1. An administrator creates a scoped, expiring enrollment token in the dashboard.
2. The agent is installed natively on the monitored host and uses the token to enroll.
3. The backend validates enrollment and binds the agent identity to a server record.
4. The agent collects supported metrics at its configured interval.
5. The agent timestamps and normalizes observations, then buffers them locally.
6. The agent transmits versioned batches over authenticated gRPC.
7. The backend validates, applies idempotency rules, and writes durable telemetry to PostgreSQL.
8. The backend updates defined live state/cache entries in Redis where appropriate.
9. Authorized dashboard subscriptions receive live updates over WebSocket.
10. Historical views query PostgreSQL with bounded time ranges and aggregation.
11. Alert evaluation consumes validated metrics/events and records state transitions.
12. Approved remote actions travel through the authenticated agent control channel and are audited.

## 5. Docker Boundaries

Docker is used for reproducible local control-plane setup through Docker Compose. The initial local stack is expected to include the Go backend, Next.js frontend, PostgreSQL, and Redis.

The monitored-host agent remains a native Go executable/service so it can access host-level operating-system counters without relying on being inside an application container.

Where container monitoring is enabled, the agent may use Docker metadata and cgroup information to identify containers and workloads. Access to the Docker Engine API is security-sensitive; permissions must be minimized and documented. Docker is not mandatory on every monitored machine.

## 6. Project Attribution Model

Project-level resource usage requires explicit attribution:

1. Prefer OS-enforced workload boundaries such as Linux cgroups.
2. Use container runtime metadata to map container identities to projects.
3. Use Systemd service metadata where applicable.
4. Allow explicit operator-maintained mappings.
5. Use process-tree inference only as a lower-confidence fallback.

Every attributed metric should retain its attribution method. Shared host overhead and unclassified resource consumption remain visible rather than being arbitrarily assigned to projects.

## 7. Live and Historical Data

Live and historical paths have different goals:

- Live path: bounded buffering, recent values, low fan-out latency, and explicit stale-state behavior.
- Historical path: durable persistence, query limits, indexes/partitions, retention, and metric-appropriate downsampling.

Redis must not become the only place where telemetry exists. PostgreSQL persistence and agent-side buffering define the durability boundary.

## 8. Security Boundaries

- Agent enrollment credentials are scoped, short-lived, and revocable.
- Agent/backend traffic is encrypted and authenticated.
- User authorization is checked by backend APIs and live subscriptions.
- Organization, server, and project scopes are validated on every relevant request.
- Remote actions use allowlists, least privilege, timeouts, replay protection, and audit records.
- The system does not expose unrestricted remote shell access.
- Host telemetry, labels, and agent-provided metadata are untrusted input.
- Docker Engine access is treated as privileged access.

## 9. Performance and Reliability Boundaries

The design uses bounded queues, cancellation, deadlines, backpressure, retry limits, idempotency, batch writes, connection pooling, graceful shutdown, and health checks.

Collection interval, ingestion latency, backend processing latency, and browser rendering freshness are separate measurements. A 100ms collection interval does not imply 1ms end-to-end latency.

Performance goals must be validated using repeatable test workloads and documented hardware/software environments. Crash-free and loss-free operation cannot be guaranteed for every failure; failure modes and recovery boundaries must be explicit.

## 10. Architecture Decisions Still Required

The following must be resolved and recorded in ADRs before or during Phase 2:

- Authentication/session implementation and organization tenancy model.
- Exact gRPC enrollment handshake and credential rotation protocol.
- Protocol version compatibility and payload limits.
- PostgreSQL schema, partition strategy, and retention defaults.
- Redis topology and behavior during cache outage.
- Agent local spool format, encryption, quota, and acknowledgement semantics.
- WebSocket fan-out design and authorization model.
- Docker Engine access strategy and required permissions.
- Remote-action allowlist and execution identity.
- Installer signing and artifact distribution strategy.


## Phase 3 Implementation Status

The Phase 3 data path now has an initial implementation: native Linux procfs agent -> TLS gRPC stream -> authenticated batch validation -> PostgreSQL transaction -> post-commit organization-scoped Redis Pub/Sub event. The agent retains each batch in a local JSONL WAL until the backend returns an acknowledgement. Redis notifications are ephemeral; historical metric values and batch idempotency hashes live in PostgreSQL.

The browser live endpoint uses a 30-second single-use ticket derived from a valid user bearer session. The ticket is stored by hash and consumed once; WebSocket subscribers are isolated by organization and have a fixed-size outbound queue. Historical queries are authenticated and scoped to the user's organization and requested server. Slow clients reconnect with a fresh ticket and use the durable history endpoint to fill gaps.

This implementation is Linux-first. Windows/macOS collectors, full Docker Engine discovery, process CPU metrics, app request instrumentation, persistent rollup tables, dashboard UI, and production installers are not claimed as implemented.
