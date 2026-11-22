# Zero-to-Production Roadmap

This roadmap turns the product vision into independently deliverable phases. Each phase has an explicit boundary and a completion gate. The project must remain buildable and resumable after every completed phase.

## Delivery Status

- [x] Phase 0 — Repository foundation
- [ ] Phase 1 — Product requirements and measurable acceptance criteria
- [ ] Phase 2 — Architecture and security decisions
- [ ] Phase 3 — Backend and frontend foundations
- [ ] Phase 4 — Persistent data model and storage
- [ ] Phase 5 — Agent identity, enrollment, and communication protocol
- [ ] Phase 6 — Linux host agent and core telemetry
- [ ] Phase 7 — Reliable ingestion and live data distribution
- [ ] Phase 8 — Frontend application, authentication, and server inventory
- [ ] Phase 9 — Real-time dashboard and custom visualization
- [ ] Phase 10 — Project-level resource attribution and switching
- [ ] Phase 11 — Process, port, connection, and application telemetry
- [ ] Phase 12 — Alert rules, event processing, and notifications
- [ ] Phase 13 — Secure remote actions
- [ ] Phase 14 — Windows and macOS agents
- [ ] Phase 15 — One-click installers, upgrades, and uninstall
- [ ] Phase 16 — Security, integration, and end-to-end hardening
- [ ] Phase 17 — Performance, reliability, and scale validation
- [ ] Phase 18 — Production deployment and operations
- [ ] Phase 19 — Final documentation and as-built system design

---

## Phase 0 — Repository Foundation

**Status:** Completed

### Scope
- Initialize the private repository with project principles.
- Establish English-only documentation and code rules.
- Establish phase gates, commit standards, and continuation instructions.
- Record the zero-to-production roadmap and exact resume point.

### Completion gate
- README, repository instructions, roadmap, and project state exist.
- The project has one unambiguous next action.
- All initial documentation is written in English.

## Phase 1 — Product Requirements and Measurable Acceptance Criteria

**Status:** Not Started

### Scope
- Convert the initial product brief into precise functional requirements.
- Define the target users and primary workflows.
- Define an MVP boundary separately from later enterprise capabilities.
- Define the supported operating-system and feature-capability matrix.
- Define metric names, units, timestamps, sampling semantics, and accuracy expectations.
- Define server, project, agent, user, and organization concepts.
- Define dashboard, alert, remote-action, and installation acceptance criteria.
- Define scale goals as staged test targets, not unsupported guarantees.

### Deliverables
- `docs/PRODUCT_REQUIREMENTS.md`
- `docs/METRIC_CATALOG.md`
- A feature-to-test acceptance matrix.

### Completion gate
- Every MVP feature has an observable acceptance test.
- Cross-platform limitations and required privileges are documented.
- API latency and HTTP error-rate collection requirements are explicitly defined, including the need for application, proxy, or supported tracing instrumentation.
- Sampling interval and end-to-end latency are defined as separate measurements.
- The MVP and post-MVP scope are agreed.

## Phase 2 — Architecture and Security Decisions

**Status:** Not Started

### Scope
- Define system boundaries and deployment topology.
- Define the initial modular monolith/service boundaries; avoid premature microservices.
- Design agent enrollment, identity, transport security, and revocation.
- Design telemetry delivery, retry, idempotency, backpressure, and durable buffering.
- Design PostgreSQL partitioning, retention, and downsampling.
- Define Redis cache semantics and failure behavior.
- Define remote-action threat model and permission model.
- Record major choices as Architecture Decision Records (ADRs).

### Deliverables
- High-level component and data-flow diagrams.
- Initial threat model.
- ADRs for major architecture decisions.
- Deployment topology for local development and first production deployment.

### Completion gate
- Critical data flows and trust boundaries are documented.
- Failure modes and recovery behavior are documented.
- No critical architecture decision depends on an unverified performance claim.

## Phase 3 — Backend and Frontend Foundations

**Status:** Not Started

### Scope
- Establish a maintainable repository layout.
- Initialize the Go backend with configuration, structured logging, graceful shutdown, health, and readiness endpoints.
- Initialize the Next.js and TypeScript frontend with a minimal application shell.
- Add formatting, linting, unit-test, and CI workflows.
- Add configuration examples without secrets.
- Establish local developer workflows.

### Completion gate
- Backend and frontend build from a clean checkout.
- Health/readiness checks behave correctly.
- CI runs relevant formatting, lint, and test commands.
- No credentials are committed.

## Phase 4 — Persistent Data Model and Storage

**Status:** Not Started

### Scope
- Design PostgreSQL schemas for organizations, users, servers, agents, projects, metric definitions, metric samples, events, and audit records as required by the agreed MVP.
- Add versioned migrations and rollback guidance.
- Implement bounded connection pooling and batch-write paths.
- Define time partitioning, indexes, retention, and downsampling strategy.
- Define data consistency and idempotency keys.
- Add Redis integration for explicitly ephemeral live state and cache use cases.

### Completion gate
- Migrations work on a clean database.
- Repository tests cover reads, writes, duplicate ingestion, and failure paths.
- Cache loss does not destroy durable telemetry.
- Retention and partition behavior are tested.

