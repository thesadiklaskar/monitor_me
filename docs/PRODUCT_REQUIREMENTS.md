# Product Requirements — Monitor Me

## 1. Document Control

| Field | Value |
| --- | --- |
| Product | Monitor Me |
| Document status | Approved baseline for implementation planning |
| Baseline authored | Phase 1 — Product Specification and Technical Blueprint |
| Latest audited delivery state | Phases 1–3 completed; Phase 4 — Dashboard and Operational Features not started |
| Product category | Custom infrastructure and application monitoring |
| Primary audience | Infrastructure engineers, platform teams, and engineering organizations |
| Language | English |

This document translates the product brief into an implementation-oriented specification. The product is being built as an original monitoring platform. It must not embed or depend on an existing monitoring platform such as Grafana, Prometheus, Zabbix, or Netdata.

> **Superseding product decision — 2026-11-24:** This original Phase 1 requirements baseline included team, project, organization and remote-operation features that are no longer in scope. The current product is an individual owner's independent self-hosted monitoring installation. No browser login/signup, user management, organization, customer project, workspace selector, or remote host-control action. Primary navigation is Servers, Logs, Dashboards and Alerts. Host/process/systemd/container/cgroup data remains technical host context only. Requirements FR-009 (project views), FR-010 (project attribution/mappings), FR-014 (remote host actions) and FR-018 (enterprise organization access) are **retired**. The remaining legacy code and schema are migration debt, not target functionality. See [the current product contract](PRODUCT_VISION_PLUG_AND_PLAY.md) and [release backlog](PRODUCT_BACKLOG.md).

## 2. Product Vision

Monitor Me is intended to provide a simple, secure, cross-platform way to enroll machines, observe host and project-level resource usage, investigate application behavior, receive alerts, and perform controlled operational actions.

The product should combine a low-friction user experience with reliable telemetry collection and a backend architecture that can grow from a small installation toward large fleets.

The product is also expected to demonstrate professional engineering practices: clear architecture, measurable performance, safe operations, tests, meaningful Git history, and complete documentation.

## 3. Product Principles

1. **Custom monitoring product:** Build our own host agent, telemetry pipeline, metric model, alerting, project attribution, and dashboard experience.
2. **Simple operator experience:** Common tasks should be understandable without requiring users to learn a complex query language.
3. **Accurate measurement:** Define units, timestamps, collection semantics, and attribution limits. Never invent or silently approximate a metric.
4. **Durable telemetry:** Temporary cache loss or network interruption must not silently destroy data that the system has acknowledged as durable.
5. **Secure by default:** Authenticate agents, encrypt communication, apply least privilege, and audit privileged actions.
6. **Cross-platform with honest capability reporting:** Share common concepts while using native collectors and documenting OS-specific differences.
7. **Measured performance:** Publish benchmark results and test conditions rather than claiming zero lag, zero crashes, or unlimited scale.
8. **Incremental delivery:** Keep each phase independently reviewable, testable, documented, and resumable.

## 4. Users and Primary Workflows

### 4.1 Infrastructure Operator

- Creates an organization or workspace.
- Adds a server from the dashboard.
- Installs and enrolls the host agent.
- Confirms that the server is connected and reporting.
- Opens the overall server view.
- Investigates resource usage and historical trends.
- Receives and acknowledges alerts.

### 4.2 Project or Service Owner

- Selects a project from the global project selector.
- Reviews metrics attributed to that project.
- Identifies its processes, containers, ports, and supported application telemetry.
- Reviews project-specific alerts and events.
- Requests an approved restart or stop operation when authorized.

### 4.3 Platform Administrator

- Manages users, roles, servers, projects, and agent credentials.
- Configures retention and alert policies.
- Reviews remote-action audit events.
- Manages upgrades, backups, and operational settings.

## 5. Scope

### 5.1 Product Capabilities

The target product includes:

- One-click server enrollment and guided installation.
- Host inventory and agent health.
- Overall host CPU, memory, disk, and network telemetry.
- Per-core CPU and process-level resource measurements where supported.
- All Projects and project-specific views.
- Docker container and Systemd service discovery where supported and authorized.
- Process, listening-port, and connection visibility.
- Application request latency and HTTP error metrics through supported instrumentation or integrations.
- Live dashboards and historical charts.
- Configurable alerts, events, and notifications.
- Secure, allowlisted remote process or service actions.
- Historical retention, aggregation, and downsampling.
- Linux, Windows, and macOS agents with capability-specific behavior.
- Local development and supported deployment through Docker Compose, plus native agent installation on monitored hosts.

### 5.2 Initial MVP Boundary

The first usable MVP must prove the complete basic path on Linux before attempting broad feature parity across every operating system.

MVP includes:

