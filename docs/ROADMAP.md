# Zero-to-Production Roadmap

Monitor Me is delivered in **five major phases**. Each phase contains smaller independently reviewable tasks and has a clear completion gate. The repository must remain understandable and resumable between sessions.

## Overall Phase Status

| Phase | Name | Status |
| --- | --- | --- |
| 1 | Product Specification and Technical Blueprint | **Completed** |
| 2 | Platform Foundation and Secure Connectivity | **Completed** |
| 3 | Monitoring Agent and Telemetry Data Engine | Not Started |
| 4 | Dashboard and Operational Features | Not Started |
| 5 | Cross-Platform Production and Scale | Not Started |

A feature can be implemented before its phase is complete, but its status must remain honest. "Completed" requires implementation and verification evidence, not just a specification.

---

## Phase 1 — Product Specification and Technical Blueprint

**Status: Completed**

### Scope delivered

- Defined product vision, primary users, core workflows, and product principles.
- Defined initial MVP and post-MVP/enterprise scope.
- Converted the original feature brief into stable functional requirement IDs and acceptance criteria.
- Defined host, process, disk, network, application, and agent-health metric semantics.
- Defined timestamps, counter-delta calculations, sampling constraints, and missing-data behavior.
- Defined the five-phase delivery plan.
- Documented the proposed component architecture, data flow, Docker boundaries, persistence responsibilities, and security boundaries.
- Updated README as the project landing page and current status summary.
- Established feature acceptance and resumable project-state records.

### Deliverables

- `docs/PRODUCT_REQUIREMENTS.md`
- `docs/METRIC_CATALOG.md`
- `docs/FEATURE_ACCEPTANCE_MATRIX.md`
- `docs/ARCHITECTURE_OVERVIEW.md`
- Updated `README.md`, `docs/ROADMAP.md`, and `docs/PROJECT_STATE.md`

### Completion gate

- [x] Product scope and MVP boundary are documented.
- [x] Functional requirements have stable IDs and acceptance criteria.
- [x] Metric names, units, sources, and limitations are documented.
- [x] Architecture and data flow are documented.
- [x] Go, Next.js, PostgreSQL, Redis, Docker Compose, gRPC, Protocol Buffers, and WebSocket responsibilities are explicit.
- [x] Docker control-plane use is distinguished from native host-agent installation.
- [x] README accurately describes the current implementation status.
- [x] Feature matrix and next phase are recorded.

## Phase 2 — Platform Foundation and Secure Connectivity

**Status: Completed**

### Scope delivered

1. Established repository boundaries for the Go control plane, versioned protocol, Next.js frontend, and native Go agent workspace.
2. Added formatting, linting, unit-test, race-test, static-analysis, build, and full-stack CI workflows.
3. Implemented the Go API with environment configuration, JSON structured logging, health/readiness checks, HTTP timeouts, and graceful shutdown.
4. Implemented the Next.js/TypeScript/Tailwind application shell.
5. Added Docker Compose orchestration for PostgreSQL, Redis, the API, migration runner, and frontend.
6. Added versioned PostgreSQL migrations, bounded connection pooling, and an embedded migration runner.
7. Added Redis connectivity with bounded pool/timeouts; PostgreSQL remains authoritative for durable data.
8. Generated the versioned Protocol Buffer contract and implemented a TLS-protected gRPC agent-control server with bounded message sizes and concurrent streams.
9. Implemented one-time first-owner bootstrap, Argon2id password hashing, opaque bearer sessions, logout, membership lookup, and role-based authorization.
10. Implemented organization-scoped server inventory, enrollment-token issuance/reissue/revocation, agent identity binding, heartbeat/capability updates, re-enrollment, reconnect, and agent revocation.

### Implementation and security evidence

