# Project State

This file is the authoritative resume point for Monitor Me. Read it before making changes and update it after every work session.

## Current Snapshot

| Field | Value |
| --- | --- |
| Project | Monitor Me |
| Repository | WorkRCS/monitor_me |
| Default branch | main |
| Working branch | main |
| Branch policy | All work and commits are made directly on main; do not create another branch |
| Repository visibility | Private |
| Completed phases | Phase 1 — Product Specification and Technical Blueprint; Phase 2 — Platform Foundation and Secure Connectivity |
| Current phase | Phase 3 — Monitoring Agent and Telemetry Data Engine |
| Current phase status | Not Started |
| Implementation status | Phase 2 control-plane foundation, authentication/authorization, TLS gRPC enrollment/heartbeat, Docker Compose runtime, PostgreSQL migrations, Redis outage/recovery checks, and frontend shell are CI-verified. Native agent executable, host collectors, telemetry spool/ingestion, and live delivery are not implemented yet. |
| Latest implementation commit | 1ad3e70d6e3a6174da2ebae6b43fdf117117f465 |
| Latest verified CI run | [#45030320358 — success](https://github.com/WorkRCS/monitor_me/actions/runs/45030320358) |
| Next action | Initialize the native Go agent module in agent/: add go.mod, a cmd/monitor-agent entry point, validated configuration, structured logging, graceful shutdown, and focused unit tests; wire its build/test checks into CI. |

## Completed Phase 1 — Product Specification and Technical Blueprint

**Status: Completed**

- [x] Product requirements and MVP boundary.
- [x] Functional requirements with stable IDs and acceptance criteria.
- [x] Metric catalog with units, sources, calculations, timestamps, and platform limitations.
- [x] Feature acceptance matrix.
- [x] Architecture overview and data flow.
- [x] Five-phase delivery roadmap.
- [x] Professional README and continuous maintenance rules.

Key Phase 1 commits:

- 5c4bf9b3684107bfb3884ebf26181013ed42ea4b — Product requirements.
- 4f982c881823b9cdea6250432f96cdad407af94f — Metric catalog.
- 5bd649196345de671d58fa3d01a3187e6f689608 — Feature acceptance matrix.
- 6d4719e81ab032c9f35f9631275212f78aceef7d — Architecture overview.

## Completed Phase 2 — Platform Foundation and Secure Connectivity

**Status: Completed**

### Delivered implementation

- [x] Repository boundaries for backend, frontend, protocol, deployment, and native agent workspace.
- [x] Go API with environment configuration, JSON structured logging, bounded HTTP timeouts, health/readiness endpoints, and graceful shutdown.
- [x] Next.js, TypeScript, React, and Tailwind application shell.
- [x] Docker Compose services for PostgreSQL, Redis, migration runner, API, and frontend.
- [x] PostgreSQL connection pool and embedded versioned migration runner.
- [x] Redis client with bounded pool settings and explicit timeouts.
- [x] Dependency-aware readiness; liveness remains independent of PostgreSQL/Redis availability.
- [x] First-owner bootstrap, Argon2id password hashing, opaque bearer sessions, login/logout, and authenticated profile.
- [x] Role-based API authorization and organization-scoped server inventory.
- [x] Server creation, one-time enrollment-token issuance/reissue/revocation, and agent revocation.
- [x] Generated versioned Protobuf contract and TLS-protected gRPC server.
- [x] Hash-only agent credentials, 15-minute single-use enrollment tokens, stable agent identity, 30-day agent sessions, heartbeat, capability reporting, reconnect, and re-enrollment/session rotation.
- [x] Bounded gRPC messages (1 MiB) and concurrent streams (128 per connection).
- [x] Telemetry stream explicitly deferred until Phase 3 durable ingestion is implemented.

### Verification evidence

GitHub Actions run [#45030320358](https://github.com/WorkRCS/monitor_me/actions/runs/45030320358) completed successfully:

- [x] Go dependency resolution and verification, Protobuf generation, gofmt, race-enabled tests, go vet, and backend builds.
- [x] Frontend dependency install, ESLint, TypeScript check, and production build.
- [x] Docker Compose configuration validation and full stack startup.
- [x] PostgreSQL migrations and Redis connectivity checks.
- [x] Owner registration/login/logout and role/organization authorization integration tests.
- [x] TLS gRPC enrollment, heartbeat, reconnect, re-enrollment, credential expiry/rejection, and agent revocation.
- [x] PostgreSQL durability and readiness recovery during Redis outage.
- [x] Full stack cleanup after CI.

### Phase 2 boundaries

- The frontend is still a foundation shell. Authentication screens, project switching, dashboards, graphs, alerts, and operational workflows are later-phase work.
- No native agent executable or collectors are shipped yet. The agent workspace boundary is reserved for Phase 3.
- The gRPC telemetry stream intentionally returns UNIMPLEMENTED until Phase 3 can validate, durably persist, and idempotently acknowledge telemetry.
- The Compose deployment is a local development topology, not a production deployment design.

### Key Phase 2 commits

- 1ce85a0b91cf0cce5fa1a29b94967a70c08f87d0 — Authentication, identity, TLS gRPC enrollment, and integration tests.
- 5873f5310b7d7e91a74ef4bcd68ec8bbd12cf142 — Go formatting.
- 17fd0f00c6a094febe768693d9b11101987d806c — Integration/server formatting.
- 9d75c7bcc2027fbd2ee7220ff8e06a8c5a3086a9 — Final integration-test formatting.
- 6b250d32b9e87d3fa8a909594df2284cc0738c8f — Tenant isolation and Redis outage/recovery tests.
- 3afe7bd43715ea7b6bc0c797e2c311f574798d83 — Health endpoint HEAD support and test correction.
- f93f2be2a72abae3056c9610834fad577c4a2d14 — Resolve Protobuf integration-test dependencies in CI.
- 8ba37d96ce10bd5781a02df20f28f0453e6a0be1 — Correct integration TLS certificate path.
- 3f67f317676227f287096c28388acbf90541cf4f — Explicit gRPC reconnect integration test.
- 1ad3e70d6e3a6174da2ebae6b43fdf117117f465 — Bound concurrent gRPC streams and document native agent workspace.
- 694ffccdf8676b03fe9212366c3abaa17b2de985 — Format the reconnect heartbeat request.

## Current Phase 3 — Monitoring Agent and Telemetry Data Engine

**Status: Not Started**

### Phase 3 scope

- [ ] Native Go agent module, command lifecycle, configuration, structured logs, and capabilities.
- [ ] Secure enrollment client using the Phase 2 TLS gRPC contract; persist/reload session credentials safely.
- [ ] Linux CPU and per-core collection.
- [ ] Linux memory, cache, swap, and process RSS collection with documented semantics.
- [ ] Disk read/write throughput and IOPS; network throughput and packet counters.
- [ ] Agent health, uptime, collector errors, and process inventory.
- [ ] Counter-delta calculations with monotonic elapsed time and reset handling.
- [ ] Bounded queue and persistent local spool/WAL.
- [ ] Authenticated, versioned, idempotent telemetry ingestion with validation, batching, and backpressure.
- [ ] Durable PostgreSQL telemetry storage and retention/partition strategy.
- [ ] Authorized WebSocket live fan-out and reconnect behavior.
- [ ] Docker/Systemd discovery, cgroups attribution, port/connection metadata, and application instrumentation foundations.
- [ ] Aggregation/downsampling and reproducible collection/ingestion benchmarks.

### Exact resume instructions

Continue directly on main. Do not create a new branch.

**Next implementation slice:** initialize the native Go agent module in agent/.

1. Add a standalone Go module and the cmd/monitor-agent entry point.
2. Add validated environment/config-file loading without logging secrets.
3. Add JSON structured logging and graceful shutdown.
4. Define an explicit agent capability/version structure and unit tests.
5. Add CI steps for go test -race ./..., go vet ./..., and agent binary build.
6. Update README, feature acceptance matrix, and this state file when the slice is verified.
7. Only after this foundation, implement the TLS enrollment client and Linux CPU/memory collector slices.

Do not implement unbounded queues, acknowledge telemetry before durable persistence, or claim metric accuracy/performance without source-based tests and benchmarks.

## Repository Rules

- English only in source code, comments, documentation, UI strings, logs, tests, and commit messages.
- All work must be committed directly on main; do not create a branch or PR unless the user explicitly changes this instruction.
- Build the monitoring product itself; do not embed Grafana, Prometheus, Zabbix, Netdata, or another ready-made monitoring platform.
- Go, Next.js/TypeScript, PostgreSQL, Redis, Docker Compose, gRPC, Protocol Buffers, and WebSocket are allowed technologies.
- The host monitoring agent will run as a native Go service; Docker is for control-plane development/deployment and optional container discovery.
- Comments explain intent, invariants, security boundaries, and non-obvious logic; do not comment every trivial line mechanically.
- Use descriptive Conventional Commits.
- Update README whenever a feature is verified and at each phase completion.
- Never mark a phase complete based on a partial implementation or a green build alone.

## Session History

Phase 1 is recorded above. Phase 2 implementation and verification evidence is listed above. Phase 2 closed after all three jobs in [GitHub Actions run #45030320358](https://github.com/WorkRCS/monitor_me/actions/runs/45030320358) succeeded.