1. A runnable Go backend and Next.js frontend.
2. PostgreSQL persistence and Redis for explicitly ephemeral live state/cache.
3. Secure agent enrollment and authenticated gRPC communication.
4. A native Linux Go agent.
5. CPU, memory, disk, network, uptime, and agent-health collection.
6. Durable agent-side buffering for temporary disconnections.
7. Server inventory and an overall server dashboard.
8. Basic historical queries.
9. A repeatable local deployment using Docker Compose.
10. Tests and documentation for the complete enrollment-to-dashboard path.

### 5.3 Post-MVP / Enterprise Scope

The following remain planned until implemented and validated:

- Project-level attribution across Docker, cgroups, Systemd, and process trees.
- Advanced eBPF-based tracing.
- Application request latency and HTTP status metrics.
- Advanced alert policies and notification integrations.
- Secure remote restart and stop workflows.
- Windows and macOS agent capability coverage.
- Signed installers, automated upgrades, rollback, and uninstall.
- Distributed ingestion, high availability, and fleet-scale capacity validation.
- Advanced retention, downsampling, and multi-tenant operations.

## 6. Functional Requirements

Priority definitions:

- **P0:** Required for the first usable MVP.
- **P1:** Required for a complete first production release.
- **P2:** Advanced capability or scale enhancement.

### FR-001 — Server Enrollment (P0)

The dashboard shall provide an Add Server workflow that creates a scoped, expiring enrollment credential and an installation instruction for the selected operating system.

Acceptance:
- An authorized user can create an enrollment token.
- Tokens have an expiry and can be revoked.
- The agent can enroll with a valid token and is rejected with an invalid, expired, or revoked token.
- The enrolled agent receives a stable server identity.
- The dashboard shows connection and last-seen status.

### FR-002 — Native Host Agent (P0)

The product shall provide a native Go agent for the monitored host.

Acceptance:
- The agent can run as a foreground process for development and as an OS service in supported deployment modes.
- Collector failures are isolated and reported.
- Shutdown releases resources and flushes buffered data according to the durability policy.
- Agent health, version, OS, architecture, and supported capabilities are reported.

### FR-003 — Host Resource Telemetry (P0)

The agent shall collect host CPU, memory, disk, network, uptime, and collection-health metrics.

Acceptance:
- Every metric has a documented name, unit, source, and calculation.
- Counter-based rates use elapsed monotonic time and handle counter resets.
- Missing or unsupported metrics are reported as unavailable, not fabricated.
- Collection interval is configurable within validated platform limits.

### FR-004 — Telemetry Ingestion (P0)

The backend shall accept authenticated, versioned telemetry batches over gRPC using Protocol Buffers.

Acceptance:
- Invalid payloads and unauthorized agents are rejected.
- Batches are validated before persistence.
- Duplicate batch delivery is safe and does not create duplicate logical samples.
- Queue and worker limits are bounded.
- Backpressure is observable and does not result in unbounded memory growth.

### FR-005 — Durable Buffering and Recovery (P0)

The agent shall retain unsent telemetry in a bounded persistent local spool.

Acceptance:
- A network interruption causes queued data to be retained within configured spool limits.
- Agent restart recovers valid queued records.
- Successful delivery advances or removes acknowledged records.
- Spool exhaustion follows a documented policy and produces an observable health signal.
- The product documents any data-loss boundary caused by disk failure, quota limits, or forced termination.

### FR-006 — Server Overview (P0)

The dashboard shall present overall server health and host resource metrics.

Acceptance:
- The page identifies the selected server and its agent connection state.
- CPU, memory, disk, and network values show units and timestamps.
- Empty, stale, loading, and disconnected states are distinguishable.
- The UI does not present stale data as live data.

### FR-007 — Historical Metrics (P0)

The backend shall persist host metrics in PostgreSQL and support bounded time-range queries.

Acceptance:
- Queries require a server, metric scope, and time range.
- Results are ordered and timestamped.
- Query limits and retention behavior are documented.
- Redis loss does not remove already persisted historical records.

### FR-008 — Live Dashboard Updates (P0)

The frontend shall receive authorized live updates without requiring a full page reload.

Acceptance:
- Live subscriptions are scoped to the authenticated user and authorized server.
- Reconnection restores the subscription.
- Slow clients cannot cause unbounded server-side buffering.
- UI freshness is displayed or otherwise observable.

### FR-009 — Overall and Project Views (P1)

The product shall provide an All Projects / Overall Server view and project-specific views.

Acceptance:
- Overall host metrics remain distinguishable from project-attributed metrics.
- Project selection changes all project-scoped panels and subscriptions without a full page reload.
- Shared and unclassified resources are represented explicitly.
- The system never assigns ambiguous resource usage to a project without recording the attribution method and confidence.

### FR-010 — Workload Discovery and Attribution (P1)

