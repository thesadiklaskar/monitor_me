# Project State

This file is the authoritative resume point for Monitor Me. Read it before making changes and update it after every work session.

## Current Snapshot

| Field | Value |
| --- | --- |
| Project | Monitor Me |
| Repository | `WorkRCS/monitor_me` |
| Default branch | `main` |
| Working branch | `phase-2/platform-foundation` |
| Repository visibility | Private |
| Completed phase | Phase 1 — Product Specification and Technical Blueprint |
| Current phase | Phase 2 — Platform Foundation and Secure Connectivity |
| Current phase status | In Progress |
| Implementation status | Go/Next.js foundations, PostgreSQL/Redis startup integration, automatic migrations, and full local Compose smoke checks are CI-verified; authentication and agent enrollment remain incomplete |
| Latest integration commit | `ab806951ed0e1cb2534d2b3a5ce1808aa3ab18ad` |
| Latest CI run | [#45023682627 — success](https://github.com/WorkRCS/monitor_me/actions/runs/45023682627) |
| Next action | Implement generated Protocol Buffer code and the authenticated gRPC enrollment service, then add token issuance/verification, agent identity binding, and integration tests. |

## Completed Phase 1 — Product Specification and Technical Blueprint

**Status: Completed**

- [x] Product requirements and MVP boundary.
- [x] Functional requirements with stable IDs and acceptance criteria.
- [x] Metric catalog with units, sources, calculations, timestamps, and platform limits.
- [x] Feature acceptance matrix.
- [x] Proposed architecture and data flow.
- [x] Five-phase delivery roadmap.
- [x] Professional README with continuous maintenance rules.

Key Phase 1 commits:

- `5c4bf9b3684107bfb3884ebf26181013ed42ea4b` — Product requirements.
- `4f982c881823b9cdea6250432f96cdad407af94f` — Metric catalog.
- `5bd649196345de671d58fa3d01a3187e6f689608` — Feature acceptance matrix.
- `6d4719e81ab032c9f35f9631275212f78aceef7d` — Architecture overview.

## Current Phase 2 — Platform Foundation and Secure Connectivity

**Status: In Progress**

### Verified platform foundation

- [x] Repository layout for backend, frontend, protocol, and deployment configuration.
- [x] Go API bootstrap using the standard library.
- [x] JSON structured logging, HTTP timeouts, and graceful shutdown.
- [x] Liveness endpoint independent of backend dependency availability.
- [x] Dependency-aware readiness endpoint with bounded checks and no secret leakage.
- [x] Unit tests for healthy dependencies, unavailable dependency response, and method restrictions.
- [x] Next.js, TypeScript, React, and Tailwind application shell.
- [x] Docker Compose services for PostgreSQL, Redis, API, migration runner, and web frontend.
- [x] PostgreSQL connection pool using pgx through `database/sql`.
- [x] Redis client with bounded pool settings and explicit timeouts.
- [x] Embedded versioned PostgreSQL migration runner as a one-shot executable.
- [x] Initial PostgreSQL identity/inventory schema applies successfully on a clean database.
- [x] Versioned Protocol Buffer agent contract draft.
- [x] Local development guide and GitHub Actions CI.

### CI verification evidence

GitHub Actions run [#45023682627](https://github.com/WorkRCS/monitor_me/actions/runs/45023682627) completed successfully:

- [x] Go module resolution and verification.
- [x] Go formatting check.
- [x] Go unit tests with race detection and coverage.
- [x] Go static analysis with `go vet`.
- [x] Go API and migration-runner builds.
- [x] Frontend dependency install, ESLint, TypeScript typecheck, and Next.js production build.
- [x] Docker Compose configuration validation.
- [x] Full Compose startup with PostgreSQL, Redis, migration service, API, and frontend image.
- [x] API readiness succeeds with live PostgreSQL and Redis.
- [x] CI verifies the migrated `organizations` table exists.
- [x] CI verifies Redis responds with `PONG`.
- [x] CI verifies dependency-aware readiness and cleans up the test stack.

### Still incomplete

- [ ] Generated Protocol Buffer code and version compatibility checks.
- [ ] Running authenticated gRPC server.
- [ ] Authentication and server-side authorization foundations.
- [ ] Secure enrollment-token issuance, hashing, expiry, one-time consumption, and revocation.
- [ ] Agent identity binding and heartbeat/reconnect lifecycle.
- [ ] Integration tests for enrollment rejection and revocation.
- [ ] Application repositories and durable application workflows.
- [ ] Explicit Redis outage/fallback behavior tests for application data paths.
- [ ] Full Phase 2 security review and protocol integration.

## Exact Resume Instructions

Continue on `phase-2/platform-foundation`. Do not restart the verified platform/database/cache work.

**Next implementation slice: generated Protobuf code and authenticated gRPC enrollment.**

1. Add a pinned Protocol Buffer toolchain and reproducible code-generation commands.
2. Generate Go message and gRPC service code from `proto/monitor/v1/agent.proto`.
3. Implement the server-side gRPC service with TLS requirements and strict message-size/deadline limits.
4. Implement enrollment-token issuance, secure random generation, hash-only persistence, expiry, single-use consumption, and revocation.
5. Bind enrolled agent identity to its server record and persist agent capabilities/heartbeat state.
6. Add integration tests for valid, expired, invalid, consumed, and revoked enrollment tokens.
7. Add tests for invalid agent identity/session, malformed messages, reconnect, and heartbeat state transitions.
8. Update README, roadmap, feature acceptance matrix, and this state file as each capability is verified.
9. Commit the coherent gRPC/enrollment slice using a Conventional Commit.

Do not start host metric collection before enrollment and authenticated agent communication are working and tested.

## Repository Rules

- English only in source code, comments, documentation, UI strings, logs, tests, and commit messages.
- Build the monitoring product itself; do not embed Grafana, Prometheus, Zabbix, Netdata, or another ready-made monitoring platform.
- Go, Next.js/TypeScript, PostgreSQL, Redis, Docker Compose, gRPC, Protocol Buffers, and WebSocket are allowed technologies.
- The host monitoring agent will run as a native Go service; Docker is for control-plane deployment and optional container discovery.
- Comments explain intent, invariants, security boundaries, and non-obvious logic; do not comment every trivial line mechanically.
- Use descriptive Conventional Commits.
- Update README whenever a feature is verified and at each phase completion.
- Never mark a phase complete based on a partial implementation or a green build alone.

## Session History

### Phase 2 — Initial Foundation and Database/Cache Integration

Implementation commits:

- `2b88d17fe66715d766d093eddc6348e339552392` — Initial Go API, Next.js shell, Compose stack, schema draft, and protocol contract.
- `53c22126ce739a52fd2b86094510f26e6d51006a` — PostgreSQL/Redis clients, migration runner, and dependency-aware readiness.
- `78a50d8d747f6641ab377d822974e11fb1e869aa` — Go import formatting.
- `9bd98a2fed9699847860c37e845dc49b98623895` — Container dependency resolution fix (later refined).
- `98cac4fc5616375d90501301a50ec0300736263b` — More actionable Go formatting diagnostics.
- `a82910354695f3787f01f46e2c2f318cf3d82453` — PostgreSQL config formatting.
- `ab806951ed0e1cb2534d2b3a5ce1808aa3ab18ad` — Resolve Go dependencies after source copy in Docker build.

Verification: GitHub Actions run [#45023682627](https://github.com/WorkRCS/monitor_me/actions/runs/45023682627) passed all three jobs: backend, frontend, and full Compose integration. It verified clean migration execution, PostgreSQL schema presence, Redis connectivity, and API dependency readiness.

Phase 2 remains In Progress because authentication, generated Protobuf/gRPC serving, and secure agent enrollment are not implemented yet.
