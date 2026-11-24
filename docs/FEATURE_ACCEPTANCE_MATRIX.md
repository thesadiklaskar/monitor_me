# Feature Acceptance Matrix

This document is the test-oriented checklist for the product brief. The matrix is a plan until implementation and test evidence exist.

## Status Definitions

- **Planned:** Requirements are documented; implementation has not been verified.
- **In Progress:** Implementation is underway but one or more acceptance checks remain.
- **Verified:** Implementation and required tests passed for the documented environment.
- **Blocked:** Work cannot proceed until a named dependency or decision is resolved.

No item may be marked Verified based only on a code stub, mock UI, or documentation.

## Feature Matrix

| ID | Feature | Priority | Acceptance evidence | Current status |
| --- | --- | --- | --- | --- |
| F-001 | Add Node and one-use enrollment token | P0 | Add Machine creates a short-lived token without target-OS selection; dashboard offers PowerShell/POSIX launchers; signed downloadable installers, automatic hostname naming and end-to-end acceptance remain | In Progress |
| F-002 | Native Go host agent lifecycle | P0 | Native foreground process, validated configuration, JSON logs, signal shutdown, process CPU/collector/WAL health telemetry, and Linux systemd unit validation pass race-enabled CI checks | Verified |
| F-003 | Host CPU and per-core metrics | P0 | Linux /proc/stat counter fixtures, reset tests, and documented source semantics | Verified |
| F-004 | Host memory, swap, cache metrics | P0 | Unit and OS-specific semantics tests | Verified |
| F-005 | Disk throughput and IOPS | P0 | Linux /proc/diskstats sector conversion, read/write operation and byte-rate fixtures, and reset tests | Verified |
| F-006 | Network throughput and packets | P0 | Linux /proc/net/dev byte/packet/error counter fixtures, interface identity, and reset tests | Verified |
| F-007 | Authenticated gRPC ingestion | P0 | Integration tests for authentication, schema validation, and malformed payloads | Verified |
| F-008 | Durable local telemetry spool | P0 | Disconnect, process restart, recovery, quota, and acknowledgement tests | Verified |
| F-009 | PostgreSQL historical persistence | P0 | Migration, batch-write, duplicate-delivery, and query tests | Verified |
| F-010 | Redis live cache/state | P0 | Cache outage does not corrupt durable records; bounded reconnect tests | Verified |
| F-011 | Server inventory and agent status | P0 | Organization-scoped inventory UI, loading/empty/error states and backend-reported status/last-seen display implemented; stale/disconnected component tests remain | In Progress |
| F-012 | Overall server dashboard | P0 | Server-detail history/live metrics, time ranges, units, gaps and stale-data states are implemented; dashboard acceptance/component evidence remains | In Progress |
| F-013 | Authorized live WebSocket updates | P0 | Scope authorization, reconnect, slow-client, and bounded-buffer tests | Verified |
| F-014 | Docker Compose local stack | P0 | Clean-start, health-check, restart, and documented local setup tests | Verified |
| F-015 | All Projects / project switcher | P1 | Project scope selectors are present for server overview/detail and API queries; complete owner acceptance of every scoped panel remains | In Progress |
| F-016 | Docker and Systemd discovery | P1 | cgroup/Systemd/container selector fixtures; full Docker Engine API discovery and least-privilege runtime integration remain | In Progress |
| F-017 | cgroups-based project attribution | P1 | Database-backed mappings and mapping administration API/UI are implemented; end-to-end attribution acceptance and full Docker Engine discovery remain | In Progress |
| F-018 | Process and port monitoring | P1 | Top-128 process RSS and listening-port inventory verified; per-process CPU and lifecycle views remain | In Progress |
| F-019 | Connection monitoring | P1 | TCP state/count fixtures and platform capability tests | Verified |
| F-020 | Application request latency and HTTP errors | P1 | Controlled application/proxy test with known response distribution | Planned |
| F-021 | Historical query and retention | P1 | Bounded time-range/aggregation queries and 30-day raw plus 90-day idempotency retention tests; partitioning is deferred until scale data justifies it | Verified |
| F-022 | Downsampling | P1 | Query-time date_bin aggregation and raw-retention tests pass; persistent downsampled rollup tables have not been implemented | In Progress |
| F-023 | Custom live visualization | P1 | Rendering benchmark on reference hardware and representative point counts | Planned |
| F-024 | Alert evaluation and recovery | P1 | Alert rule/evaluation and recovery workflows plus dashboard views are implemented; current deterministic end-to-end verification remains | In Progress |
| F-025 | Notifications | P1 | Persisted in-app firing/recovery notifications and read-state UI are present; external provider delivery/retry is not implemented | In Progress |
| F-026 | Secure remote restart/stop | P1 | Backend and native agent support allowlisted systemd restart/stop with owner/admin authorization, expiry and audit; operator UI added; full agent end-to-end acceptance remains | In Progress |
| F-027 | Windows agent | P1 | Initial Windows collector for CPU, memory/swap, disk capacity/I/O, network throughput, uptime, process inventory/RSS and OS/virtualization metadata; Docker-based x64 build/run helper plus CI cross-compile step configured; owner runtime acceptance, Windows service lifecycle, signed installer and metric parity remain | In Progress |
| F-028 | macOS agent | P1 | OS-version-specific collector and installer test suite | Planned |
| F-029 | Signed installers and upgrades | P1 | Signature validation, install, upgrade, rollback, and uninstall tests | Planned |
| F-030 | Role-based access and tenant isolation | P1 | API, WebSocket, and cross-tenant denial tests | Verified |
| F-031 | Performance benchmark suite | P2 | Collector, WAL, live-event and TLS gRPC/PostgreSQL benchmarks run with documented runner/toolchain/workload; staged fleet-scale capacity reports remain | In Progress |
| F-032 | Failure and recovery testing | P2 | WAL reopen/torn-tail, unavailable-ACK, reconnect, Redis and PostgreSQL readiness/outage recovery are tested; broader API/agent crash and storage fault injection remains | In Progress |
| F-033 | Horizontal scale validation | P2 | Staged multi-agent load reports and identified capacity limits | Planned |
| F-034 | Production operations and backup | P1 | Production Compose/Nginx, readiness, PostgreSQL backup/restore and app-image rollback scripts are prepared; real VPS/restore/rollback exercise remains | In Progress |
| F-035 | Final as-built Excalidraw design | P1 | Diagram reviewed against actual source, data flows, and deployment | Planned |
| F-036 | Production Linux host firewall setup | P1 | Dry-run/additive UFW/firewalld helper supports control-plane allow rules and agent-only no-inbound mode with recorded rollback; default-policy convergence, IPv4/IPv6, Docker, SSH and external host tests remain | In Progress |


