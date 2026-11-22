# Zero-to-Production Roadmap

Monitor Me is delivered in **five major phases**. Each phase contains smaller independently reviewable tasks and has a clear completion gate. The repository must remain understandable and resumable between sessions.

## Overall Phase Status

| Phase | Name | Status |
| --- | --- | --- |
| 1 | Product Specification and Technical Blueprint | **Completed** |
| 2 | Platform Foundation and Secure Connectivity | **Completed** |
| 3 | Monitoring Agent and Telemetry Data Engine | **Completed** |
| 4 | Dashboard and Operational Features | Not Started |
| 5 | Cross-Platform Production and Scale | Not Started |

A feature can be implemented before its phase is complete, but its status must remain honest. "Completed" requires implementation and verification evidence, not just a specification.

## User-Directed Linux MVP Release Sequence

The agreed order is:

1. **Phase 4 — Monitoring Dashboard:** complete the dashboard and operator workflows, then pass the Phase 4 acceptance gate.
2. **Phase 5 — Firewall setup:** apply and verify the role-aware Linux firewall policy for the control plane and monitored hosts, including Docker/network exposure.
3. **Phase 5 — Production shift:** deploy the verified control plane and agent workflow to the selected production VPS/VM with TLS, private data services, backups/restore, health checks, and rollback.
4. **Owner-led local acceptance:** after Phase 4 and Phase 5, the project owner performs the first end-to-end local acceptance test before wider rollout or additional expansion work.

For the first Linux MVP release, Phase 5 has two primary release deliverables: firewall setup and production deployment/shift. Windows/macOS collectors and fleet-scale validation remain broader roadmap work and must not be claimed as complete by a Linux-only production cutover.

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
- [x] **Phase 2 closeout (historical):** the gRPC telemetry stream was deliberately left UNIMPLEMENTED because durable, idempotent ingestion belonged to Phase 3. Phase 3 has since implemented and verified telemetry-batch validation, PostgreSQL persistence, idempotency, and post-commit ACKs; see the Phase 3 completion gate below.
- [x] **Phase 2 closeout (historical):** the native agent workspace boundary was documented at agent/README.md. Phase 3 has since delivered the Linux Go agent executable and procfs collectors.

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
- [x] **Phase 2 closeout (historical):** README, development guide, ADR, feature acceptance matrix, and project state recorded Phase 3 as the next phase. They now record Phases 1–3 as complete and Phase 4 as not started.