## Phase 5 — Agent Identity, Enrollment, and Communication Protocol

**Status:** Not Started

### Scope
- Define versioned Protocol Buffer messages.
- Establish authenticated gRPC communication over TLS.
- Implement secure, expiring server enrollment.
- Bind agent identity to a registered server.
- Add heartbeats, capability reporting, protocol versioning, reconnect behavior, and credential revocation.
- Define server-side validation for all agent payloads.

### Completion gate
- An agent can enroll, authenticate, report health, reconnect, and be revoked in integration tests.
- Invalid or expired credentials are rejected.
- Protocol compatibility rules are documented.

## Phase 6 — Linux Host Agent and Core Telemetry

**Status:** Not Started

### Scope
- Build a small Go agent with explicit lifecycle and bounded resource use.
- Collect CPU utilization and per-core counters.
- Collect memory, swap, and relevant cache measurements.
- Collect disk throughput and IOPS from supported OS interfaces.
- Collect network throughput and packet counters.
- Collect host identity, uptime, and agent health.
- Implement monotonic counter-delta calculations and timestamping.
- Add a configurable sampling interval; validate resource impact before selecting aggressive defaults.

### Completion gate
- Metric calculations are tested against known fixtures.
- Units and semantics match the metric catalog.
- Collector errors are isolated and visible.
- Agent CPU and memory overhead are benchmarked on documented test hosts.
- Linux support limitations are documented.

## Phase 7 — Reliable Ingestion and Live Data Distribution

**Status:** Not Started

### Scope
- Validate and ingest telemetry batches.
- Implement bounded queues, backpressure, retry, and deduplication.
- Implement an agent-side persistent spool/WAL with size limits and recovery.
- Acknowledge batches only according to the agreed durability contract.
- Distribute live metric updates to authorized dashboard subscribers.
- Isolate slow consumers and prevent unbounded memory growth.

### Completion gate
- Network interruption and backend restart tests demonstrate recovery.
- Duplicate batches do not create duplicate logical samples.
- Queue limits and overload behavior are verified.
- Data-loss boundaries are documented and tested.
- Live updates are authorized and filtered by server/project scope.

## Phase 8 — Frontend Application, Authentication, and Server Inventory

**Status:** Not Started

### Scope
- Build the English-only application shell.
- Implement sign-in and session handling using the architecture selected in Phase 2.
- Implement organization and role-aware navigation.
- Add server inventory, server details, agent status, and onboarding entry points.
- Add loading, empty, error, and reconnecting states.

### Completion gate
- Core navigation works without page errors.
- Protected views enforce authorization.
- Server inventory reflects backend data.
- UI behavior is covered by relevant tests.

## Phase 9 — Real-Time Dashboard and Custom Visualization

**Status:** Not Started

### Scope
- Build the custom metric visualization layer using Canvas/WebGL where measurement justifies it.
- Implement live CPU, memory, disk, and network panels.
- Add time ranges, pause/resume, units, tooltips, and historical queries.
- Use bounded rendering windows and downsampled data for long ranges.
- Keep the interface understandable without requiring users to learn complex query languages.

### Completion gate
- Dashboard data matches backend fixtures.
- Rendering remains responsive under a documented sample volume.
- Long-range views do not request or render every raw sample.
- Reconnect and empty-data states behave correctly.

## Phase 10 — Project-Level Resource Attribution and Switching

**Status:** Not Started

### Scope
- Implement All Projects / Overall Server views.
- Implement project creation and project-to-workload mapping.
- Attribute resources using supported Linux cgroups, container metadata, and system service/process relationships.
- Add project-scoped metric queries and live subscriptions.
- Add instant project switching without a full page reload.
- Clearly represent unclassified and shared host resources.

### Completion gate
- Project resource totals reconcile with host-level measurements within documented measurement boundaries.
- Project switching updates every relevant panel and query scope.
- Shared and unclassified resources are not silently assigned to a project.
- Attribution behavior is tested with representative workloads.

## Phase 11 — Process, Port, Connection, and Application Telemetry

**Status:** Not Started

### Scope
- Add process inventory and per-process resource usage.
- Add listening-port and connection metadata where supported.
- Add optional eBPF-based Linux tracing behind capability checks.
- Define application instrumentation or proxy integration for request latency and HTTP status/error rates.
- Add application health checks and trace/metric correlation where in scope.

### Completion gate
- Platform capabilities and required privileges are visible.
- Unsupported metrics are reported as unavailable, not fabricated.
- API latency and HTTP error rates are validated against a controlled test application.
- Collection overhead and privacy implications are documented.

## Phase 12 — Alert Rules, Event Processing, and Notifications

**Status:** Not Started

### Scope
- Implement threshold and condition-based alert rules.
- Support scoped rules for servers and projects.
- Add alert state transitions, deduplication, grouping, cooldowns, and recovery events.
- Add notification channels selected during requirements and architecture phases.
- Add alert history and delivery status.

### Completion gate
- Alert evaluation is deterministic and tested.
- Duplicate events are controlled.
- Recovery and notification failure behavior are documented.
- Alert permissions and audit requirements are met.