The agent shall discover supported workloads and map them to projects using configured mappings and operating-system metadata.

Acceptance:
- Docker containers and Systemd services can be discovered where supported and authorized.
- Linux cgroups are the preferred boundary for resource attribution when available.
- Process-tree inference is marked as inferred and is not represented as exact isolation.
- Workload-to-project mappings can be reviewed and corrected by an authorized user.
- Access to the Docker Engine API is restricted to the minimum required operations; unrestricted Docker socket access must not be granted casually.

### FR-011 — Process, Port, and Connection Visibility (P1)

The product shall report supported process and socket metadata.

Acceptance:
- Process identity and resource values are timestamped.
- Listening ports and connection counts are scoped to a host and, where possible, a workload.
- Platform permission gaps are visible.
- Socket metadata is not presented as HTTP request latency or status-code telemetry.

### FR-012 — Application Request Metrics (P1)

The product shall support request latency and HTTP status/error metrics through explicitly supported sources such as application instrumentation, reverse-proxy integrations, or supported tracing.

Acceptance:
- Each integration documents its required configuration and limitations.
- A controlled test application can be used to verify latency and status-code counts.
- Port scanning alone is never treated as sufficient evidence of HTTP request latency or error rates.

### FR-013 — Alerts and Events (P1)

The backend shall evaluate configurable alert rules for supported metrics and workload states.

Acceptance:
- Rules define scope, condition, threshold, evaluation interval, and recovery behavior.
- Alert transitions are deduplicated and timestamped.
- Alert evaluation delay is measurable.
- A disconnected agent is distinguishable from a healthy host with no activity.
- Notification failures are observable and retry behavior is bounded.

### FR-014 — Secure Remote Actions (P1)

The product shall support a controlled set of remote operations such as restarting or stopping an explicitly registered process, service, or container.

Acceptance:
- Only allowlisted operations are executable.
- Authorization and least-privilege checks occur before execution.
- The user, target, requested action, result, and timestamps are audited.
- Commands have timeouts and replay protection.
- Arbitrary remote shell execution is not exposed as a monitoring feature.

### FR-015 — Cross-Platform Agent (P1)

The product shall provide native collectors for supported Linux, Windows, and macOS versions.

Acceptance:
- Each release publishes a tested OS/architecture support matrix.
- Each agent reports its capability set.
- Unsupported features are reported as unavailable.
- Equivalent metrics use consistent semantics where the operating systems expose comparable data.

### FR-016 — One-Click Installation and Lifecycle (P1)

The dashboard shall generate an installation flow for supported systems.

Acceptance:
- Downloaded artifacts are versioned and signature/checksum verified before execution.
- Enrollment credentials are scoped and short-lived.
- Installation, upgrade, rollback, and uninstall are documented and tested per supported OS.
- Installer output does not expose credentials.

### FR-017 — Retention and Downsampling (P1)

The storage layer shall support configurable retention and aggregated views for longer time ranges.

Acceptance:
- Raw and aggregated data have documented resolution and retention.
- Aggregation functions are metric-appropriate.
- Long-range dashboard queries use bounded result sizes.
- Downsampling does not overwrite the raw samples before the configured raw retention expires.

### FR-018 — Enterprise Access and Audit (P1)

The product shall provide role-based access to organizations, servers, projects, alert rules, and remote actions.

Acceptance:
- Authorization is enforced in backend APIs and live subscriptions.
- Cross-tenant access tests are included.
- Privileged changes and remote actions create audit records.
- Secrets are never stored in plaintext logs.

### FR-019 — Performance and Scale Validation (P2)

The product shall have repeatable benchmark and load-test suites.

Acceptance:
- Test reports identify hardware, OS, build, configuration, workload, duration, and results.
- Collection overhead, ingestion throughput, processing latency, storage latency, query latency, and dashboard rendering are measured separately.
- Scale claims are limited to demonstrated test conditions.
- Million-host support is a long-term capacity objective, not an assumed property of the initial implementation.

## 7. Non-Functional Requirements

### Reliability and Data Integrity

- Bounded queues, explicit backpressure, timeouts, and cancellation are required.
- Agent reconnect and retry behavior must be deterministic and bounded.
- Durable acknowledgement semantics must be documented.
- Redis is an ephemeral cache/coordination component, not the sole source of truth for telemetry.
- PostgreSQL stores durable historical metrics and application state.
- Agent-side persistent buffering protects against temporary network outages within configured limits.
- All services must support graceful shutdown and health/readiness reporting.

### Performance

- The initial target collection range is configurable from 100ms to 500ms where a collector and host can sustain it.
- A 100ms sampling target is not a guarantee of 1ms accuracy or 1ms end-to-end delivery.
- Collection interval, ingestion latency, server processing latency, and dashboard freshness are separate measurements.
- The dashboard should target smooth 60 FPS interaction on documented reference hardware, measured with representative data volume.
- Resource overhead must be measured before selecting default sampling intervals.

