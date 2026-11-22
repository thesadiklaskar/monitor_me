# Monitor Me

**Monitor Me is a custom infrastructure monitoring platform** designed to make server observability easier to deploy, easier to understand, and safer to operate across infrastructure environments.

The product is being built around its own native host agent, telemetry ingestion pipeline, metric model, project attribution, alert engine, and operator dashboard. It is intended for infrastructure engineers, platform teams, and organizations that need clear visibility into servers, workloads, and applications.

> **Project setup:** **Completed** — repository structure, local development stack, database migrations, and the Phase 1–3 CI baseline are established and verified. **Product status:** Phases 1, 2 and 3 are complete. The final Phase 1–3 audit on `main` passed all four CI jobs in [GitHub Actions run #45043161542](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542). The verified Linux-first implementation includes native host collection, a durable bounded agent WAL, TLS gRPC delivery, idempotent PostgreSQL history, bounded historical queries, retention, organization-scoped WebSocket delivery, and explicit cgroup/Systemd/container workload attribution. Phase 4 — Dashboard and Operational Features has not started; the frontend remains a foundation shell. Phase 5 production work, including firewall setup, is still planned. The setup baseline is complete; the operator-facing MVP is not yet released.

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
- [Production Firewall Requirements](#production-firewall-requirements)
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
| Project setup baseline | **Completed** | Repository layout, documented toolchain, Docker Compose local stack, database migrations, and the four-job Phase 1–3 CI baseline are established and verified |
| Product requirements | **Completed** | Functional requirements, MVP scope, and acceptance criteria documented |
| Metric definitions | **Completed** | Names, units, calculations, timestamps, and platform limitations documented |
| High-level architecture | **Completed** | Component boundaries, data flow, storage responsibilities, and security boundaries documented |
| Feature acceptance matrix | **Completed** | Planned features mapped to implementation tests and CI evidence |
| Backend implementation | **Phases 2–3 verified** | Authentication, organization-scoped APIs, TLS gRPC enrollment, durable telemetry ingestion, history, retention, and live transport pass CI |
| Frontend implementation | **Foundation verified** | Next.js shell passes ESLint, TypeScript, and production build; product authentication and monitoring screens remain later-phase work |
| Local platform stack | **Runtime verified** | PostgreSQL, Redis, migration runner, API, and web services start through Docker Compose |
| Database and cache integration | **Runtime verified** | PostgreSQL remains durable; Redis and PostgreSQL dependency outage/readiness recovery are tested |
| Agent protocol and ingestion | **Phase 3 verified** | TLS enrollment, persisted session, heartbeat, reconnect/backoff, streaming batches, post-commit ACK, and idempotent PostgreSQL persistence pass end-to-end CI |
| Native Linux host agent | **Phase 3 verified** | Native Go agent, Linux procfs metrics, TCP inventory, cgroup workload selectors, private session state, and crash-recoverable durable spool pass race-enabled CI tests |
| Telemetry history and live transport | **Phase 3 verified** | Bounded PostgreSQL range query, 30-day raw retention, 90-day idempotency retention, one-use session-bound WebSocket tickets, cross-tenant isolation, and bounded subscriber queues pass CI |
| Project/workload attribution | **Phase 3 verified (Linux mapping path)** | cgroup/Systemd/container selectors, exact server/project mappings, and explicit host-total/unclassified/shared attribution pass database integration tests; full Docker Engine discovery remains later work |
| Monitoring dashboard | Not started | Planned for Phase 4 |
| Production firewall setup | Planned | Phase 5 will add role-aware Linux host firewall profiles for the control plane and monitored agents |
| Production release | Not started | Planned for Phase 5 |

**Status vocabulary:** Completed means implemented and verified for the documented environment. Specified means requirements are documented but implementation is not complete. Planned means work has not started. A design document or mock screen does not count as a working feature.

## Verified Phase 3 Implementation

The current Linux-first implementation provides a real telemetry path rather than a collector mock:

- **Native Linux agent:** aggregate and per-core CPU, memory available/used/cache/swap, disk byte and operation rates, network byte/packet/error rates, host/agent uptime, process count, top-128 readable process RSS, TCP state counts, and listening-port metadata.
- **Correct rate semantics:** counter deltas use actual monotonic elapsed time. Initial samples have no fabricated rate; resets and invalid intervals produce missing values rather than negative rates.
- **Workload metadata:** process RSS samples include cgroup path and workload selector. Systemd units and container cgroup IDs are identified when present. Unmapped and shared resource labels remain explicit.
- **Agent durability:** append-only JSONL local WAL, fsync per batch, 0600 file/0700 directory, configurable disk quota, restart recovery for a truncated trailing record, and atomic compaction after accepted acknowledgements.
- **Secure delivery:** TLS gRPC enrollment with a short-lived one-use server token; session secret is persisted locally with private permissions. The agent reports heartbeats, retries with bounded backoff, and removes a revoked local session when re-enrollment credentials are available.
- **Durable ingestion:** the backend validates authenticated batches, limits message/sample/label sizes, stores batch hashes and telemetry samples in PostgreSQL in one transaction, and sends ACK only after commit. Same-payload retries are idempotent; changed content under the same batch ID is rejected.
- **Historical query:** authenticated server-scoped range queries aggregate with PostgreSQL `date_bin`. The query enforces a 100ms minimum step, a 30-day raw range, a 5,000-point maximum and rejects a bucket grid that would be silently truncated.
- **Live delivery:** post-commit latest-value events publish to organization-specific Redis Pub/Sub channels. Each event contains at most 128 metric points. Browsers exchange their bearer session for a 30-second, single-use ticket; only the ticket hash is stored. Tickets are invalidated when the parent user session is revoked. WebSocket output queues are bounded to 32 events; slow clients are disconnected and should reconnect and fetch durable history to catch up. Cross-organization delivery is covered by integration tests.
- **Retention:** default raw samples are kept for 30 days; batch idempotency records for 90 days; expired live tickets are removed in bounded cleanup batches. Redis is notification/cache infrastructure, never the durable telemetry store.

The Phase 3 implementation does **not** yet claim Windows/macOS collection, per-process CPU, eBPF, complete Docker Engine metadata, HTTP request latency/error instrumentation, persistent materialized rollups, the remaining AgentHealth samples (agent CPU, collector errors, spool size, pending queue depth, and last successful send), or a dashboard UI. The dashboard and user workflows are Phase 4; cross-platform collectors, installers, and production scale are Phase 5. Historical queries aggregate with PostgreSQL date_bin; persistent rollup tables are deferred until scale measurements justify them. A monitored Linux host does not need Docker installed.

## Phase 3 CI Benchmark Snapshot

The verified [Phase 3 CI run #45039191041](https://github.com/WorkRCS/monitor_me/actions/runs/45039191041) executed the following benchmarks on a GitHub-hosted Ubuntu runner:

| Benchmark | Result | Workload |
| --- | ---: | --- |
| Linux proc collector | 5.33 ms/op; 1.03 MB/op; 5,163 allocs/op | Read host counters, processes, and workload metadata |
| Counter-rate calculation | 3.68 ns/op; 0 allocs/op | One counter delta and monotonic elapsed-time calculation |
| Agent spool append | 256 μs/op; 952 B/op; 12 allocs/op | Append and fsync a one-sample batch to the local WAL |
| Live event preparation | 128 μs/op; 165.7 KB/op; 897 allocs/op | Collapse and encode a representative 96-sample event using an in-memory publisher |
| TLS gRPC to PostgreSQL ingestion | 4.40 ms per 64-sample batch; 14,533 samples/sec | Authenticated TLS gRPC stream, PostgreSQL transaction, durable ACK, and Redis publication |

These figures describe one hosted CI runner and this benchmark workload only. They are not production capacity guarantees or evidence of million-host scale. Hardware, filesystem durability, database placement, network latency, metric cardinality, and write concurrency materially affect results. Phase 5 will add staged multi-agent capacity testing.

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

The API exposes `/health/live` and `/health/ready`. Liveness reports process health independently; readiness checks PostgreSQL and Redis and returns HTTP 503 when either dependency is unavailable. Docker Compose runs a one-shot migration service before the API starts. The frontend remains a foundation preview and does not yet render the telemetry stream.

Build the native agent and its tests:

```sh
cd agent
go mod tidy
go test -race ./...
go vet ./...
go build -trimpath ./cmd/monitor-agent
```

To run a Linux agent against a local control plane, create a server through the authenticated API, copy its one-time enrollment token, then start the agent with the local development CA:

```sh
MONITOR_GRPC_ADDRESS=localhost:8443 \
MONITOR_TLS_CA_FILE=../.data/certs/server.crt \
MONITOR_ENROLLMENT_TOKEN="$ENROLLMENT_TOKEN" \
MONITOR_SPOOL_DIR=./.monitor-me/spool \
MONITOR_AGENT_STATE_FILE=./.monitor-me/agent-state.json \
./monitor-agent
```

The enrollment token is single-use and should not be committed to shell history or source control. The agent stores the returned session in a private local state file; after the initial enrollment, a restart can reuse that session without the enrollment token while it remains valid.

Authenticated history endpoint:

```text
GET /v1/servers/{serverID}/metrics?metric=host.cpu.utilization&from=2026-11-22T10:00:00Z&to=2026-11-22T10:05:00Z&step=1s&limit=300
```

Browser live connections first call `POST /v1/live/ticket` with the bearer session, then connect to `GET /v1/live/ws?ticket={single-use-ticket}`. The ticket is valid for 30 seconds and cannot be reused.

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

The control plane implements owner bootstrap, bearer authentication, organization-scoped server inventory, enrollment-token lifecycle, and TLS-protected gRPC enrollment/heartbeat. Phase 3 now provides a native Linux collector, durable spool, authenticated idempotent PostgreSQL ingestion, range queries, retention, and authorized WebSocket delivery. Dashboards and operator workflows remain in Phase 4. The diagram shows the intended end-state data flow, not a claim that every component is already implemented.

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
- Production deployment uses a least-privilege host firewall: expose only intended HTTPS/gRPC entry points, restrict SSH to administrators, and keep database/cache ports private.

## Production Firewall Requirements

The production firewall policy is defined for two roles: a control-plane VPS/VM and agent-only monitored Linux hosts. Agent-only hosts require no inbound Monitor Me ports; they send outbound TLS gRPC to the control plane. The control plane normally exposes TCP 443 and TCP 8443, optional TCP 80 for ACME HTTP-01, and SSH only from a trusted administrator CIDR. PostgreSQL 5432, Redis 6379, API 8080, and web 3000 must remain private. See [Production Firewall Requirements](docs/PRODUCTION_FIREWALL.md) for UFW/firewalld examples, Docker firewall caveats, and local/LAN deployment rules. Firewall automation is a Phase 5 deliverable, not a currently implemented installer feature.

## Five-Phase Delivery Roadmap

| Phase | Name | Status | Main outcome |
| --- | --- | --- | --- |
| 1 | Product Specification and Technical Blueprint | **Completed** | Requirements, metric catalog, acceptance matrix, and architecture baseline |
| 2 | Platform Foundation and Secure Connectivity | **Completed** | Go/Next.js foundation, PostgreSQL/Redis, migrations, owner authentication, organization authorization, TLS gRPC enrollment, heartbeat, reconnect, and revocation pass CI |
| 3 | Monitoring Agent and Telemetry Data Engine | **Completed** | Linux host collection, durable spool, TLS gRPC ingestion, PostgreSQL history/retention, authorized live delivery, workload attribution, and CI benchmarks all verified |
| 4 | Dashboard and Operational Features | Not started | Server/project dashboards, custom charts, application visibility, alerts, notifications, one-click installation workflow, and secure remote actions |
| 5 | Cross-Platform Production and Scale | Not started | Windows/macOS agents, signed lifecycle tooling, role-aware Linux firewall setup, security hardening, benchmarks, production operations, and final as-built design |

A phase is marked Completed only after implementation, acceptance tests, relevant security/failure checks, documentation, and Git commit requirements pass.

## MVP Readiness

Phases 1–3 are complete and their phase gates pass CI. This does **not** mean the first usable product MVP is released. The authenticated login UI, server inventory UI, Add Server installation/token wizard, overall server dashboard, and browser-facing live metric views are still Phase 4 work. The API, Linux agent, durable ingestion, history, and live transport are verified foundations for those workflows.

## Documentation Map

| Document | Purpose |
| --- | --- |
| [Product Requirements](docs/PRODUCT_REQUIREMENTS.md) | Product vision, MVP scope, functional requirements, and non-functional requirements |
| [Metric Catalog](docs/METRIC_CATALOG.md) | Metric names, units, sources, calculations, and platform limitations |
| [Feature Acceptance Matrix](docs/FEATURE_ACCEPTANCE_MATRIX.md) | Feature-by-feature verification criteria and evidence status |
| [Architecture Overview](docs/ARCHITECTURE_OVERVIEW.md) | Components, data flow, storage, Docker boundaries, and security boundaries |
| [Production Firewall Requirements](docs/PRODUCTION_FIREWALL.md) | Least-privilege firewall profiles for the control plane, agent hosts, Docker, and local/LAN deployments |
| [Phase 4 Execution Plan](docs/PHASE_4_EXECUTION_PLAN.md) | Dashboard implementation order, verified API baseline, acceptance gates, and agreed release/test sequence |
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

**Current resume point:** Phases 1–3 are complete on `main`; Phase 4 and Phase 5 have not started. The final audit passed all four CI jobs in [GitHub Actions run #45043161542](https://github.com/WorkRCS/monitor_me/actions/runs/45043161542). No Phase 4 or Phase 5 implementation was started during this audit. Read [docs/PROJECT_STATE.md](docs/PROJECT_STATE.md) for the exact next action.
