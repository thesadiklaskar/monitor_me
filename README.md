# Monitor Me

**Monitor Me is a custom infrastructure monitoring platform** designed to make server observability easier to deploy, easier to understand, and safer to operate across infrastructure environments.

The product is being built around its own native host agent, telemetry ingestion pipeline, metric model, project attribution, alert engine, and operator dashboard. It is intended for infrastructure engineers, platform teams, and organizations that need clear visibility into servers, workloads, and applications.

> **Current status:** Phase 1 is complete. Phase 2 — Platform Foundation and Secure Connectivity is in progress. The initial foundation slice is committed and its GitHub Actions CI checks have passed: Go API health endpoints/tests, Next.js lint/typecheck/production build, Docker Compose configuration validation, and service image builds. Database/cache application clients, runtime Compose smoke tests, authentication, generated gRPC code, and agent enrollment are not complete yet.

---

## Table of Contents

- [Product Vision](#product-vision)
- [Current Project Status](#current-project-status)
- [What the Product Will Provide](#what-the-product-will-provide)
- [Technology Stack](#technology-stack)
- [Architecture at a Glance](#architecture-at-a-glance)
- [Docker and Deployment Model](#docker-and-deployment-model)
- [Telemetry and Data Reliability](#telemetry-and-data-reliability)
- [Security Principles](#security-principles)
- [Five-Phase Delivery Roadmap](#five-phase-delivery-roadmap)
- [Documentation Map](#documentation-map)
- [Engineering and Contribution Standards](#engineering-and-contribution-standards)
- [Performance and Reliability Policy](#performance-and-reliability-policy)
- [Final System Design](#final-system-design)

## Product Vision

Monitor Me aims to provide a guided, low-friction monitoring experience without requiring operators to assemble a monitoring stack manually.

The target workflow is:

1. An operator signs in and selects **Add Server**.
2. The dashboard creates a scoped, short-lived enrollment credential and an installation instruction for the target operating system.
3. The native Go agent is installed on a Bare Metal server, VPS, VM, or supported cloud host.
4. The agent securely enrolls and starts reporting supported host metrics.
5. The operator sees the server's overall health and resource usage.
6. The operator can switch between **All Projects** and individual project views, where workload attribution is supported.
7. The operator can investigate historical data, receive alerts, and perform explicitly authorized actions.

The long-term objective is an enterprise-oriented product with a straightforward user experience and a measured path from a small deployment to large monitored fleets.

## Current Project Status

| Area | Status | Details |
| --- | --- | --- |
| Product requirements | **Completed** | Functional requirements, MVP scope, and acceptance criteria documented |
| Metric definitions | **Completed** | Names, units, calculations, timestamps, and platform limitations documented |
| High-level architecture | **Completed** | Component boundaries, data flow, storage responsibilities, and security boundaries documented |
| Feature acceptance matrix | **Completed** | Planned features mapped to implementation tests and CI evidence |
| Backend implementation | **Phase 2 verified** | Go API, owner bootstrap, bearer sessions, organization-scoped inventory, role checks, and TLS gRPC enrollment/heartbeat pass CI |
| Frontend implementation | **Foundation verified** | Next.js shell passes ESLint, TypeScript, and production build; product authentication and monitoring screens remain later-phase work |
| Local platform stack | **Runtime verified** | PostgreSQL, Redis, migration runner, API, and web services start through Docker Compose |
| Database and cache integration | **Runtime verified** | PostgreSQL remains durable; Redis dependency outage and recovery behavior are tested |
| Agent protocol runtime | **Phase 2 verified** | Generated Protobuf contract, TLS gRPC, single-use enrollment, heartbeat, reconnect, and revocation pass integration tests |
| Native host agent | **Planned** | Workspace boundary is defined; executable, collectors, and persistent spool begin in Phase 3 |
| Monitoring dashboard | Not started | Planned for Phase 4 |
| Production release | Not started | Planned for Phase 5 |

**Status vocabulary:** Completed means implemented and verified for the documented environment. Specified means requirements are documented but implementation is not complete. Planned means work has not started. A design document or mock screen does not count as a working feature.

## What the Product Will Provide

The following are target capabilities. They are not represented as implemented until their acceptance tests pass.

### Server Enrollment and Host Monitoring

- Guided **Add Server** and one-command installation flow.
- Secure, expiring, revocable agent enrollment.
- Overall server view for CPU, per-core CPU, memory, swap, disk, IOPS, and network.
- Agent health, server uptime, and last-seen status.
- Native monitoring agent for supported Linux, Windows, and macOS versions.

### Project and Workload Monitoring

- Header-level **All Projects / Project Selector**.
- Dashboard updates without a full page reload when the project changes.
- Project-specific CPU, memory, disk, network, process, and supported application metrics.
- Docker container and Systemd service discovery where supported and authorized.
- Linux cgroups-based resource attribution, with explicit labels for inferred or shared resources.

### Process and Application Visibility

- Process inventory and per-process CPU/RSS information.
- Listening ports and connection metadata.
- Application request latency and HTTP status/error rates through supported application instrumentation, reverse-proxy integrations, or tracing.
- Clear distinction between socket visibility and actual HTTP request telemetry.

### Dashboards, Alerts, and Operations

- Custom live and historical charts.
- Configurable metric sampling and time ranges.
- Threshold and condition-based alerts.
- Alert history, recovery events, and notifications.
- Allowlisted, audited remote restart/stop actions for supported targets.
- Historical retention and metric-appropriate downsampling.

## Technology Stack

| Layer | Technology | Responsibility |
| --- | --- | --- |
| Frontend | Next.js, TypeScript | Operator interface and dashboard |
| UI styling | Tailwind CSS | Consistent and responsive UI |
| Backend | Go (Golang) | APIs, enrollment, ingestion, processing, alerts, and action orchestration |
| Host agent | Native Go binary | OS-level metrics and workload collection |
| Agent transport | gRPC, Protocol Buffers | Authenticated, versioned binary communication |
| Live browser updates | WebSocket | Authorized live metric and event streaming |
| Primary database | PostgreSQL | Durable application state and historical telemetry |
| Live cache | Redis | Ephemeral cache and short-lived coordination |
| Local orchestration | Docker Compose | Repeatable local control-plane environment |
| Visualization | Custom Canvas/WebGL where justified | Efficient live and historical metric rendering |
| System design | Excalidraw | Final as-built architecture diagrams |

The monitoring product itself is custom-built. The project will not embed or depend on ready-made monitoring platforms such as Grafana, Prometheus, Zabbix, or Netdata. General-purpose development and infrastructure technologies listed above are allowed.

## Local Development

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for prerequisites and current limitations.

For the initial local control-plane environment:

```sh
cp .env.example .env
# Replace the local PostgreSQL and Redis passwords in .env.
docker compose up --build -d
docker compose ps
```

The API exposes `/health/live` and `/health/ready`. Liveness reports process health independently; readiness checks PostgreSQL and Redis and returns HTTP 503 when either dependency is unavailable. Docker Compose runs a one-shot migration service before the API starts. The frontend is a foundation preview and does not display live monitoring data.

## Architecture at a Glance

```text
 Monitored Hosts
 Bare Metal / VPS / VM / Cloud / Local Machines
 Linux / Windows / macOS
                 |
                 v
       Native Go Monitoring Agent
       - OS-specific collectors
       - Process and workload discovery
       - Bounded queue and persistent spool
       - Agent health and capabilities
                 |
                 | Authenticated gRPC + Protocol Buffers
                 v
          Custom Go Backend
       - Authentication and enrollment
       - Telemetry validation and ingestion
       - Project and workload attribution
       - Historical query and alert processing
       - Authorized remote-action orchestration
             /             \
            v               v
      PostgreSQL           Redis
      Durable data         Live cache and
      and history          ephemeral state
            \               /
             v             v
          Authorized WebSocket Updates
                 |
                 v
         Next.js Dashboard
       - Overall server view
       - All Projects / project view
       - Live and historical charts
       - Alerts, events, and operations
```

The Phase 2 control plane currently implements owner bootstrap, bearer authentication, organization-scoped server inventory, enrollment-token lifecycle, and TLS-protected gRPC enrollment/heartbeat. Telemetry ingestion, collectors, WebSocket fan-out, dashboards, and operational workflows remain in their later phases. The diagram shows the intended end-state data flow, not a claim that every component is already implemented.

## Docker and Deployment Model

Docker is part of the project toolchain and has two distinct roles:

1. **Control-plane development and deployment:** Docker Compose will provide a repeatable local environment for the Go backend, Next.js frontend, PostgreSQL, and Redis. A containerized single-node deployment may also be supported.
2. **Monitored workload discovery:** Where explicitly enabled and authorized, the native host agent may collect Docker container metadata and use supported cgroup information for workload attribution.

The monitoring agent is intended to run as a **native Go service/binary on the monitored host**. Docker is not required on every monitored machine. A Bare Metal server or VPS can be monitored without running the control plane in Docker.

## Telemetry and Data Reliability

The system separates live delivery from durable historical storage:

- **PostgreSQL** is the durable store for application state and historical telemetry.
- **Redis** is used for defined ephemeral workloads such as live cache and short-lived coordination. It is not the sole source of truth.
- **Agent-side persistent spool/WAL** retains unsent telemetry across temporary network interruptions and agent restarts, within configured storage limits.
- **Bounded queues, backpressure, retry limits, idempotency, and graceful shutdown** are required across the telemetry pipeline.
- **Downsampling and retention** must preserve documented resolution and must not silently replace raw data before the configured raw-retention period expires.

The initial collection target is configurable in the 100ms–500ms range where host overhead and collector capabilities allow it. Sampling interval, ingestion latency, processing latency, and dashboard freshness are separate measurements. A 100ms collection interval does not imply 1ms end-to-end latency.

## Security Principles

- TLS-protected and authenticated agent communication.
- Short-lived, scoped, revocable enrollment credentials.
- Server-side authorization for APIs and live subscriptions.
- Organization, server, and project isolation.
- Least-privilege host collection and Docker access.
- Explicit allowlists and audit records for remote actions.
- No unrestricted remote shell feature.
- Signed and verified installer artifacts before execution.
- Secrets are never committed to the repository or written to logs.

## Five-Phase Delivery Roadmap

| Phase | Name | Status | Main outcome |
| --- | --- | --- | --- |
| 1 | Product Specification and Technical Blueprint | **Completed** | Requirements, metric catalog, acceptance matrix, and architecture baseline |
| 2 | Platform Foundation and Secure Connectivity | **Completed** | Go/Next.js foundation, PostgreSQL/Redis, migrations, owner authentication, organization authorization, TLS gRPC enrollment, heartbeat, reconnect, and revocation pass CI |
| 3 | Monitoring Agent and Telemetry Data Engine | Not started | Native Linux agent, core host metrics, durable spool, ingestion, historical storage, live distribution, and workload attribution |
| 4 | Dashboard and Operational Features | Not started | Server/project dashboards, custom charts, application visibility, alerts, notifications, one-click installation workflow, and secure remote actions |
| 5 | Cross-Platform Production and Scale | Not started | Windows/macOS agents, signed lifecycle tooling, security hardening, benchmarks, production operations, and final as-built design |

A phase is marked Completed only after implementation, acceptance tests, relevant security/failure checks, documentation, and Git commit requirements pass.

## Documentation Map

| Document | Purpose |
| --- | --- |
| [Product Requirements](docs/PRODUCT_REQUIREMENTS.md) | Product vision, MVP scope, functional requirements, and non-functional requirements |
| [Metric Catalog](docs/METRIC_CATALOG.md) | Metric names, units, sources, calculations, and platform limitations |
| [Feature Acceptance Matrix](docs/FEATURE_ACCEPTANCE_MATRIX.md) | Feature-by-feature verification criteria and evidence status |
| [Architecture Overview](docs/ARCHITECTURE_OVERVIEW.md) | Components, data flow, storage, Docker boundaries, and security boundaries |
| [Zero-to-Production Roadmap](docs/ROADMAP.md) | Five delivery phases and completion gates |
| [Project State](docs/PROJECT_STATE.md) | Exact current phase, completed work, commit references, and next action |
| [Repository Instructions](AGENTS.md) | Continuation, code quality, testing, security, and commit rules |

## Engineering and Contribution Standards

- All source code, comments, documentation, UI strings, logs, tests, and commit messages must be written in English.
- Comments explain intent, invariants, security boundaries, and non-obvious logic. Comments are not added mechanically to every trivial line.
- Every feature must have relevant unit/integration tests and documented acceptance evidence.
- Use descriptive Conventional Commits, such as `feat(agent): collect host memory metrics`.
- Keep the default branch buildable; complete one small, reviewable unit at a time.
- Update the feature acceptance matrix when implementation status changes.
- Update this README when a feature is verified and whenever a phase is completed. The README must always distinguish implemented capabilities from planned capabilities.
- Update `docs/PROJECT_STATE.md` after every work session so development can resume from one exact next action.
- Never mark an item Completed based solely on a stub, mock, or design document.

## Performance and Reliability Policy

Monitor Me targets low-latency telemetry, responsive visualization, bounded resource use, and predictable recovery. No software is assumed to be perfectly crash-free or loss-free. Reliability and scale claims must be supported by reproducible tests and benchmarks that record the environment, build, workload, duration, and results.

## Final System Design

After the implementation is complete, the final Excalidraw system design will be created from the actual source code and deployment configuration. It will cover:

- Components and service boundaries.
- Agent enrollment and communication.
- Telemetry and event data flows.
- Database schema and retention paths.
- Project/workload attribution.
- Authentication, authorization, and trust boundaries.
- Failure handling, recovery, and deployment topology.
- Measured scaling and performance characteristics.

The final diagram must describe the system that was actually built, not an aspirational architecture.

---

**Current resume point:** Phase 2 is complete on main; Phase 3 — Monitoring Agent and Telemetry Data Engine is next. The complete Phase 2 implementation passes [GitHub Actions run #45030320358](https://github.com/WorkRCS/monitor_me/actions/runs/45030320358) with all three CI jobs successful. Read [docs/PROJECT_STATE.md](docs/PROJECT_STATE.md) for the exact next action.
