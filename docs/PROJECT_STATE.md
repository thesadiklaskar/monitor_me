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
| Completed phases | Phase 1 — Product Specification and Technical Blueprint; Phase 2 — Platform Foundation and Secure Connectivity; Phase 3 — Monitoring Agent and Telemetry Data Engine |
| Current phase | Phase 4 — Dashboard and Operational Features |
| Current phase status | Not Started |
| MVP status | Not complete — authenticated dashboard, server overview, Add Server UI, and live metric views remain Phase 4 requirements |
| Implementation status | Phases 1–3 phase gates are complete and CI-verified. The Linux agent, durable WAL, TLS gRPC ingestion, PostgreSQL history/retention, authorized live delivery, workload attribution, and benchmarks are verified. The overall MVP is not complete because dashboard/login/Add Server and browser-facing views remain Phase 4 work. Several non-UI feature requirements remain In Progress/Planned in the acceptance matrix. |
| Last source commit covered by latest successful CI run | af3bab3ec5f273bbbf3006d4efa122db7cccdb5c |
| Latest verified CI run | [#45043161542 — success](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542); all four jobs passed, including PostgreSQL outage/recovery canary |
| Next action | On 2026-11-23, start Phase 4 from docs/PHASE_4_EXECUTION_PLAN.md: implement the authenticated dashboard, server inventory/Add Server flow, real Linux metric views, live/history charts, and remaining Phase 4 operator workflows; verify each slice before changing status. |

## Next Work Session — 2026-11-23

**Target:** Phase 4 — Dashboard and Operational Features. No Phase 4 implementation is claimed yet; its current status remains Not Started.

### Ordered release plan

1. Complete and verify Phase 4 Monitoring Dashboard and operator workflows.
2. Complete Phase 5 firewall setup for the Linux control-plane and agent-only host roles, following docs/PRODUCTION_FIREWALL.md.
3. Complete the controlled production deployment/shift, including TLS, secrets, private PostgreSQL/Redis, migrations, backups/restore, health checks, and rollback.
4. After Phase 4 and Phase 5, the project owner performs the first local end-to-end acceptance test before wider rollout or additional expansion work.

Phase 5 has two primary release deliverables for this Linux-first MVP: **(a) firewall setup** and **(b) production shift**. Windows/macOS support and fleet-scale work remain separate broader roadmap items; the Linux production cutover must not be represented as completing them.

### Phase 4 resume point

Read docs/PHASE_4_EXECUTION_PLAN.md. The current frontend is still a foundation preview. Reuse the verified auth, server inventory, historical metric, and live-ticket APIs. Known gaps include project mapping administration, alert/notification workflows, and remote-action execution; implement and test these where required instead of presenting mock data.

### Session boundary

The next session is Phase 4 only. Do not start Phase 5 firewall work or production deployment until Phase 4 acceptance gates pass. Mark Phase 4 Completed only with full test and documentation evidence.

## Phase 1–3 Audit

The final Phase 1–3 audit passed all four jobs in [GitHub Actions run #45043161542 — success](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542). This run includes strict lowercase batch-ID validation, telemetry integration, and a PostgreSQL outage/recovery canary. The audit also identified individual product requirements that remain In Progress or Planned; see docs/PHASE_1_TO_3_AUDIT.md.

### Explicit unfinished product requirements

- Authenticated dashboard, server inventory UI, Add Server wizard and live metric views: Phase 4.
- OS service registration and remaining AgentHealth telemetry samples: In Progress.
- Workload mapping administration, full Docker Engine API discovery, process lifecycle/per-process CPU and persistent downsampled rollups: In Progress/Planned.
- Role-aware Linux firewall setup and controlled production deployment/shift: Phase 5 primary Linux MVP release workstreams. Windows/macOS agents, signed installers, staged fleet-scale testing, and broader production operations remain Phase 5 expansion/hardening items.
- Protobuf event/health/command messages are reserved; only authenticated telemetry batches are accepted by the current stream.

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

### Phase 2 boundaries at phase close (historical)

At the moment Phase 2 was closed, the frontend was a foundation shell and the native agent/telemetry stream had not yet been implemented. Phase 3 has since added the Linux agent, durable telemetry path, history, retention, and live delivery. The Compose deployment remains a local development topology, not a production deployment design.

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
- 694ffccdf8676b03fe9212366c3abaa17b2de985 — Format reconnect heartbeat request; full three-job CI passed.
- 694ffccdf8676b03fe9212366c3abaa17b2de985 — Format the reconnect heartbeat request.

## Completed Phase 3 — Monitoring Agent and Telemetry Data Engine

**Status: Completed — Phase 3 acceptance gate verified. The overall P0 MVP remains incomplete until Phase 4 dashboard workflows are delivered.**

**Verification evidence:** [GitHub Actions run #45043161542 — success](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542). All four jobs passed: Go backend, native Linux agent, Next.js frontend, and Docker Compose integration. The Compose job verified migrations, authenticated TLS enrollment, telemetry/history/retention, Redis outage/recovery, and PostgreSQL canary durability across service restart.

### Delivered implementation

- [x] Native Linux Go agent with validated configuration, JSON logging, graceful shutdown, and explicit capability reporting.
- [x] Linux procfs aggregate/per-core CPU, memory/cache/swap, disk throughput/IOPS, network throughput/packets/errors, host/agent uptime, process count and top-128 process RSS.
- [x] TCP connection-state counts, listening-port inventory, cgroup selectors, Systemd unit and container cgroup-ID recognition.
- [x] Counter rates based on monotonic elapsed time; initial samples, resets, and invalid counter intervals never generate fabricated negative rates.
- [x] Private append-only JSONL WAL with fsync, bounded disk quota, truncated-tail recovery, and atomic acknowledgement compaction.
- [x] TLS gRPC enrollment, private persisted agent session, heartbeat, reconnect/backoff, and streaming telemetry batches.
- [x] Bounded authenticated ingestion, batch schema/label validation, idempotency hash, post-commit ACK, and negative/mismatched ACK retention tests.
- [x] PostgreSQL transactionally stores batches and samples; Redis is used for organization-scoped ephemeral live notifications only.
- [x] Authenticated historical range API with exact epoch-aligned `date_bin` buckets, 100ms minimum step, 30-day raw range, and maximum 5,000 points.
- [x] 30-day raw sample and 90-day idempotency retention; expired WebSocket tickets cleaned in bounded batches.
- [x] 30-second single-use live tickets bound to user sessions, logout invalidation, cross-organization isolation, 32-message bounded subscriber queue, slow-client disconnect, and reconnect.
- [x] Database attribution verifies mapped Systemd workload samples and keeps host-total, unclassified, and shared resource categories separate.
- [x] CI benchmarks execute Linux collection, counter-rate calculation, WAL append, live event preparation, and real TLS gRPC-to-PostgreSQL ingestion.
- [x] Redis and PostgreSQL dependency outage/readiness recovery are exercised in Compose CI. Persistent AgentHealth samples for CPU, collector errors, spool size and last successful send, and persistent downsampled rollups remain explicitly deferred.

### Benchmark snapshot (GitHub-hosted Ubuntu runner)

| Benchmark | Result |
| --- | ---: |
| Linux proc collector | 5.33 ms/op; 1.03 MB/op; 5,163 allocs/op |
| Counter rate calculation | 3.68 ns/op; 0 allocs/op |
| One-sample WAL append/fsync | 256 μs/op; 952 B/op; 12 allocs/op |
| Live event preparation (96 input samples) | 128 μs/op; 165.7 KB/op; 897 allocs/op |
| TLS gRPC to PostgreSQL (64 samples/batch) | 4.40 ms/batch; 14,533 samples/sec; 9.2 KB/op; 535 allocs/op |

These measurements are a single CI-runner snapshot, not production capacity guarantees or evidence of million-host scale.

### Phase 3 boundaries

- Implemented host collector platform: Linux. Windows and macOS are Phase 5 work.
- Full Docker Engine API discovery is not implemented; container identification uses cgroup metadata exposed by the host.
- Per-process CPU and eBPF are not implemented.
- Listening ports and TCP state are not HTTP request latency or HTTP error telemetry. Those require application instrumentation, proxy integration, or tracing.
- Historical aggregation is performed on query with PostgreSQL `date_bin`; persistent materialized rollups and table partitioning remain later scale optimizations.
- Dashboard, project selector, alerting, notifications, installer UX, and remote operations remain Phase 4 work.

## Current Phase 4 — Dashboard and Operational Features

**Status: Not Started**

### Phase 4 scope

- [ ] Implement authenticated Next.js login/logout and server session handling.
- [ ] Build server inventory, empty/loading/error states, Add Server and one-time enrollment-token display.
- [ ] Build server detail and overall host dashboard using the verified history/live APIs.
- [ ] Add All Projects / project selector with no-full-reload data scope changes.
- [ ] Add Canvas/WebGL charts after rendering benchmarks.
- [ ] Add process/port/connection/workload views and supported application instrumentation integrations.
- [ ] Add alert rules, deduplication, recovery events and notifications.
- [ ] Add allowlisted, authorized and audited remote restart/stop operations through the agent control channel.

### Exact resume instructions

Continue directly on `main`. Do not create a new branch.

**Next implementation slice:** build the authenticated server inventory experience in the Next.js frontend.

1. Add a typed API client for login, current-user, logout and server inventory endpoints.
2. Implement login and session handling without exposing bearer tokens in logs or URLs.
3. Build the server inventory list with loading, empty, error and disconnected states.
4. Add the Add Server flow and show the one-time enrollment token exactly once with copy/install guidance.
5. Add component/API tests and keep all UI strings, comments and documentation in English.
6. Update README, feature acceptance matrix and this state file after the slice passes CI.

Do not start charts, alerts, project switching or remote actions until the authenticated server inventory and Add Server flow are accepted and tested.

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

Phase 1 and Phase 2 implementation/verification history is recorded above. The final Phase 1–3 audit passed all four jobs in [GitHub Actions run #45043161542](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542).