## Phase 13 — Secure Remote Actions

**Status:** Not Started

### Scope
- Implement a small explicit command allowlist.
- Support approved process/project restart and stop workflows.
- Require authorization, least privilege, confirmation, timeouts, and audit records.
- Use an authenticated bidirectional control channel.
- Add safe handling for unavailable agents and partial failures.
- Do not expose arbitrary remote shell execution.

### Completion gate
- Unauthorized actions are rejected.
- Every attempted action is auditable.
- Timeouts and retries cannot accidentally repeat non-idempotent actions.
- Agent-side execution is restricted to approved operations.

## Phase 14 — Windows and macOS Agents

**Status:** Not Started

### Scope
- Implement OS-native collectors behind shared agent interfaces.
- Add process, CPU, memory, disk, and network metrics supported by each OS.
- Report platform-specific capability differences.
- Add OS-specific packaging and integration tests.
- Keep unsupported Linux-only features explicitly unavailable.

### Completion gate
- Supported collectors pass platform-specific tests.
- Installation and upgrade behavior is verified on supported OS versions.
- Cross-platform metrics have consistent documented semantics where possible.

## Phase 15 — One-Click Installers, Upgrades, and Uninstall

**Status:** Not Started

### Scope
- Generate short-lived, scoped enrollment commands from the dashboard.
- Download signed, versioned agent packages.
- Verify package signatures and checksums before installation.
- Install and configure services using OS-native mechanisms.
- Support upgrade, rollback, uninstall, and credential revocation.
- Provide clear privilege and firewall requirements.

### Completion gate
- Clean-machine install, upgrade, rollback, and uninstall tests pass for each supported platform.
- Invalid signatures and expired enrollment tokens are rejected.
- Installation logs do not expose secrets.
- Installation does not silently execute unverified artifacts.

## Phase 16 — Security, Integration, and End-to-End Hardening

**Status:** Not Started

### Scope
- Review authentication, authorization, tenant isolation, secrets, transport security, and audit controls.
- Run dependency and static security checks.
- Add integration and end-to-end coverage across agent, backend, database, cache, and frontend.
- Test malformed input, slow consumers, resource exhaustion, and service restarts.
- Document backup, restore, and incident procedures.

### Completion gate
- Critical security findings are resolved or explicitly accepted.
- Tenant and project isolation tests pass.
- Backup and restore procedures are exercised.
- End-to-end release checks are repeatable.

## Phase 17 — Performance, Reliability, and Scale Validation

**Status:** Not Started

### Scope
- Establish reproducible benchmarks for collection overhead, ingest throughput, processing latency, storage throughput, query latency, and dashboard rendering.
- Test progressively larger host counts and metric rates.
- Profile Go services and frontend rendering.
- Test backpressure, slow databases, Redis outages, packet loss, process crashes, and restarts.
- Tune worker pools, batching, indexes, partitions, retention, and cache behavior based on evidence.
- Define and publish measured service objectives.

### Completion gate
- Benchmark environment, workload, duration, versions, and results are recorded.
- Bottlenecks have evidence-based fixes.
- Resource limits and overload behavior are known.
- Scale claims are limited to demonstrated test results.

## Phase 18 — Production Deployment and Operations

**Status:** Not Started

### Scope
- Define production topology and supported deployment modes.
- Add deployment automation, configuration validation, secrets handling, TLS setup, migrations, and rollback.
- Document capacity planning, backups, restore, upgrades, and incident response.
- Add health/readiness checks and operational diagnostics.
- Validate a clean production-like deployment.

### Completion gate
- A new environment can be deployed using documented steps.
- Upgrade and rollback procedures are tested.
- Backup restoration is demonstrated.
- Operational ownership and known limitations are documented.

## Phase 19 — Final Documentation and As-Built System Design

**Status:** Not Started

### Scope
- Reconcile README feature status with the actual implementation and test evidence.
- Publish installation, configuration, API, troubleshooting, security, and operations documentation.
- Generate the final Excalidraw system design from the completed implementation.
- Document components, data flows, storage, protocols, security boundaries, deployment, failure recovery, and scaling.
- Perform a final repository and release review.

### Completion gate
- No feature is marked complete without implementation and verification evidence.
- The Excalidraw design matches the actual deployed architecture.
- README and release notes reflect the shipped product.
- Known limitations and unsupported capabilities are explicit.

---

## Cross-Phase Definition of Done

Every phase must satisfy all applicable items before it is marked complete:

- [ ] Scope and acceptance criteria are satisfied.
- [ ] Implementation is complete and integrated.
- [ ] Unit, integration, and relevant end-to-end tests pass.
- [ ] Formatting, linting, and static checks pass.
- [ ] Security and failure paths are reviewed.
- [ ] English documentation and code comments are updated.
- [ ] README and project state are consistent.
- [ ] Changes are committed with a descriptive Conventional Commit.
- [ ] The next exact action is recorded in `docs/PROJECT_STATE.md`.

A phase can be split into smaller tasks, but its status must remain honest. A partially implemented phase is not complete.