### Security

- Agent-to-backend communication uses TLS.
- Enrollment credentials are short-lived, scoped, revocable, and never logged.
- User authentication and authorization are enforced server-side.
- Remote actions use explicit allowlists and least privilege.
- Installer artifacts are verified before execution.
- Telemetry and labels are treated as untrusted input.
- Docker access is capability-scoped and must not expose unrestricted host control by default.

### Maintainability

- All repository content is English.
- Code comments explain intent, invariants, security boundaries, and non-obvious logic.
- Every meaningful feature has tests and documentation.
- Git commits use Conventional Commits.
- README, roadmap, acceptance matrix, and project state remain synchronized.

## 8. Technology and Deployment Decisions

| Area | Baseline |
| --- | --- |
| Frontend | Next.js and TypeScript |
| UI styling | Tailwind CSS |
| Backend | Go |
| Host agent | Native Go binary with OS-specific collectors |
| Agent protocol | gRPC and Protocol Buffers |
| Browser live updates | WebSocket |
| Primary database | PostgreSQL |
| Live cache / ephemeral state | Redis |
| Local service orchestration | Docker Compose |
| Visualization | Custom Canvas/WebGL rendering where benchmarks justify it |
| Final system design | Excalidraw |

### Docker Usage

Docker is part of the platform's development and deployment toolchain:

- Docker Compose runs the local control-plane stack: backend, frontend, PostgreSQL, and Redis.
- Docker may be used for supported single-node deployments.
- Docker metadata can be collected for container discovery and project attribution when explicitly enabled and authorized.
- The monitored host agent is a native Go service/binary; it is not required to run inside a container.
- A monitored Linux, Windows, or macOS machine does not need Docker unless the operator wants container monitoring or the chosen deployment mode requires it.

### Storage Responsibilities

- **PostgreSQL:** durable application state, server/project metadata, alert state, and historical telemetry.
- **Redis:** ephemeral live cache, short-lived coordination, and transient fan-out state.
- **Agent persistent spool/WAL:** bounded local durability for telemetry not yet acknowledged by the backend.
- A separate custom time-series database is not part of the initial MVP; it may only be reconsidered after measured PostgreSQL limits justify it.

## 9. Measurement Semantics and Important Constraints

- CPU utilization is calculated from counter deltas over a measured elapsed interval.
- Disk and network rates are calculated from cumulative counter deltas divided by elapsed monotonic time.
- Memory fields must distinguish host-wide used/available/cache values from per-process RSS. Summing process RSS can double-count shared pages.
- IOPS means completed read/write operations per second, based on supported device counters.
- Port and socket discovery does not reveal HTTP request latency or HTTP response status codes by itself.
- Linux cgroups provide stronger workload attribution than inferred process trees.
- Linux eBPF, Windows tracing facilities, and macOS instrumentation expose different capabilities and privileges.
- Sampling timestamps, collection timestamps, ingestion timestamps, and browser receipt timestamps must not be conflated.

## 10. Out of Scope for the Initial MVP

- Guaranteed 1ms end-to-end latency.
- Guaranteed zero crashes or zero data loss under every possible failure.
- One million concurrently monitored hosts without scale testing.
- An unrestricted remote shell.
- Exact project attribution where the OS does not expose an enforceable workload boundary.
- HTTP request-level metrics from port discovery alone.
- A custom database engine before the need is demonstrated by benchmarks.

## 11. Phase 1 Exit Criteria

Phase 1 is complete when:

- The product scope and MVP boundary are documented.
- Functional requirements have stable IDs and acceptance criteria.
- Metric names, units, sources, and limitations are defined.
- The five-phase delivery plan is agreed and reflected in the roadmap.
- The proposed system boundaries and data flow are documented.
- Docker, PostgreSQL, Redis, Go, Next.js, gRPC, Protocol Buffers, and WebSocket responsibilities are explicit.
- README and project state identify the true implementation status.
- All Phase 1 documentation is committed to the repository.

## 12. Delivery Status Audit

The document above remains the approved requirements baseline. The delivery phase is tracked separately from the product's total release readiness.

As of the Phase 1–3 audit on 2026-11-22:

- Phases 1, 2, and 3 have passed their documented phase completion gates on the main branch.
- The overall first usable MVP is **not yet complete**. P0 requirements for the authenticated dashboard, server overview, Add Server user interface, and live dashboard presentation remain unfinished and are tracked for Phase 4.
- The Linux telemetry data path is implemented and verified, but a verified backend API is not equivalent to a completed operator-facing workflow.
- Product capabilities marked In Progress or Planned in the feature acceptance matrix must not be represented as shipped functionality.