| F-037 | Single-install no-login operator mode | P0 | No signup/login/organization UI; implicit install workspace; local loopback access; remote no-login mode is blocked unless HTTPS and administrator network restrictions are verified | In Progress |
| F-038 | Automatic control-plane and node bootstrap | P0 | One control-plane command generates secrets/TLS, starts Compose, applies migrations/health checks and firewall policy; per-node command detects OS/architecture, installs signed native agent and service without Docker/source checkout | In Progress |
| F-039 | High-volume raw log ingestion and exploration | P0 | Linux journald/files and Windows Event Logs/files; ClickHouse durable batch ingestion, indexed time-bounded search, live tail, retention, redaction, back-pressure and crash/replay/load tests; currently not implemented | Planned |
| F-040 | Professional monitoring-console shell | P0 | Responsive operator console, collapsible icon sidebar, consistent icon system, global node/time selectors, accessible controls, clear status/data states and removal of placeholder UI | In Progress |

## Phase-Level Verification

### Phase 1 — Product Specification and Technical Blueprint

- [x] Product scope and MVP boundary documented.
- [x] Functional requirements assigned stable IDs.
- [x] Metrics and calculation semantics documented.
- [x] Feature acceptance matrix created.
- [x] Five-phase roadmap and high-level architecture documented.
- [x] README and project state updated to reflect documentation-only completion.

### Phase 2 — Platform Foundation and Secure Connectivity

- [x] Go API bootstrap, structured logging, health/readiness behavior, graceful shutdown, formatting, race-enabled unit tests, vet, and builds pass in CI.
- [x] Next.js application shell passes ESLint, TypeScript checks, and production build in CI.
- [x] Docker Compose starts the full local stack successfully.
- [x] PostgreSQL migrations apply to a clean database and the expected identity schema is present.
- [x] PostgreSQL connection pooling and Redis client configuration pass build and runtime checks.
- [x] Redis is non-authoritative: API readiness fails during Redis outage, PostgreSQL identity data remains durable, and readiness recovers when Redis returns.
- [x] Versioned Protobuf sources generate reproducibly in CI; TLS gRPC server enforces 1 MiB message bounds and 128 concurrent streams per connection.
- [x] First-owner registration, Argon2id password validation, bearer login/logout, and authenticated profile checks pass integration tests.
- [x] Role authorization rejects viewer server creation and enrollment-token revocation.
- [x] Organization isolation prevents one organization's owner from seeing another organization's server inventory.
- [x] Server creation issues a random 15-minute, single-use enrollment token; invalid, expired, consumed, and revoked tokens are rejected.
- [x] TLS gRPC enrollment creates a stable agent identity and a 30-day session with hash-only persistence.
- [x] Heartbeats update server/agent liveness and capability metadata.
- [x] A new gRPC connection can reconnect with the existing session; re-enrollment preserves agent identity and invalidates previous sessions.
- [x] Agent revocation rejects future heartbeat requests.
- [x] **Phase 2 closeout (historical):** the telemetry stream was deferred until Phase 3 implemented durable ingestion. The current stream accepts authenticated telemetry batches and acknowledges only after PostgreSQL commit.