**CI evidence:** [GitHub Actions run #45030320358](https://github.com/WorkRCS/monitor_me/actions/runs/45030320358) — all three jobs succeeded: Go backend checks, Next.js frontend checks, and Docker Compose integration. The run generated Protobuf sources, built the Go services, started the full stack, exercised user authentication and TLS agent enrollment, verified migrations and Redis connectivity, and tested Redis outage/recovery with durable PostgreSQL state.

**Decision record:** ADR 0002 — User Authentication and Secure Agent Enrollment.

## Phase 3 — Monitoring Agent and Telemetry Data Engine

**Status: Completed**

### Delivered scope

1. Native Go Linux agent lifecycle, validated configuration, structured logging, signal-aware shutdown, and capability reporting.
2. Linux aggregate/per-core CPU and documented memory, cache, swap, disk, network, uptime, process, TCP-state, and listening-port collection.
3. Counter-derived rates using monotonic elapsed time, with missing values on first samples, resets, and invalid counter intervals.
4. Bounded append-only agent JSONL WAL with fsync, disk quota, private permissions, trailing-record recovery, and atomic compaction after accepted ACKs.
5. TLS gRPC enrollment, persisted agent sessions, heartbeat, reconnect/backoff, and streaming batch delivery.
6. Authenticated ingestion with 1 MiB transport bounds, a 4,096-sample batch ceiling, 128 concurrent gRPC streams, a 16-batch agent delivery window, sequential durable ACKs, payload validation, idempotency hashes, and oversized-batch rejection. A separate in-memory ingestion worker queue is not used; sustained concurrent overload/capacity limits remain Phase 5 work.
7. Atomic PostgreSQL batch/sample persistence; ACKs are sent only after commit. Redis remains an ephemeral post-commit notification path.
8. Authenticated, organization/server-scoped historical range query using epoch-aligned PostgreSQL `date_bin` aggregation, a 100ms minimum step, 30-day raw range, and a 5,000-point cap.
9. Bounded retention: raw metric samples for 30 days, batch idempotency records for 90 days, and expired live tickets removed in bounded batches.
10. Session-bound, 30-second single-use WebSocket tickets, organization-scoped channels, 32-event subscriber queues, 128-point event caps, slow-client disconnect, reconnect, and history catch-up.
11. Linux cgroup/Systemd/container selector recognition and database-backed server/project workload mappings. Host-total, unclassified, and shared resources remain separately represented. User-facing mapping administration remains unfinished.
12. TCP connection-state/listening-port metadata and documented limits: port discovery does not infer application HTTP request latency or error rates.
13. On-query aggregation and reproducible collector, WAL, live-event, and TLS gRPC/PostgreSQL benchmarks. Persistent rollup tables and partitioning remain future scale optimizations.

### Completion gate

- [x] Linux agent runs natively and reports explicit capabilities.
- [x] Core Linux metrics have fixture/unit tests tied to documented procfs semantics.
- [x] Counter rates handle resets and actual elapsed time without negative or fabricated values.
- [x] Agent WAL survives process reopen and truncated-tail recovery; failed, negative, and mismatched ACKs do not remove queued batches.
- [x] Ingestion bounds message/sample sizes, validates payloads, rejects oversized batches, and uses batch-ID/payload-hash idempotency.
- [x] PostgreSQL persists telemetry durably and exposes bounded history; Redis is not the source of truth.
- [x] Live delivery is session-authorized, single-use-ticket protected, organization isolated, bounded, and reconnectable.
- [x] Project attribution exposes mapping method and preserves host-total, unclassified, and shared samples.
- [x] Collection, WAL, event preparation, and TLS gRPC/PostgreSQL ingestion are benchmarked in CI.
- [x] README, metric catalog, development guide, architecture overview, acceptance matrix, roadmap, and project state match the implementation.

**Verification evidence:** [GitHub Actions run #45043161542 — success](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542) verified all four jobs on `main`: Go backend, native Linux agent, Next.js frontend, and Docker Compose integration. The Compose job verified telemetry ingestion, history/retention, Redis outage/recovery, and a PostgreSQL durability canary across service restart.

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

### Primary Linux MVP release deliverables

**5A. Firewall setup** — implement and verify the role-aware UFW/firewalld policy documented in docs/PRODUCTION_FIREWALL.md. Control-plane public ingress must be limited to intended HTTPS/gRPC entry points and source-restricted SSH; agent-only monitored hosts need no inbound Monitor Me port. PostgreSQL, Redis, and internal application ports remain private. Preserve Docker networking, IPv4/IPv6 policy, and rollback capability.

**5B. Production shift** — deploy the verified Linux-first control plane and agent workflow to the selected VPS/VM. Configure DNS/TLS/reverse proxy and the gRPC endpoint, secure secrets, keep PostgreSQL/Redis private, apply migrations, exercise backup/restore, health/readiness, restart/recovery, and document cutover/rollback. Do not enable broad external onboarding until the owner completes the planned local acceptance test.

Windows/macOS support, signed cross-platform installers, and fleet-scale capacity work remain broader Phase 5 expansion/hardening requirements. A Linux-only cutover does not complete those items.

### Scope

1. Implement and test Windows native collectors.
2. Implement and test macOS native collectors.
3. Publish an OS/version/architecture capability matrix.
4. Add signed, versioned installers, upgrades, rollback, uninstall, and credential rotation.
5. Complete security review, tenant-isolation tests, dependency/static checks, and end-to-end hardening.
6. Implement and verify role-aware Linux host firewall setup for production VPS/VM and local/LAN deployments: supported UFW/firewalld profiles, least-privilege ingress/egress, IPv4/IPv6 parity, Docker-aware port exposure, idempotent changes, diagnostics, and rollback. Agent-only monitored hosts must not receive unnecessary inbound rules.
7. Test agent, backend, PostgreSQL, Redis, and network failures with controlled fault injection.
8. Add repeatable performance benchmarks for collection overhead, ingestion, storage, queries, alert delay, and browser rendering.
9. Perform staged multi-agent load tests and document demonstrated capacity limits.
10. Finalize retention, backups, restore, production deployment, and incident procedures.
11. Reconcile every README feature status with source code and test evidence.
12. Create the final Excalidraw as-built system design from the completed implementation and deployment configuration.
13. Prepare a release checklist and final production documentation.

### Completion gate

- [ ] Windows and macOS support is tested for each documented OS/architecture combination.
- [ ] Install, upgrade, rollback, and uninstall paths pass platform tests.
- [ ] Security and tenant-isolation tests pass with no unresolved critical findings.
- [ ] Recovery tests demonstrate documented behavior for supported failures.
- [ ] Benchmark reports include environment, versions, workload, duration, and results.
- [ ] Scale claims do not exceed demonstrated test capacity.
- [ ] Production deployment, backup, and restore are exercised.
- [ ] Firewall profiles are tested on supported Linux distributions; agent-only hosts require no inbound Monitor Me ports, control-plane exposure is least-privilege, and PostgreSQL/Redis/internal app ports remain private.
- [ ] Firewall changes are idempotent, preserve SSH access and Docker networking, cover intended IPv4/IPv6 paths, and have verification/rollback tests.
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
