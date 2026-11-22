# Project State

This file is the authoritative resume point for Monitor Me. Read it before making changes and update it after every work session.

## Current Snapshot

| Field | Value |
| --- | --- |
| Project | Monitor Me |
| Repository | `WorkRCS/monitor_me` |
| Default branch | `main` |
| Repository visibility | Private |
| Completed phase | Phase 1 — Product Specification and Technical Blueprint |
| Current phase | Phase 2 — Platform Foundation and Secure Connectivity |
| Current phase status | Not Started |
| Implementation status | Product specification is complete; application implementation has not started |
| Last completed phase deliverable commit | `526d0fec87491c6f87d493f12d8121f208761899` |
| Next action | Design the repository layout and Phase 2 architecture decisions, then initialize the Go backend, Next.js frontend, PostgreSQL, Redis, and Docker Compose baseline in small buildable commits. |

## Completed Phases

### Phase 1 — Product Specification and Technical Blueprint

**Status: Completed**

Deliverables created:

- [x] `docs/PRODUCT_REQUIREMENTS.md` — vision, users, MVP boundary, functional and non-functional requirements.
- [x] `docs/METRIC_CATALOG.md` — metric names, units, collection semantics, calculations, and platform constraints.
- [x] `docs/FEATURE_ACCEPTANCE_MATRIX.md` — feature-by-feature verification requirements and five-phase completion checks.
- [x] `docs/ARCHITECTURE_OVERVIEW.md` — component responsibilities, data flow, Docker boundaries, storage, and security.
- [x] Updated `docs/ROADMAP.md` — five major phases with scope and completion gates.
- [x] Updated `README.md` — professional project overview, current feature status, architecture, technology, Docker model, roadmap, and document navigation.
- [x] Updated `AGENTS.md` — README maintenance is mandatory whenever a feature or phase is verified.
- [x] Defined the exact next action for Phase 2.

### Phase 1 Commit References

| Deliverable | Commit |
| --- | --- |
| Product requirements | `5c4bf9b3684107bfb3884ebf26181013ed42ea4b` |
| Metric catalog | `4f982c881823b9cdea6250432f96cdad407af94f` |
| Feature acceptance matrix | `5bd649196345de671d58fa3d01a3187e6f689608` |
| Architecture overview | `6d4719e81ab032c9f35f9631275212f78aceef7d` |
| README update | `df5612381f74aa49367cf2ed625deb9225ff2879` |
| Five-phase roadmap | `11a5f2112b8c2ce1381a9ae8f2f7ed77fc66a839` |
| README maintenance rules | `526d0fec87491c6f87d493f12d8121f208761899` |

**Verification status:** All Phase 1 specification documents were created successfully. Repository file reads and content checks are performed after the documentation updates. Phase 1 completion is documentation/requirements completion only; no backend, frontend, agent, or monitoring feature is implemented yet.

## Current Phase

### Phase 2 — Platform Foundation and Secure Connectivity

**Status: Not Started**

Planned work:

1. Define repository layout for Go backend, Go agent, and Next.js frontend.
2. Add formatting, linting, tests, static checks, and CI.
3. Initialize Go backend configuration, structured logging, health/readiness, and graceful shutdown.
4. Initialize Next.js with TypeScript and Tailwind CSS.
5. Add Docker Compose for frontend, backend, PostgreSQL, and Redis.
6. Add PostgreSQL migrations and connection management.
7. Add Redis for explicitly ephemeral live cache/coordination.
8. Define versioned Protocol Buffers and gRPC services.
9. Implement authentication and authorization foundations.
10. Implement secure agent enrollment, identity, heartbeat, capabilities, reconnect, and revocation.

## Exact Resume Instructions

Start Phase 2 with repository structure and architecture decisions. Work in small, independently buildable slices.

The first implementation slice should:

1. Define the root directory layout and service boundaries.
2. Add pinned toolchain/runtime versions and minimal build/test commands.
3. Create the smallest Go backend service with configuration, structured logging, health/readiness endpoints, and graceful shutdown.
4. Add a minimal Next.js application shell.
5. Add Docker Compose for local development with PostgreSQL and Redis health checks.
6. Add a concise developer setup guide and tests for the initial service health behavior.
7. Update the README immediately when any part becomes verified, and update the phase status only when its complete gate passes.

Do not start the full monitoring collectors before the backend, data, protocol, and enrollment foundations are in place.

## Repository Rules

- English only in source code, comments, documentation, user-facing strings, logs, tests, and commits.
- Use Go, Next.js/TypeScript, PostgreSQL, Redis, gRPC, Protocol Buffers, WebSocket, and Docker Compose according to documented responsibilities.
- Build the monitoring product itself; do not embed Grafana, Prometheus, Zabbix, Netdata, or another ready-made monitoring platform.
- Use native Go agents on monitored hosts; Docker is for the control-plane environment and optional container discovery, not a universal host-agent requirement.
- Add meaningful comments for intent, invariants, security rules, and non-obvious logic; do not comment trivial lines mechanically.
- Use Conventional Commits for every reviewable unit.
- Update the feature acceptance matrix and README when a feature becomes verified.
- Keep README, roadmap, and project state synchronized.
- Never claim crash-free, lag-free, perfectly accurate, or million-host operation without reproducible evidence.

## Session History

### Phase 1 Closeout

- Completed the product specification and technical blueprint.
- Consolidated the delivery plan into five major phases as requested.
- Established the README as the continuously maintained product guide.
- Set Phase 2 as the exact next phase.