**CI evidence:** [GitHub Actions run #45030320358](https://github.com/WorkRCS/monitor_me/actions/runs/45030320358) — all three jobs passed: Go backend, Next.js frontend, and full Docker Compose integration.

### Phase 3 — Monitoring Agent and Telemetry Data Engine

- [x] Native Linux agent builds, passes race-enabled tests/vet, runs natively, reports capabilities, emits process CPU and agent-health samples, and its Linux systemd unit passes systemd-analyze verify.
- [x] Aggregate/per-core CPU, memory/cache/swap, disk throughput/IOPS, network throughput/packets/errors, uptime, process RSS, TCP states, and listening ports pass fixture/source-semantics tests.
- [x] Monotonic counter calculations handle initial samples, resets, and elapsed-time changes without negative or fabricated rates.
- [x] Durable WAL survives reopen and partial-tail recovery; disk quota is bounded; unacknowledged, rejected, or mismatched batches remain queued.
- [x] TLS gRPC ingestion enforces 1 MiB transport and 4,096-sample batch bounds, schema/label validation, idempotency, and post-commit acknowledgements.
- [x] PostgreSQL history queries preserve sample count/min/max/weighted average, use exact epoch-aligned bucket bounds, and enforce bounded range/point limits.
- [x] 30-day raw sample retention and 90-day idempotency retention are verified against PostgreSQL.
- [x] WebSocket tickets are one-use, session-bound, invalidated on logout, isolated by organization, bounded for slow clients, and reconnectable.
- [x] Workload mappings verify Systemd project attribution while keeping host-total, unclassified, and shared resources separate.
- [x] Agent collection/WAL, live event preparation, and TLS gRPC/PostgreSQL ingestion benchmarks execute in CI.
- [x] All Phase 3 gates passed in [GitHub Actions run #45043161542](https://github.com/WorkRCS/monitor_me/actions/runs/45045054465); all four jobs succeeded, including canonical batch-ID validation and PostgreSQL outage/recovery.

**Phase 3 boundaries:** Linux is the only implemented collector platform. Full Windows/macOS support, complete Docker Engine API discovery, per-process CPU, eBPF, HTTP request latency/error instrumentation, persistent materialized rollups, and dashboard UI remain later-phase work.

### Phase 4 — Dashboard and Operational Features

**Status: In Progress — implementation began 2026-11-23.**

- [x] First-owner setup, login/logout and current-session UI connected to existing authenticated APIs through HttpOnly cookie-backed same-origin route handlers.
- [x] Organization-scoped server inventory, loading/empty/error states, Add Server, one-time token display/copy/expiry, and token rotation/revocation controls pass all four jobs in [GitHub Actions run #45188259431](https://github.com/WorkRCS/monitor_me/actions/runs/45188259431). Full one-command install UX and stale/disconnected component tests remain.
- [ ] Server and project views display correct scoped data.
- [ ] Custom charts pass representative rendering tests.
- [ ] Process, port, and application metrics show documented capabilities.
- [ ] Alerts and notifications pass deterministic tests.
- [ ] Remote actions enforce authorization, allowlists, and audit logging.
- [ ] One-click installation workflow is usable and secure.

### Phase 5 — Cross-Platform Production and Scale

- [ ] Windows and macOS supported collectors are validated.
- [ ] Installer lifecycle and upgrades pass platform tests.
- [ ] Security, recovery, and end-to-end tests pass.
- [ ] Reproducible performance and capacity reports are published.
- [ ] Production deployment, backup, and restore are tested.
- [ ] Production firewall setup is tested for control-plane and agent-only host roles, IPv4/IPv6, Docker port exposure, SSH preservation, and rollback.
- [ ] README and final Excalidraw design match the shipped implementation.

## Evidence Record

For each Verified row, record links or paths to the tests, benchmark reports, supported OS/build, and the commit containing the implementation. Keep this matrix synchronized with README and `docs/PROJECT_STATE.md`.