- [x] First-owner bootstrap is serialized and can only succeed once per database.
- [x] Passwords are hashed with Argon2id; bearer session and agent credentials are stored as SHA-256 digests, not raw secrets.
- [x] Organization membership and role are resolved from the authenticated session for protected requests.
- [x] Viewer permissions are read-only; unauthorized server creation and token revocation are rejected.
- [x] Server inventory is scoped to the caller's organization; a second organization cannot see the first organization's server.
- [x] Enrollment tokens are random, hash-only at rest, expire after 15 minutes, and can be consumed once.
- [x] Invalid, expired, consumed, and revoked enrollment credentials are rejected.
- [x] TLS gRPC enrollment returns an opaque 30-day agent session; agent identity is stable across re-enrollment and previous sessions are invalidated.
- [x] Heartbeat updates liveness and bounded capability metadata; a new gRPC connection can resume with the persisted session credential.
- [x] Agent revocation invalidates existing sessions.
- [x] PostgreSQL migrations apply on a clean database; Redis availability, dependency-aware readiness, PostgreSQL durability during Redis outage, and readiness recovery are tested.
- [x] The gRPC telemetry stream is deliberately left UNIMPLEMENTED until Phase 3 provides durable, idempotent telemetry persistence. The server does not acknowledge data it has not stored.
- [x] A native agent workspace boundary is documented at agent/README.md; the working agent executable and collectors are Phase 3 deliverables.

### Completion gate

- [x] Backend and frontend build from a clean checkout in CI.
- [x] Go formatting, race-enabled unit tests, go vet, and binary builds pass.
- [x] Frontend ESLint, TypeScript typecheck, and production build pass.
- [x] Docker Compose starts the full local control-plane stack and the API becomes ready.
- [x] PostgreSQL schema migrations and Redis connectivity pass runtime checks.
- [x] Redis outage causes readiness to fail without making Redis the durable data store; PostgreSQL state remains intact and readiness recovers.
- [x] User authentication, session lifecycle, organization authorization, and tenant isolation pass integration tests.
- [x] TLS gRPC enrollment, heartbeat/capability reporting, reconnect, re-enrollment, and revocation pass integration tests.
- [x] Invalid, expired, consumed, and revoked enrollment credentials are rejected.
- [x] README, development guide, ADR, feature acceptance matrix, and project state record implemented behavior and Phase 3 as the next phase.

