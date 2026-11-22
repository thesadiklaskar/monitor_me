# Feature Acceptance Matrix

This document is the test-oriented checklist for the product brief. The matrix is a plan until implementation and test evidence exist.

## Status Definitions

- **Planned:** Requirements are documented; implementation has not been verified.
- **In Progress:** Implementation is underway but one or more acceptance checks remain.
- **Verified:** Implementation and required tests passed for the documented environment.
- **Blocked:** Work cannot proceed until a named dependency or decision is resolved.

No item may be marked Verified based only on a code stub, mock UI, or documentation.

## Feature Matrix

| ID | Feature | Priority | Acceptance evidence | Initial status |
| --- | --- | --- | --- | --- |
| F-001 | Add Server and scoped enrollment token | P0 | Valid enrollment succeeds; expired/revoked/invalid token fails | Planned |
| F-002 | Native Go host agent lifecycle | P0 | Foreground and service modes; health and graceful shutdown tests | Planned |
| F-003 | Host CPU and per-core metrics | P0 | Counter fixtures, reset tests, comparison with OS tools | Planned |
| F-004 | Host memory, swap, cache metrics | P0 | Unit and OS-specific semantics tests | Planned |
| F-005 | Disk throughput and IOPS | P0 | Counter-delta tests and known workload comparison | Planned |
| F-006 | Network throughput and packets | P0 | Counter-delta, interface lifecycle, and reset tests | Planned |
| F-007 | Authenticated gRPC ingestion | P0 | Integration tests for authentication, schema validation, and malformed payloads | Planned |
| F-008 | Durable local telemetry spool | P0 | Disconnect, process restart, recovery, quota, and acknowledgement tests | Planned |
| F-009 | PostgreSQL historical persistence | P0 | Migration, batch-write, duplicate-delivery, and query tests | Planned |
| F-010 | Redis live cache/state | P0 | Cache outage does not corrupt durable records; bounded reconnect tests | Planned |
| F-011 | Server inventory and agent status | P0 | API and UI tests for online, stale, and disconnected states | Planned |
| F-012 | Overall server dashboard | P0 | Metric correctness, timestamp, units, and empty-state UI tests | Planned |
| F-013 | Authorized live WebSocket updates | P0 | Scope authorization, reconnect, slow-client, and bounded-buffer tests | Planned |
| F-014 | Docker Compose local stack | P0 | Clean-start, health-check, restart, and documented local setup tests | Planned |
| F-015 | All Projects / project switcher | P1 | No-reload switch; all panels and subscriptions change scope | Planned |
| F-016 | Docker and Systemd discovery | P1 | Discovery fixtures and least-privilege integration tests | Planned |
| F-017 | cgroups-based project attribution | P1 | Known workload resource comparison and attribution labels | Planned |
| F-018 | Process and port monitoring | P1 | Process lifecycle, listening ports, and permission tests | Planned |
| F-019 | Connection monitoring | P1 | TCP state/count fixtures and platform capability tests | Planned |
| F-020 | Application request latency and HTTP errors | P1 | Controlled application/proxy test with known response distribution | Planned |
| F-021 | Historical query and retention | P1 | Time-range, partition, retention, and bounded-query tests | Planned |
| F-022 | Downsampling | P1 | Aggregation correctness and raw-retention preservation tests | Planned |
| F-023 | Custom live visualization | P1 | Rendering benchmark on reference hardware and representative point counts | Planned |
| F-024 | Alert evaluation and recovery | P1 | Deterministic rule tests, deduplication, recovery, and delay measurement | Planned |
| F-025 | Notifications | P1 | Provider success/failure, retry, and delivery-state tests | Planned |
| F-026 | Secure remote restart/stop | P1 | Authorization, allowlist, audit, timeout, and replay tests | Planned |
| F-027 | Windows agent | P1 | OS-version-specific collector and installer test suite | Planned |
| F-028 | macOS agent | P1 | OS-version-specific collector and installer test suite | Planned |
| F-029 | Signed installers and upgrades | P1 | Signature validation, install, upgrade, rollback, and uninstall tests | Planned |
| F-030 | Role-based access and tenant isolation | P1 | API, WebSocket, and cross-tenant denial tests | Planned |
| F-031 | Performance benchmark suite | P2 | Reproducible reports with environment and workload metadata | Planned |
| F-032 | Failure and recovery testing | P2 | Network, database, cache, agent, and backend fault-injection evidence | Planned |
| F-033 | Horizontal scale validation | P2 | Staged multi-agent load reports and identified capacity limits | Planned |
| F-034 | Production operations and backup | P1 | Clean deployment, backup, restore, and rollback exercise | Planned |
| F-035 | Final as-built Excalidraw design | P1 | Diagram reviewed against actual source, data flows, and deployment | Planned |

## Phase-Level Verification

### Phase 1 — Product Specification and Technical Blueprint

- [x] Product scope and MVP boundary documented.
- [x] Functional requirements assigned stable IDs.
- [x] Metrics and calculation semantics documented.
- [x] Feature acceptance matrix created.
- [x] Five-phase roadmap and high-level architecture documented.
- [x] README and project state updated to reflect documentation-only completion.

### Phase 2 — Platform Foundation and Secure Connectivity

- [x] Go API bootstrap, health endpoint tests, formatting, race-enabled unit tests, vet, and builds pass in GitHub Actions.
- [x] Next.js application shell passes ESLint, TypeScript checks, and production build in GitHub Actions.
- [x] Docker Compose starts the full local stack successfully in CI.
- [x] Initial PostgreSQL identity/inventory migration applies on a clean database; CI verifies the resulting `organizations` table.
- [x] PostgreSQL application client and bounded connection pool pass build and startup integration checks.
- [x] Redis application client and connectivity checks pass in the full Compose integration job.
- [x] Dependency-aware readiness reports PostgreSQL and Redis status; liveness remains independent.
- [x] Versioned agent Protocol Buffer contract is drafted and documented.
- [ ] Application repositories and explicit cache-outage behavior tests are implemented.
- [ ] Authentication, authorization, expiring enrollment tokens, heartbeat, reconnect, and revocation pass integration tests.
- [ ] Generated Protocol Buffer code and a running authenticated gRPC service pass integration tests.

**CI evidence:** [GitHub Actions run #45023682627](https://github.com/WorkRCS/monitor_me/actions/runs/45023682627) — Go backend checks, frontend checks, and full Docker Compose integration all passed.

### Phase 3 — Monitoring Agent and Telemetry Data Engine

- [ ] Linux agent collects verified core host metrics.
- [ ] Persistent spool survives expected network and process interruptions.
- [ ] Ingestion is bounded, idempotent, and observable.
- [ ] Metrics are durably persisted and queryable.
- [ ] Live update path and project/workload attribution pass tests.

### Phase 4 — Dashboard and Operational Features

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
- [ ] README and final Excalidraw design match the shipped implementation.

## Evidence Record

For each Verified row, record links or paths to the tests, benchmark reports, supported OS/build, and the commit containing the implementation. Keep this matrix synchronized with README and `docs/PROJECT_STATE.md`.