**CI evidence:** [GitHub Actions run #45030320358](https://github.com/WorkRCS/monitor_me/actions/runs/45030320358) — all three jobs succeeded: Go backend checks, Next.js frontend checks, and Docker Compose integration. The run generated Protobuf sources, built the Go services, started the full stack, exercised user authentication and TLS agent enrollment, verified migrations and Redis connectivity, and tested Redis outage/recovery with durable PostgreSQL state.

**Decision record:** ADR 0002 — User Authentication and Secure Agent Enrollment.

## Phase 3 — Monitoring Agent and Telemetry Data Engine

**Status: Not Started**

### Scope

1. Implement the native Go agent lifecycle, configuration, logging, and capability model.
2. Implement Linux CPU and per-core collection.
3. Implement Linux memory, swap, cache, and process RSS collection with correct semantics.
4. Implement disk throughput, read/write operations, network throughput, and packet counters.
5. Add uptime, agent health, collector errors, and process inventory.
6. Implement counter-delta calculations, monotonic elapsed-time measurement, and counter-reset handling.
7. Implement bounded in-memory queues and a persistent agent-side spool/WAL.
8. Implement authenticated, versioned telemetry ingestion with validation, idempotency, batching, and backpressure.
9. Persist telemetry in PostgreSQL and define retention/partition behavior.
10. Implement live data fan-out to authorized WebSocket subscribers.
11. Add Docker/Systemd discovery and cgroups-based project attribution where supported.
12. Add port/connection metadata and the foundations for supported application instrumentation or proxy integrations.
13. Add metric aggregation and downsampling after correctness tests.

### Completion gate

- [ ] Linux agent runs natively and reports its capabilities.
- [ ] Core metrics have unit/fixture tests and comparisons against documented OS sources.
- [ ] Rate calculations handle resets and elapsed-time changes without negative or fabricated values.
- [ ] Agent buffering survives expected network interruption and process restart scenarios.
- [ ] Ingestion has bounded queues, observable backpressure, idempotency, and tested overload behavior.
- [ ] Historical data is durable in PostgreSQL; Redis is not the only copy.
- [ ] Live delivery is authorized, bounded, and reconnectable.
- [ ] Project attribution exposes its method and does not hide shared/unclassified resources.
- [ ] Collection overhead and initial ingest performance are benchmarked.
- [ ] README and state reflect only verified telemetry capabilities; state points to Phase 4.

## Phase 4 — Dashboard and Operational Features

**Status: Not Started**

### Scope

1. Build server inventory, Add Server, server detail, and agent health pages.
2. Build the overall host dashboard.
3. Build the header-level All Projects / project selector and no-full-reload switching.
4. Build custom Canvas/WebGL visualizations where benchmarks justify their use.
5. Add time ranges, historical charts, query limits, and stale-data indicators.
6. Add process, port, connection, workload, and application metric views.
7. Add API latency and HTTP error dashboards through validated instrumentation/proxy integrations.
8. Add alert rules, deduplication, recovery events, and notification delivery.
9. Add authorized, allowlisted, audited restart/stop actions through the agent control channel.
10. Complete the guided one-command installation workflow and user-facing operational guidance.
11. Add user/role/project administration and audit views as required for the first production release.

### Completion gate

- [ ] Dashboard values, units, scopes, and timestamps match backend test fixtures.
- [ ] All Projects and individual project selection update all scoped panels and subscriptions without a full page reload.
- [ ] Overall host totals remain distinct from project-attributed totals.
- [ ] Graphs pass rendering tests on documented reference hardware and representative sample counts.
- [ ] Historical queries are bounded and use appropriate aggregation/downsampling.
- [ ] API metrics are validated against a controlled test application or proxy.
- [ ] Alerts pass deterministic transition, deduplication, and recovery tests.
- [ ] Notifications have observable delivery and retry state.
- [ ] Remote actions enforce authorization, allowlists, timeouts, and audit logging.
- [ ] Installation instructions are reproducible and do not expose credentials.
- [ ] README reflects implemented operator workflows; state points to Phase 5.

## Phase 5 — Cross-Platform Production and Scale

**Status: Not Started**

### Scope

1. Implement and test Windows native collectors.
2. Implement and test macOS native collectors.
3. Publish an OS/version/architecture capability matrix.
4. Add signed, versioned installers, upgrades, rollback, uninstall, and credential rotation.
5. Complete security review, tenant-isolation tests, dependency/static checks, and end-to-end hardening.
6. Test agent, backend, PostgreSQL, Redis, and network failures with controlled fault injection.
7. Add repeatable performance benchmarks for collection overhead, ingestion, storage, queries, alert delay, and browser rendering.
8. Perform staged multi-agent load tests and document demonstrated capacity limits.
9. Finalize retention, backups, restore, production deployment, and incident procedures.
10. Reconcile every README feature status with source code and test evidence.
11. Create the final Excalidraw as-built system design from the completed implementation and deployment configuration.
12. Prepare a release checklist and final production documentation.

### Completion gate

- [ ] Windows and macOS support is tested for each documented OS/architecture combination.
- [ ] Install, upgrade, rollback, and uninstall paths pass platform tests.
- [ ] Security and tenant-isolation tests pass with no unresolved critical findings.
- [ ] Recovery tests demonstrate documented behavior for supported failures.
- [ ] Benchmark reports include environment, versions, workload, duration, and results.
- [ ] Scale claims do not exceed demonstrated test capacity.
- [ ] Production deployment, backup, and restore are exercised.
- [ ] README feature statuses match verified shipped behavior.
- [ ] Final Excalidraw diagrams match actual code, data flow, security, and deployment.
- [ ] Final project state identifies the release commit and known limitations.

---

## Cross-Phase Definition of Done

A feature or phase is not complete until all applicable conditions are satisfied:

- [ ] Scope and acceptance criteria are met.
- [ ] Implementation is integrated; no placeholder is presented as finished functionality.
- [ ] Relevant unit, integration, and end-to-end tests pass.
- [ ] Formatting, linting, and static checks pass.
- [ ] Security and failure paths are reviewed where relevant.
- [ ] Comments explain intent, invariants, and non-obvious logic.
- [ ] Documentation and feature acceptance status are updated.
- [ ] README is updated to describe the verified feature and current phase status.
- [ ] Git changes are committed using a descriptive Conventional Commit.
- [ ] `docs/PROJECT_STATE.md` records the completed work, commit reference, blockers, and one exact next action.

## Resume Rule

Before making changes, read `README.md`, `AGENTS.md`, and `docs/PROJECT_STATE.md`. Continue from the exact next action in the project state. Never restart completed phases or mark a phase complete based solely on elapsed time or a partially implemented feature.
